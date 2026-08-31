package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/config"
	"github.com/d2cTool/goprofile/internal/domain"
	"github.com/d2cTool/goprofile/internal/handlers"
	"github.com/d2cTool/goprofile/internal/services"
)

type healthPinger struct{}

func (healthPinger) Ping(context.Context) error { return nil }

type emptyStore struct{}

func (emptyStore) Create(context.Context, *domain.Avatar) error { return nil }
func (emptyStore) GetByID(context.Context, uuid.UUID) (*domain.Avatar, error) {
	return nil, domain.ErrAvatarNotFound
}
func (emptyStore) GetLatestByUserID(context.Context, string) (*domain.Avatar, error) {
	return nil, domain.ErrAvatarNotFound
}
func (emptyStore) ListByUserID(context.Context, string) ([]domain.Avatar, error) {
	return []domain.Avatar{}, nil
}
func (emptyStore) SoftDeleteOwned(context.Context, uuid.UUID, string) (*domain.Avatar, error) {
	return nil, domain.ErrAvatarNotFound
}
func (emptyStore) SoftDeleteLatestOwned(context.Context, string) (*domain.Avatar, error) {
	return nil, domain.ErrAvatarNotFound
}
func (emptyStore) MarkProcessing(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (emptyStore) CompleteProcessing(context.Context, uuid.UUID, map[string]string, int, int) error {
	return nil
}
func (emptyStore) FailProcessing(context.Context, uuid.UUID) error  { return nil }
func (emptyStore) ClaimEvent(context.Context, string) (bool, error) { return true, nil }
func (emptyStore) ReleaseEvent(context.Context, string) error       { return nil }

type emptyObj struct{}

func (emptyObj) Upload(context.Context, string, string, []byte) error { return nil }
func (emptyObj) Download(context.Context, string) ([]byte, string, error) {
	return nil, "", domain.ErrAvatarNotFound
}
func (emptyObj) Delete(context.Context, []string) error { return nil }

type emptyPub struct{}

func (emptyPub) PublishUpload(context.Context, domain.AvatarUploadEvent) error   { return nil }
func (emptyPub) PublishDelete(context.Context, domain.AvatarDeleteEvent) error   { return nil }
func (emptyPub) PublishProcess(context.Context, domain.AvatarProcessEvent) error { return nil }

func TestRouterHealthAndUploadPage(t *testing.T) {
	svc := services.NewAvatarService(emptyStore{}, emptyObj{}, emptyPub{}, "http://localhost", domain.MaxFileSize)
	avatars := handlers.NewAvatarHandler(svc, domain.MaxFileSize)
	webh, err := handlers.NewWebHandler(avatars, domain.MaxFileSize)
	if err != nil {
		t.Fatal(err)
	}
	h := NewRouter(
		config.Config{CORSOrigins: []string{"*"}, RateLimitRPS: 100},
		avatars,
		&handlers.HealthHandler{DB: healthPinger{}, S3: healthPinger{}, Kafka: healthPinger{}},
		webh,
	)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/web/upload", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload page %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("root %d", rec.Code)
	}
}
