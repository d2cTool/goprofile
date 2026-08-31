package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/d2cTool/goprofile/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrAvatarNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Avatar not found"})
	case errors.Is(err, domain.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":   "Forbidden",
			"details": "You can only delete your own avatars",
		})
	case errors.Is(err, domain.ErrInvalidUserID):
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "Invalid user id",
			"details": "X-User-ID header is required",
		})
	case errors.Is(err, domain.ErrFileRequired):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
	case errors.Is(err, domain.ErrFileTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":    "File too large",
			"max_size": domain.MaxFileSize,
		})
	case errors.Is(err, domain.ErrInvalidFileFormat), errors.Is(err, domain.ErrInvalidImage):
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "Invalid file format",
			"details": "Supported formats: jpeg, png, webp",
		})
	case errors.Is(err, domain.ErrImageTooLarge):
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "Image too large",
			"details": "Maximum resolution is 20 megapixels",
		})
	case errors.Is(err, domain.ErrInvalidSizeParam):
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "Invalid size",
			"details": "Supported values: original, 100x100, 300x300",
		})
	case errors.Is(err, domain.ErrInvalidFormatParam):
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "Invalid format",
			"details": "Supported values: jpeg, png, webp",
		})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

type uploadResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type dimensions struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type thumbnailDTO struct {
	Size string `json:"size"`
	URL  string `json:"url"`
}

type metadataResponse struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	FileName   string         `json:"file_name"`
	MimeType   string         `json:"mime_type"`
	Size       int64          `json:"size"`
	Dimensions dimensions     `json:"dimensions"`
	Thumbnails []thumbnailDTO `json:"thumbnails"`
	Status     string         `json:"status"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
}

func toMetadata(a *domain.Avatar, urlFn func(size string) string) metadataResponse {
	thumbs := make([]thumbnailDTO, 0, 2)
	for _, size := range []string{domain.Size100, domain.Size300} {
		if _, ok := a.ThumbnailKey(size); ok {
			thumbs = append(thumbs, thumbnailDTO{Size: size, URL: urlFn(size)})
		}
	}
	return metadataResponse{
		ID:       a.ID.String(),
		UserID:   a.UserID,
		FileName: a.FileName,
		MimeType: a.MimeType,
		Size:     a.SizeBytes,
		Dimensions: dimensions{
			Width:  a.Width,
			Height: a.Height,
		},
		Thumbnails: thumbs,
		Status:     a.ProcessingStatus,
		CreatedAt:  a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  a.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
