package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/domain"
)

func TestListAndGetByUser(t *testing.T) {
	h, repo := newTestHandler()
	id := uuid.New()
	repo.avatar = &domain.Avatar{
		ID:               id,
		UserID:           "erin",
		FileName:         "a.png",
		MimeType:         "image/png",
		S3Key:            "originals/erin/" + id.String() + ".png",
		ThumbnailS3Keys:  map[string]string{domain.Size100: "t"},
		ProcessingStatus: domain.ProcessingCompleted,
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("user_id", "erin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/erin/avatars", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.ListByUser(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}

	delReq := requestWithUser(httptest.NewRequest(http.MethodDelete, "/api/v1/users/erin/avatar", nil), "mallory")
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), chi.RouteCtxKey, rctx))
	delRec := httptest.NewRecorder()
	h.DeleteByUser(delRec, delReq)
	if delRec.Code != http.StatusForbidden {
		t.Fatalf("forbidden %d", delRec.Code)
	}
}
