package services

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/domain"
)

type memRepo struct {
	mu         sync.Mutex
	items      map[uuid.UUID]*domain.Avatar
	outbox     []domain.OutboxEvent
	nextID     int64
	fail       error
	failOutbox error
}

func newMemRepo() *memRepo {
	return &memRepo{items: map[uuid.UUID]*domain.Avatar{}}
}

func (m *memRepo) CreateWithOutbox(_ context.Context, a *domain.Avatar, ev domain.OutboxEvent) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return 0, m.fail
	}
	if m.failOutbox != nil {
		return 0, m.failOutbox
	}
	cp := *a
	m.items[a.ID] = &cp
	return m.enqueueLocked(ev), nil
}

func (m *memRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Avatar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.items[id]
	if !ok || a.DeletedAt != nil {
		return nil, domain.ErrAvatarNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *memRepo) GetLatestByUserID(_ context.Context, userID string) (*domain.Avatar, error) {
	list, err := m.ListByUserID(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, domain.ErrAvatarNotFound
	}
	return &list[0], nil
}

func (m *memRepo) ListByUserID(_ context.Context, userID string) ([]domain.Avatar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Avatar
	for _, a := range m.items {
		if a.UserID == userID && a.DeletedAt == nil {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (m *memRepo) SoftDeleteOwnedWithOutbox(_ context.Context, id uuid.UUID, userID string, ev domain.OutboxEvent) (*domain.Avatar, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.items[id]
	if !ok || a.DeletedAt != nil {
		return nil, 0, domain.ErrAvatarNotFound
	}
	if a.UserID != userID {
		return nil, 0, domain.ErrForbidden
	}
	if m.failOutbox != nil {
		return nil, 0, m.failOutbox
	}
	now := a.UpdatedAt
	a.DeletedAt = &now
	cp := *a
	return &cp, m.enqueueLocked(ev), nil
}

func (m *memRepo) MarkProcessing(_ context.Context, id uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.items[id]
	if !ok {
		return false, nil
	}
	if a.ProcessingStatus != domain.ProcessingPending && a.ProcessingStatus != domain.ProcessingFailed {
		return false, nil
	}
	a.ProcessingStatus = domain.ProcessingProcessing
	return true, nil
}

func (m *memRepo) CompleteProcessing(_ context.Context, id uuid.UUID, thumbs map[string]string, width, height int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.items[id]
	if !ok {
		return domain.ErrAvatarNotFound
	}
	a.ThumbnailS3Keys = thumbs
	if width > 0 {
		a.Width = width
	}
	if height > 0 {
		a.Height = height
	}
	a.ProcessingStatus = domain.ProcessingCompleted
	a.UploadStatus = domain.UploadStatusStored
	return nil
}

func (m *memRepo) FailProcessing(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.items[id]; ok {
		a.ProcessingStatus = domain.ProcessingFailed
	}
	return nil
}

func (m *memRepo) enqueueLocked(ev domain.OutboxEvent) int64 {
	m.nextID++
	ev.ID = m.nextID
	m.outbox = append(m.outbox, ev)
	return ev.ID
}

func (m *memRepo) ListUnpublished(_ context.Context, _ int) ([]domain.OutboxEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.OutboxEvent(nil), m.outbox...), nil
}

func (m *memRepo) MarkOutboxPublished(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, ev := range m.outbox {
		if ev.ID == id {
			m.outbox = append(m.outbox[:i], m.outbox[i+1:]...)
			break
		}
	}
	return nil
}

func (m *memRepo) ListStuckUploads(_ context.Context, _ int) ([]domain.Avatar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Avatar
	for _, a := range m.items {
		if a.DeletedAt == nil && a.ProcessingStatus != domain.ProcessingCompleted {
			out = append(out, *a)
		}
	}
	return out, nil
}

type memObjects struct {
	mu   sync.Mutex
	data map[string][]byte
	mime map[string]string
}

func newMemObjects() *memObjects {
	return &memObjects{data: map[string][]byte{}, mime: map[string]string{}}
}

func (m *memObjects) Upload(_ context.Context, key, contentType string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.data[key] = cp
	m.mime[key] = contentType
	return nil
}

func (m *memObjects) Download(_ context.Context, key string) ([]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.data[key]
	if !ok {
		return nil, "", domain.ErrAvatarNotFound
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, m.mime[key], nil
}

func (m *memObjects) Delete(_ context.Context, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.data, k)
		delete(m.mime, k)
	}
	return nil
}

type memPub struct {
	mu      sync.Mutex
	uploads []domain.AvatarUploadEvent
	deletes []domain.AvatarDeleteEvent
}

func (m *memPub) PublishUpload(_ context.Context, event domain.AvatarUploadEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploads = append(m.uploads, event)
	return nil
}

func (m *memPub) PublishDelete(_ context.Context, event domain.AvatarDeleteEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletes = append(m.deletes, event)
	return nil
}

type failPub struct{}

func (failPub) PublishUpload(context.Context, domain.AvatarUploadEvent) error {
	return errors.New("kafka down")
}
func (failPub) PublishDelete(context.Context, domain.AvatarDeleteEvent) error {
	return errors.New("kafka down")
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUploadGetDeleteAndProcess(t *testing.T) {
	repo := newMemRepo()
	objects := newMemObjects()
	pub := &memPub{}
	svc := NewAvatarService(repo, objects, pub, "http://localhost:8080", domain.MaxFileSize)
	ctx := context.Background()

	avatar, err := svc.Upload(ctx, "alice", "face.png", pngBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	if avatar.UserID != "alice" || avatar.MimeType != "image/png" {
		t.Fatalf("%+v", avatar)
	}
	if len(pub.uploads) != 0 {
		t.Fatal("upload must not publish kafka synchronously")
	}
	if len(repo.outbox) != 1 {
		t.Fatalf("outbox %d", len(repo.outbox))
	}
	if err := svc.FlushOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if len(pub.uploads) != 1 {
		t.Fatal("upload event not published")
	}

	got, err := svc.Get(ctx, avatar.ID)
	if err != nil || got.FileName != "face.png" {
		t.Fatalf("get: %+v %v", got, err)
	}

	if err := svc.ProcessUpload(ctx, pub.uploads[0]); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessUpload(ctx, pub.uploads[0]); err != nil {
		t.Fatal(err)
	}
	processed, err := svc.Get(ctx, avatar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if processed.ProcessingStatus != domain.ProcessingCompleted {
		t.Fatalf("status %s", processed.ProcessingStatus)
	}
	if _, ok := processed.ThumbnailKey(domain.Size100); !ok {
		t.Fatal("thumb 100 missing")
	}

	img, err := svc.FetchImage(ctx, processed, "100x100", "jpeg")
	if err != nil || img.ContentType != "image/jpeg" {
		t.Fatalf("fetch %+v %v", img, err)
	}

	if err := svc.Delete(ctx, avatar.ID, "bob"); err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if err := svc.Delete(ctx, avatar.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, avatar.ID); err != domain.ErrAvatarNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	if len(pub.deletes) != 0 {
		t.Fatal("delete must not publish kafka synchronously")
	}
	if err := svc.FlushOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if len(pub.deletes) != 1 {
		t.Fatal("delete event not published")
	}
	if err := svc.ProcessDelete(ctx, pub.deletes[0]); err != nil {
		t.Fatal(err)
	}
}

func TestUploadValidation(t *testing.T) {
	svc := NewAvatarService(newMemRepo(), newMemObjects(), &memPub{}, "http://x", 16)
	if _, err := svc.Upload(context.Background(), "", "a.png", []byte("x")); err != domain.ErrInvalidUserID {
		t.Fatalf("userid: %v", err)
	}
	if _, err := svc.Upload(context.Background(), "u", "a.png", nil); err != domain.ErrFileRequired {
		t.Fatalf("empty: %v", err)
	}
	if _, err := svc.Upload(context.Background(), "u", "a.png", bytes.Repeat([]byte("a"), 20)); err != domain.ErrFileTooLarge {
		t.Fatalf("large: %v", err)
	}
	if _, err := svc.Upload(context.Background(), "u", "a.png", []byte("not-image")); err != domain.ErrInvalidFileFormat {
		t.Fatalf("format: %v", err)
	}
}

func TestDeleteRollsBackWhenOutboxFails(t *testing.T) {
	repo := newMemRepo()
	objects := newMemObjects()
	svc := NewAvatarService(repo, objects, &memPub{}, "http://x", domain.MaxFileSize)
	ctx := context.Background()

	avatar, err := svc.Upload(ctx, "alice", "a.png", pngBytes(t))
	if err != nil {
		t.Fatal(err)
	}

	repo.failOutbox = errors.New("outbox down")
	if err := svc.Delete(ctx, avatar.ID, "alice"); err == nil {
		t.Fatal("expected outbox error")
	}
	if _, err := svc.Get(ctx, avatar.ID); err != nil {
		t.Fatalf("avatar must stay visible after failed delete tx: %v", err)
	}
	if len(objects.data) == 0 {
		t.Fatal("s3 object must remain when delete is rolled back")
	}
}

func TestUploadCompensatesS3WhenCreateFails(t *testing.T) {
	repo := newMemRepo()
	repo.fail = errors.New("db down")
	objects := newMemObjects()
	svc := NewAvatarService(repo, objects, &memPub{}, "http://x", domain.MaxFileSize)
	if _, err := svc.Upload(context.Background(), "alice", "a.png", pngBytes(t)); err == nil {
		t.Fatal("expected create error")
	}
	if len(objects.data) != 0 {
		t.Fatalf("s3 orphan %#v", objects.data)
	}
}

func TestUploadSucceedsWhenPublishFails(t *testing.T) {
	repo := newMemRepo()
	svc := NewAvatarService(repo, newMemObjects(), failPub{}, "http://x", domain.MaxFileSize)
	avatar, err := svc.Upload(context.Background(), "alice", "a.png", pngBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	if avatar.ID == uuid.Nil {
		t.Fatal("avatar")
	}
	if len(repo.outbox) != 1 {
		t.Fatalf("outbox %d", len(repo.outbox))
	}
}

func TestFlushOutboxAndRecoverStuck(t *testing.T) {
	repo := newMemRepo()
	objects := newMemObjects()
	pub := &memPub{}
	svc := NewAvatarService(repo, objects, pub, "http://x", domain.MaxFileSize)
	ctx := context.Background()

	avatar, err := svc.Upload(ctx, "alice", "a.png", pngBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	repo.mu.Lock()
	_ = repo.enqueueLocked(domain.OutboxEvent{
		EventID: "upload:" + avatar.ID.String() + ":retry",
		Kind:    domain.OutboxUpload,
		Payload: []byte(`{"avatar_id":"` + avatar.ID.String() + `","user_id":"alice","s3_key":"` + avatar.S3Key + `"}`),
	})
	repo.mu.Unlock()
	if err := svc.FlushOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecoverStuck(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, avatar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProcessingStatus != domain.ProcessingCompleted {
		t.Fatalf("status %s", got.ProcessingStatus)
	}
}
