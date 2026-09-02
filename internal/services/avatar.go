package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/domain"
	"github.com/d2cTool/goprofile/internal/imageutil"
)

type AvatarService struct {
	repo      AvatarStore
	objects   ObjectStore
	publisher EventPublisher
	publicURL string
	maxBytes  int64
}

func NewAvatarService(repo AvatarStore, objects ObjectStore, publisher EventPublisher, publicURL string, maxBytes int64) *AvatarService {
	if maxBytes <= 0 {
		maxBytes = domain.MaxFileSize
	}
	return &AvatarService{
		repo:      repo,
		objects:   objects,
		publisher: publisher,
		publicURL: strings.TrimRight(publicURL, "/"),
		maxBytes:  maxBytes,
	}
}

func (s *AvatarService) Upload(ctx context.Context, userID, fileName string, data []byte) (*domain.Avatar, error) {
	if err := domain.ValidateUserID(userID); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, domain.ErrFileRequired
	}
	if int64(len(data)) > s.maxBytes {
		return nil, domain.ErrFileTooLarge
	}

	info, err := imageutil.Inspect(data)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	id := uuid.New()
	key := domain.OriginalObjectKey(userID, id.String(), info.Ext)
	avatar := &domain.Avatar{
		ID:               id,
		UserID:           userID,
		FileName:         sanitizeFileName(fileName, info.Ext),
		MimeType:         info.MIME,
		SizeBytes:        int64(len(data)),
		Width:            info.Width,
		Height:           info.Height,
		S3Key:            key,
		ThumbnailS3Keys:  map[string]string{},
		UploadStatus:     domain.UploadStatusStored,
		ProcessingStatus: domain.ProcessingPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.objects.Upload(ctx, key, info.MIME, data); err != nil {
		return nil, err
	}

	event := domain.AvatarUploadEvent{
		EventID:  domain.UploadEventID(id.String()),
		AvatarID: id.String(),
		UserID:   userID,
		S3Key:    key,
	}
	ev, err := newOutboxEvent(domain.OutboxUpload, event.EventID, event)
	if err != nil {
		_ = s.objects.Delete(ctx, []string{key})
		return nil, err
	}
	if _, err := s.repo.CreateWithOutbox(ctx, avatar, ev); err != nil {
		_ = s.objects.Delete(ctx, []string{key})
		return nil, err
	}
	return avatar, nil
}

func (s *AvatarService) Get(ctx context.Context, id uuid.UUID) (*domain.Avatar, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AvatarService) GetLatest(ctx context.Context, userID string) (*domain.Avatar, error) {
	if err := domain.ValidateUserID(userID); err != nil {
		return nil, err
	}
	return s.repo.GetLatestByUserID(ctx, userID)
}

func (s *AvatarService) List(ctx context.Context, userID string) ([]domain.Avatar, error) {
	if err := domain.ValidateUserID(userID); err != nil {
		return nil, err
	}
	return s.repo.ListByUserID(ctx, userID)
}

func (s *AvatarService) Delete(ctx context.Context, id uuid.UUID, userID string) error {
	if err := domain.ValidateUserID(userID); err != nil {
		return err
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return domain.ErrForbidden
	}
	ev, err := deleteOutboxEvent(existing)
	if err != nil {
		return err
	}
	if _, _, err := s.repo.SoftDeleteOwnedWithOutbox(ctx, id, userID, ev); err != nil {
		return err
	}
	return nil
}

func (s *AvatarService) DeleteLatest(ctx context.Context, userID string) error {
	if err := domain.ValidateUserID(userID); err != nil {
		return err
	}
	current, err := s.repo.GetLatestByUserID(ctx, userID)
	if err != nil {
		return err
	}
	return s.Delete(ctx, current.ID, userID)
}

func deleteOutboxEvent(a *domain.Avatar) (domain.OutboxEvent, error) {
	keys := []string{a.S3Key}
	for _, k := range a.ThumbnailS3Keys {
		if k != "" {
			keys = append(keys, k)
		}
	}
	event := domain.AvatarDeleteEvent{
		EventID:  domain.DeleteEventID(a.ID.String()),
		AvatarID: a.ID.String(),
		S3Keys:   keys,
	}
	return newOutboxEvent(domain.OutboxDelete, event.EventID, event)
}

func newOutboxEvent(kind, eventID string, payload any) (domain.OutboxEvent, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	return domain.OutboxEvent{EventID: eventID, Kind: kind, Payload: body}, nil
}

func (s *AvatarService) publishKind(ctx context.Context, kind string, body []byte) error {
	switch kind {
	case domain.OutboxUpload:
		var event domain.AvatarUploadEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		return s.publisher.PublishUpload(ctx, event)
	case domain.OutboxDelete:
		var event domain.AvatarDeleteEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		return s.publisher.PublishDelete(ctx, event)
	default:
		return fmt.Errorf("unknown outbox kind %s", kind)
	}
}

