package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	if err := s.repo.Create(ctx, avatar); err != nil {
		return nil, err
	}

	event := domain.AvatarUploadEvent{
		EventID:  domain.UploadEventID(id.String()),
		AvatarID: id.String(),
		UserID:   userID,
		S3Key:    key,
	}
	if err := s.publisher.PublishUpload(ctx, event); err != nil {
		return nil, fmt.Errorf("publish upload event: %w", err)
	}
	_ = s.publisher.PublishProcess(ctx, domain.AvatarProcessEvent{
		EventID:  domain.ProcessEventID(id.String()),
		AvatarID: id.String(),
		Operations: []domain.ProcessingOp{
			{Name: "thumbnail", Width: 100, Height: 100},
			{Name: "thumbnail", Width: 300, Height: 300},
		},
	})
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
	deleted, err := s.repo.SoftDeleteOwned(ctx, id, userID)
	if err != nil {
		return err
	}
	return s.publishDelete(ctx, deleted)
}

func (s *AvatarService) DeleteLatest(ctx context.Context, userID string) error {
	if err := domain.ValidateUserID(userID); err != nil {
		return err
	}
	deleted, err := s.repo.SoftDeleteLatestOwned(ctx, userID)
	if err != nil {
		return err
	}
	return s.publishDelete(ctx, deleted)
}

func (s *AvatarService) publishDelete(ctx context.Context, a *domain.Avatar) error {
	keys := []string{a.S3Key}
	for _, k := range a.ThumbnailS3Keys {
		if k != "" {
			keys = append(keys, k)
		}
	}
	return s.publisher.PublishDelete(ctx, domain.AvatarDeleteEvent{
		EventID:  domain.DeleteEventID(a.ID.String()),
		AvatarID: a.ID.String(),
		S3Keys:   keys,
	})
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

	if format != "" && format != imageutil.FormatFromMIME(contentType) && format != imageutil.FormatFromMIME(avatar.MimeType) {
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
	if event.EventID == "" {
		event.EventID = domain.UploadEventID(event.AvatarID)
	}
	claimed, err := s.repo.ClaimEvent(ctx, event.EventID)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}

	id, err := uuid.Parse(event.AvatarID)
	if err != nil {
		_ = s.repo.ReleaseEvent(ctx, event.EventID)
		return fmt.Errorf("invalid avatar_id: %w", err)
	}

	avatar, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, domain.ErrAvatarNotFound) {
		return nil
	}
	if err != nil {
		_ = s.repo.ReleaseEvent(ctx, event.EventID)
		return err
	}
	if avatar.ProcessingStatus == domain.ProcessingCompleted {
		return nil
	}

	ok, err := s.repo.MarkProcessing(ctx, id)
	if err != nil {
		_ = s.repo.ReleaseEvent(ctx, event.EventID)
		return err
	}
	if !ok && avatar.ProcessingStatus == domain.ProcessingCompleted {
		return nil
	}

	data, _, err := s.objects.Download(ctx, event.S3Key)
	if err != nil {
		_ = s.repo.FailProcessing(ctx, id)
		_ = s.repo.ReleaseEvent(ctx, event.EventID)
		return err
	}

	info, err := imageutil.Inspect(data)
	if err != nil {
		_ = s.repo.FailProcessing(ctx, id)
		_ = s.repo.ReleaseEvent(ctx, event.EventID)
		return err
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
			_ = s.repo.ReleaseEvent(ctx, event.EventID)
			return err
		}
		key := domain.ThumbnailObjectKey(event.AvatarID, spec.size, imageutil.ExtForMIME(mime))
		if err := s.objects.Upload(ctx, key, mime, body); err != nil {
			_ = s.repo.FailProcessing(ctx, id)
			_ = s.repo.ReleaseEvent(ctx, event.EventID)
			return err
		}
		thumbs[spec.size] = key
	}

	if err := s.repo.CompleteProcessing(ctx, id, thumbs, info.Width, info.Height); err != nil {
		_ = s.repo.ReleaseEvent(ctx, event.EventID)
		return err
	}
	return nil
}

func (s *AvatarService) ProcessDelete(ctx context.Context, event domain.AvatarDeleteEvent) error {
	if event.EventID == "" {
		event.EventID = domain.DeleteEventID(event.AvatarID)
	}
	claimed, err := s.repo.ClaimEvent(ctx, event.EventID)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if err := s.objects.Delete(ctx, event.S3Keys); err != nil {
		_ = s.repo.ReleaseEvent(ctx, event.EventID)
		return err
	}
	return nil
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
