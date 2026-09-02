package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/domain"
	"github.com/d2cTool/goprofile/internal/middleware"
	"github.com/d2cTool/goprofile/internal/services"
)

type stubRepo struct {
	avatar *domain.Avatar
}

func (s *stubRepo) CreateWithOutbox(_ context.Context, a *domain.Avatar, _ domain.OutboxEvent) (int64, error) {
	cp := *a
	s.avatar = &cp
	return 1, nil
}
func (s *stubRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Avatar, error) {
	if s.avatar == nil || s.avatar.ID != id {
		return nil, domain.ErrAvatarNotFound
	}
	cp := *s.avatar
	return &cp, nil
}
func (s *stubRepo) GetLatestByUserID(_ context.Context, userID string) (*domain.Avatar, error) {
	if s.avatar == nil || s.avatar.UserID != userID {
		return nil, domain.ErrAvatarNotFound
	}
	cp := *s.avatar
	return &cp, nil
}
func (s *stubRepo) ListByUserID(_ context.Context, userID string) ([]domain.Avatar, error) {
	if s.avatar == nil || s.avatar.UserID != userID {
		return []domain.Avatar{}, nil
	}
	return []domain.Avatar{*s.avatar}, nil
}
func (s *stubRepo) SoftDeleteOwnedWithOutbox(_ context.Context, id uuid.UUID, userID string, _ domain.OutboxEvent) (*domain.Avatar, int64, error) {
	if s.avatar == nil || s.avatar.ID != id {
		return nil, 0, domain.ErrAvatarNotFound
	}
	if s.avatar.UserID != userID {
		return nil, 0, domain.ErrForbidden
	}
	cp := *s.avatar
	s.avatar = nil
	return &cp, 1, nil
}
func (s *stubRepo) MarkProcessing(context.Context, uuid.UUID) (bool, error) { return true, nil }
func (s *stubRepo) CompleteProcessing(context.Context, uuid.UUID, map[string]string, int, int) error {
	return nil
}
func (s *stubRepo) FailProcessing(context.Context, uuid.UUID) error { return nil }
func (s *stubRepo) ListUnpublished(context.Context, int) ([]domain.OutboxEvent, error) {
	return nil, nil
}
func (s *stubRepo) MarkOutboxPublished(context.Context, int64) error { return nil }
func (s *stubRepo) ListStuckUploads(context.Context, int) ([]domain.Avatar, error) {
	return nil, nil
}

type stubObj struct{ store map[string][]byte }

func (s *stubObj) Upload(_ context.Context, key, _ string, data []byte) error {
	if s.store == nil {
		s.store = map[string][]byte{}
	}
	s.store[key] = append([]byte(nil), data...)
	return nil
}
func (s *stubObj) Download(_ context.Context, key string) ([]byte, string, error) {
	data, ok := s.store[key]
	if !ok {
		return nil, "", domain.ErrAvatarNotFound
	}
	return data, "image/png", nil
}
func (s *stubObj) Delete(context.Context, []string) error { return nil }

type stubPub struct{}

func (stubPub) PublishUpload(context.Context, domain.AvatarUploadEvent) error { return nil }
func (stubPub) PublishDelete(context.Context, domain.AvatarDeleteEvent) error { return nil }

func pngFile(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.RGBA{G: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newTestHandler() (*AvatarHandler, *stubRepo) {
	repo := &stubRepo{}
	svc := services.NewAvatarService(repo, &stubObj{store: map[string][]byte{}}, stubPub{}, "http://localhost:8080", domain.MaxFileSize)
	return NewAvatarHandler(svc, domain.MaxFileSize), repo
}

func TestUploadAndMetadataAndDelete(t *testing.T) {
	h, _ := newTestHandler()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", "ava.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(pngFile(t)); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/avatars", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = requestWithUser(req, "alice")
	rec := httptest.NewRecorder()
	h.Upload(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status %d body %s", rec.Code, rec.Body.String())
	}
	var created uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", created.ID)
	metaReq := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/"+created.ID+"/metadata", nil)
	metaReq = metaReq.WithContext(context.WithValue(metaReq.Context(), chi.RouteCtxKey, rctx))
	metaRec := httptest.NewRecorder()
	h.Metadata(metaRec, metaReq)
	if metaRec.Code != http.StatusOK {
		t.Fatalf("metadata %d %s", metaRec.Code, metaRec.Body.String())
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/avatars/"+created.ID, nil)
	delReq = requestWithUser(delReq, "alice")
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), chi.RouteCtxKey, rctx))
	delRec := httptest.NewRecorder()
	h.DeleteByID(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", delRec.Code, delRec.Body.String())
	}
}

func TestUploadRejectsMissingFile(t *testing.T) {
	h, _ := newTestHandler()
	req := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/v1/avatars", bytes.NewBufferString("")), "alice")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	rec := httptest.NewRecorder()
	h.Upload(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestGetUnknownAvatar(t *testing.T) {
	h, _ := newTestHandler()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", uuid.NewString())
	req := httptest.NewRequest(http.MethodGet, "/x", nil).WithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.GetByID(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWriteErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{domain.ErrAvatarNotFound, http.StatusNotFound},
		{domain.ErrForbidden, http.StatusForbidden},
		{domain.ErrFileTooLarge, http.StatusRequestEntityTooLarge},
		{domain.ErrInvalidFileFormat, http.StatusBadRequest},
		{domain.ErrImageTooLarge, http.StatusBadRequest},
		{io.EOF, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		writeError(rec, tc.err)
		if rec.Code != tc.code {
			t.Fatalf("%v: %d", tc.err, rec.Code)
		}
	}
}

func requestWithUser(r *http.Request, userID string) *http.Request {
	r.Header.Set(middleware.HeaderUserID, userID)
	return r.WithContext(middleware.WithUserID(r.Context(), userID))
}
