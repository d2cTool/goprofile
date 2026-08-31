package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	UploadStatusUploading = "uploading"
	UploadStatusStored    = "stored"
	UploadStatusFailed    = "failed"

	ProcessingPending    = "pending"
	ProcessingProcessing = "processing"
	ProcessingCompleted  = "completed"
	ProcessingFailed     = "failed"

	SizeOriginal = "original"
	Size100      = "100x100"
	Size300      = "300x300"

	MaxFileSize = 10 << 20
)

var AllowedSizes = map[string]struct{}{
	SizeOriginal: {},
	Size100:      {},
	Size300:      {},
}

var AllowedFormats = map[string]string{
	"jpeg": "image/jpeg",
	"jpg":  "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
}

type Avatar struct {
	ID               uuid.UUID
	UserID           string
	FileName         string
	MimeType         string
	SizeBytes        int64
	Width            int
	Height           int
	S3Key            string
	ThumbnailS3Keys  map[string]string
	UploadStatus     string
	ProcessingStatus string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

func (a Avatar) ThumbnailKey(size string) (string, bool) {
	if a.ThumbnailS3Keys == nil {
		return "", false
	}
	key, ok := a.ThumbnailS3Keys[size]
	return key, ok && key != ""
}

func (a Avatar) ThumbnailKeysJSON() []byte {
	if a.ThumbnailS3Keys == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(a.ThumbnailS3Keys)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func ParseThumbnailKeys(raw []byte) map[string]string {
	if len(raw) == 0 {
		return map[string]string{}
	}
	out := map[string]string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]string{}
	}
	return out
}

func OriginalObjectKey(userID, avatarID, ext string) string {
	return "originals/" + userID + "/" + avatarID + ext
}

func ThumbnailObjectKey(avatarID, size, ext string) string {
	return "thumbnails/" + avatarID + "/" + size + ext
}
