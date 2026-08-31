package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/domain"
)

type AvatarStore interface {
	Create(ctx context.Context, a *domain.Avatar) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Avatar, error)
	GetLatestByUserID(ctx context.Context, userID string) (*domain.Avatar, error)
	ListByUserID(ctx context.Context, userID string) ([]domain.Avatar, error)
	SoftDeleteOwned(ctx context.Context, id uuid.UUID, userID string) (*domain.Avatar, error)
	SoftDeleteLatestOwned(ctx context.Context, userID string) (*domain.Avatar, error)
	MarkProcessing(ctx context.Context, id uuid.UUID) (bool, error)
	CompleteProcessing(ctx context.Context, id uuid.UUID, thumbs map[string]string, width, height int) error
	FailProcessing(ctx context.Context, id uuid.UUID) error
	EnqueueOutbox(ctx context.Context, ev domain.OutboxEvent) (int64, error)
	ListUnpublished(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkOutboxPublished(ctx context.Context, id int64) error
	ListStuckUploads(ctx context.Context, limit int) ([]domain.Avatar, error)
}

type ObjectStore interface {
	Upload(ctx context.Context, key, contentType string, data []byte) error
	Download(ctx context.Context, key string) ([]byte, string, error)
	Delete(ctx context.Context, keys []string) error
}

type EventPublisher interface {
	PublishUpload(ctx context.Context, event domain.AvatarUploadEvent) error
	PublishDelete(ctx context.Context, event domain.AvatarDeleteEvent) error
}
