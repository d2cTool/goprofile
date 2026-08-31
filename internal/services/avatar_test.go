package services

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/domain"
)

type memRepo struct {
	mu     sync.Mutex
	items  map[uuid.UUID]*domain.Avatar
	events map[string]struct{}
}

func newMemRepo() *memRepo {
	return &memRepo{items: map[uuid.UUID]*domain.Avatar{}, events: map[string]struct{}{}}
}

func (m *memRepo) Create(_ context.Context, a *domain.Avatar) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *a
	m.items[a.ID] = &cp
	return nil
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

func (m *memRepo) SoftDeleteOwned(_ context.Context, id uuid.UUID, userID string) (*domain.Avatar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.items[id]
	if !ok || a.DeletedAt != nil {
		return nil, domain.ErrAvatarNotFound
	}
	if a.UserID != userID {
		return nil, domain.ErrForbidden
	}
	now := a.UpdatedAt
	a.DeletedAt = &now
	cp := *a
	return &cp, nil
}

func (m *memRepo) SoftDeleteLatestOwned(ctx context.Context, userID string) (*domain.Avatar, error) {
	a, err := m.GetLatestByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return m.SoftDeleteOwned(ctx, a.ID, userID)
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

func (m *memRepo) ClaimEvent(_ context.Context, eventID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.events[eventID]; ok {
		return false, nil
	}
	m.events[eventID] = struct{}{}
	return true, nil
}

func (m *memRepo) ReleaseEvent(_ context.Context, eventID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.events, eventID)
	return nil
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

func (m *memPub) PublishProcess(context.Context, domain.AvatarProcessEvent) error { return nil }

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
