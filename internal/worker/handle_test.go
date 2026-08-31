package worker

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"github.com/d2cTool/goprofile/internal/domain"
	"github.com/d2cTool/goprofile/internal/services"
)

type claimRepo struct {
	events map[string]struct{}
}

func (c *claimRepo) Create(context.Context, *domain.Avatar) error { return nil }
func (c *claimRepo) GetByID(context.Context, uuid.UUID) (*domain.Avatar, error) {
	return nil, domain.ErrAvatarNotFound
}
func (c *claimRepo) GetLatestByUserID(context.Context, string) (*domain.Avatar, error) {
	return nil, domain.ErrAvatarNotFound
}
func (c *claimRepo) ListByUserID(context.Context, string) ([]domain.Avatar, error) {
	return nil, nil
}
func (c *claimRepo) SoftDeleteOwned(context.Context, uuid.UUID, string) (*domain.Avatar, error) {
	return nil, domain.ErrAvatarNotFound
}
func (c *claimRepo) SoftDeleteLatestOwned(context.Context, string) (*domain.Avatar, error) {
	return nil, domain.ErrAvatarNotFound
}
func (c *claimRepo) MarkProcessing(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (c *claimRepo) CompleteProcessing(context.Context, uuid.UUID, map[string]string, int, int) error {
	return nil
}
func (c *claimRepo) FailProcessing(context.Context, uuid.UUID) error { return nil }
func (c *claimRepo) ClaimEvent(_ context.Context, eventID string) (bool, error) {
	if _, ok := c.events[eventID]; ok {
		return false, nil
	}
	c.events[eventID] = struct{}{}
	return true, nil
}
func (c *claimRepo) ReleaseEvent(_ context.Context, eventID string) error {
	delete(c.events, eventID)
	return nil
}

type delObj struct{ n int }

func (d *delObj) Upload(context.Context, string, string, []byte) error { return nil }
func (d *delObj) Download(context.Context, string) ([]byte, string, error) {
	return nil, "", domain.ErrAvatarNotFound
}
func (d *delObj) Delete(_ context.Context, keys []string) error {
	d.n += len(keys)
	return nil
}

type nopPub struct{}

func (nopPub) PublishUpload(context.Context, domain.AvatarUploadEvent) error   { return nil }
func (nopPub) PublishDelete(context.Context, domain.AvatarDeleteEvent) error   { return nil }
func (nopPub) PublishProcess(context.Context, domain.AvatarProcessEvent) error { return nil }

func TestHandleBadJSON(t *testing.T) {
	w := &Worker{
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		attempts: 1,
		svc:      services.NewAvatarService(&claimRepo{events: map[string]struct{}{}}, &delObj{}, nopPub{}, "http://x", 10),
	}
	if err := w.handleUpload(context.Background(), kafka.Message{Value: []byte("{")}); err == nil {
		t.Fatal("expected decode error")
	}
	if err := w.handleDelete(context.Background(), kafka.Message{Value: []byte("{")}); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestHandleDeleteOK(t *testing.T) {
	objects := &delObj{}
	svc := services.NewAvatarService(&claimRepo{events: map[string]struct{}{}}, objects, nopPub{}, "http://x", 10)
	w := &Worker{svc: svc, log: slog.New(slog.NewTextHandler(io.Discard, nil)), attempts: 1}

	event := domain.AvatarDeleteEvent{EventID: "delete:1", AvatarID: "1", S3Keys: []string{"a"}}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.handleDelete(context.Background(), kafka.Message{Value: body}); err != nil {
		t.Fatal(err)
	}
	if objects.n != 1 {
		t.Fatalf("deleted %d", objects.n)
	}
}
