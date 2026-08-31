package handlers

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/domain"
	"github.com/d2cTool/goprofile/internal/middleware"
	"github.com/d2cTool/goprofile/internal/services"
)

type AvatarHandler struct {
	svc      *services.AvatarService
	maxBytes int64
}

func NewAvatarHandler(svc *services.AvatarService, maxBytes int64) *AvatarHandler {
	if maxBytes <= 0 {
		maxBytes = domain.MaxFileSize
	}
	return &AvatarHandler{svc: svc, maxBytes: maxBytes}
}

func (h *AvatarHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFrom(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+512*1024)
	if err := r.ParseMultipartForm(h.maxBytes); err != nil {
		if isTooLarge(err) {
			writeError(w, domain.ErrFileTooLarge)
			return
		}
		writeError(w, domain.ErrFileRequired)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, domain.ErrFileRequired)
		return
	}
	defer file.Close()

	limited := io.LimitReader(file, h.maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		writeError(w, err)
		return
	}
	if int64(len(data)) > h.maxBytes {
		writeError(w, domain.ErrFileTooLarge)
		return
	}

	avatar, err := h.svc.Upload(r.Context(), userID, header.Filename, data)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, uploadResponse{
		ID:        avatar.ID.String(),
		UserID:    avatar.UserID,
		URL:       h.svc.PublicURL(avatar.ID, domain.SizeOriginal),
		Status:    "processing",
		CreatedAt: avatar.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

func (h *AvatarHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseAvatarID(chi.URLParam(r, "avatar_id"))
	if err != nil {
		writeError(w, domain.ErrAvatarNotFound)
		return
	}
	avatar, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeImage(w, r, avatar)
}

func (h *AvatarHandler) GetByUser(w http.ResponseWriter, r *http.Request) {
	avatar, err := h.svc.GetLatest(r.Context(), chi.URLParam(r, "user_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeImage(w, r, avatar)
}

func (h *AvatarHandler) Metadata(w http.ResponseWriter, r *http.Request) {
	id, err := parseAvatarID(chi.URLParam(r, "avatar_id"))
	if err != nil {
		writeError(w, domain.ErrAvatarNotFound)
		return
	}
	avatar, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toMetadata(avatar, func(size string) string {
		return h.svc.PublicURL(avatar.ID, size)
	}))
}

func (h *AvatarHandler) ListByUser(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context(), chi.URLParam(r, "user_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]metadataResponse, 0, len(items))
	for i := range items {
		a := items[i]
		out = append(out, toMetadata(&a, func(size string) string {
			return h.svc.PublicURL(a.ID, size)
		}))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *AvatarHandler) DeleteByID(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFrom(r.Context())
	id, err := parseAvatarID(chi.URLParam(r, "avatar_id"))
	if err != nil {
		writeError(w, domain.ErrAvatarNotFound)
		return
	}
	if err := h.svc.Delete(r.Context(), id, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AvatarHandler) DeleteByUser(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFrom(r.Context())
	target := chi.URLParam(r, "user_id")
	if userID != target {
		writeError(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteLatest(r.Context(), userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AvatarHandler) writeImage(w http.ResponseWriter, r *http.Request, avatar *domain.Avatar) {
	payload, err := h.svc.FetchImage(r.Context(), avatar, r.URL.Query().Get("size"), r.URL.Query().Get("format"))
	if err != nil {
		writeError(w, err)
		return
	}
	if match := r.Header.Get("If-None-Match"); match != "" && match == payload.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", payload.ContentType)
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Header().Set("ETag", payload.ETag)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload.Data)
}

func parseAvatarID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, domain.ErrAvatarNotFound
	}
	return id, nil
}

func isTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
