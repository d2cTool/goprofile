package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/domain"
	"github.com/d2cTool/goprofile/internal/services"
)

func TestGetImageAndDeleteLatest(t *testing.T) {
	repo := &stubRepo{}
	obj := &stubObj{store: map[string][]byte{}}
	svc := services.NewAvatarService(repo, obj, stubPub{}, "http://localhost:8080", domain.MaxFileSize)
	h := NewAvatarHandler(svc, domain.MaxFileSize)

	id := uuid.New()
	key := "originals/erin/" + id.String() + ".png"
	data := pngFile(t)
	repo.avatar = &domain.Avatar{
		ID:       id,
		UserID:   "erin",
		FileName: "a.png",
		MimeType: "image/png",
		S3Key:    key,
	}
	_ = obj.Upload(context.Background(), key, "image/png", data)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("avatar_id", id.String())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/"+id.String()+"?size=original&format=jpeg", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.GetByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get image %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("content-type %s", rec.Header().Get("Content-Type"))
	}

	userCtx := chi.NewRouteContext()
	userCtx.URLParams.Add("user_id", "erin")
	getUser := httptest.NewRequest(http.MethodGet, "/api/v1/users/erin/avatar", nil)
	getUser = getUser.WithContext(context.WithValue(getUser.Context(), chi.RouteCtxKey, userCtx))
	getRec := httptest.NewRecorder()
	h.GetByUser(getRec, getUser)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get by user %d %s", getRec.Code, getRec.Body.String())
	}

	delReq := requestWithUser(httptest.NewRequest(http.MethodDelete, "/api/v1/users/erin/avatar", nil), "erin")
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), chi.RouteCtxKey, userCtx))
	delRec := httptest.NewRecorder()
	h.DeleteByUser(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete latest %d %s", delRec.Code, delRec.Body.String())
	}
}
