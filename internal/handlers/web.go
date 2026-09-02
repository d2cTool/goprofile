package handlers

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/d2cTool/goprofile/internal/domain"
	"github.com/d2cTool/goprofile/internal/services"
	"github.com/d2cTool/goprofile/web"
)

type WebHandler struct {
	svc      *services.AvatarService
	static   http.Handler
	maxBytes int64
}

func NewWebHandler(svc *services.AvatarService, maxBytes int64) (*WebHandler, error) {
	sub, err := fs.Sub(web.FS, ".")
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		maxBytes = domain.MaxFileSize
	}
	return &WebHandler{
		svc:      svc,
		static:   http.FileServer(http.FS(sub)),
		maxBytes: maxBytes,
	}, nil
}

func (h *WebHandler) UploadPage(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, "upload.html")
}

func (h *WebHandler) GalleryPage(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, "gallery.html")
}

func (h *WebHandler) UploadForm(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+512*1024)
	if err := r.ParseMultipartForm(h.maxBytes); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		userID = strings.TrimSpace(r.Header.Get("X-User-ID"))
	}
	if err := domain.ValidateUserID(userID); err != nil {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, h.maxBytes+1))
	if err != nil {
		http.Error(w, "failed to read file", http.StatusBadRequest)
		return
	}
	if int64(len(data)) > h.maxBytes {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}

	avatar, err := h.svc.Upload(r.Context(), userID, header.Filename, data)
	if err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, r, "/web/gallery/"+avatar.UserID, http.StatusSeeOther)
}

func (h *WebHandler) Static(w http.ResponseWriter, r *http.Request) {
	h.static.ServeHTTP(w, r)
}

func (h *WebHandler) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/" + name
	h.static.ServeHTTP(w, r2)
}