func (s *AvatarService) FlushOutbox(ctx context.Context) error {
	items, err := s.repo.ListUnpublished(ctx, 50)
	if err != nil {
		return err
	}
	var first error
	for _, item := range items {
		if err := publishWithRetry(ctx, 3, func() error {
			return s.publishKind(ctx, item.Kind, item.Payload)
		}); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		if err := s.repo.MarkOutboxPublished(ctx, item.ID); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *AvatarService) RecoverStuck(ctx context.Context) error {
	items, err := s.repo.ListStuckUploads(ctx, 20)
	if err != nil {
		return err
	}
	var first error
	for _, a := range items {
		if err := s.ProcessUpload(ctx, domain.AvatarUploadEvent{
			EventID:  domain.UploadEventID(a.ID.String()),
			AvatarID: a.ID.String(),
			UserID:   a.UserID,
			S3Key:    a.S3Key,
		}); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func publishWithRetry(ctx context.Context, attempts int, fn func() error) error {
	backoff := 100 * time.Millisecond
	var last error
	for i := 0; i < attempts; i++ {
		last = fn()
		if last == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	return last
}

type ImagePayload struct {
	Data        []byte
	ContentType string
	ETag        string
}

func (s *AvatarService) FetchImage(ctx context.Context, avatar *domain.Avatar, size, format string) (*ImagePayload, error) {
	size, err := domain.NormalizeSize(size)
	if err != nil {
		return nil, err
	}
	format, err = domain.NormalizeFormat(format)
	if err != nil {
		return nil, err
	}

	key := avatar.S3Key
	if size != domain.SizeOriginal {
		if thumb, ok := avatar.ThumbnailKey(size); ok {
			key = thumb
		}
	}

	data, contentType, err := s.objects.Download(ctx, key)
	if err != nil {
		return nil, err
	}

	current := imageutil.FormatFromMIME(contentType)
	if current == "" {
		current = imageutil.FormatFromMIME(avatar.MimeType)
	}
	if format != "" && format != current {
		converted, mime, convErr := imageutil.Convert(data, format)
		if convErr != nil {
			return nil, convErr
		}
		data = converted
		contentType = mime
	} else if contentType == "" {
		contentType = avatar.MimeType
	}

	sum := sha256.Sum256(append([]byte(avatar.ID.String()+"|"+size+"|"+format+"|"), data...))
	return &ImagePayload{
		Data:        data,
		ContentType: contentType,
		ETag:        `"` + hex.EncodeToString(sum[:8]) + `"`,
	}, nil
}

func (s *AvatarService) PublicURL(id uuid.UUID, size string) string {
	u := s.publicURL + "/api/v1/avatars/" + id.String()
	if size != "" && size != domain.SizeOriginal {
		u += "?size=" + size
	}
	return u
}

func (s *AvatarService) ProcessUpload(ctx context.Context, event domain.AvatarUploadEvent) error {
	id, err := uuid.Parse(event.AvatarID)
	if err != nil {
		return domain.Permanent(fmt.Errorf("invalid avatar_id: %w", err))
	}

	avatar, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, domain.ErrAvatarNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if avatar.ProcessingStatus == domain.ProcessingCompleted {
		return nil
	}
	if event.S3Key == "" {
		event.S3Key = avatar.S3Key
	}

	ok, err := s.repo.MarkProcessing(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	data, _, err := s.objects.Download(ctx, event.S3Key)
	if err != nil {
		_ = s.repo.FailProcessing(ctx, id)
		return err
	}

	info, err := imageutil.Inspect(data)
	if err != nil {
		_ = s.repo.FailProcessing(ctx, id)
		return nil
	}

	thumbs := map[string]string{}
	for _, spec := range []struct {
		size string
		w, h int
	}{
		{domain.Size100, 100, 100},
		{domain.Size300, 300, 300},
	} {
		body, mime, err := imageutil.MakeThumbnail(data, spec.w, spec.h, imageutil.FormatFromMIME(info.MIME))
		if err != nil {
			_ = s.repo.FailProcessing(ctx, id)
			return err
		}
		key := domain.ThumbnailObjectKey(event.AvatarID, spec.size, imageutil.ExtForMIME(mime))
		if err := s.objects.Upload(ctx, key, mime, body); err != nil {
			_ = s.repo.FailProcessing(ctx, id)
			return err
		}
		thumbs[spec.size] = key
	}

	return s.repo.CompleteProcessing(ctx, id, thumbs, info.Width, info.Height)
}

func (s *AvatarService) ProcessDelete(ctx context.Context, event domain.AvatarDeleteEvent) error {
	return s.objects.Delete(ctx, event.S3Keys)
}

func sanitizeFileName(name, ext string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\\", "_")
	if name == "" || name == "." || name == ".." {
		return "avatar" + ext
	}
	if len(name) > 255 {
		name = name[:255]
	}
	return name
}
