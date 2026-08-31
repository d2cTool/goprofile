package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/d2cTool/goprofile/internal/domain"
)

type AvatarRepository struct {
	pool *pgxpool.Pool
}

func NewAvatarRepository(pool *pgxpool.Pool) *AvatarRepository {
	return &AvatarRepository{pool: pool}
}

func (r *AvatarRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *AvatarRepository) Create(ctx context.Context, a *domain.Avatar) error {
	const q = `
		INSERT INTO avatars (
			id, user_id, file_name, mime_type, size_bytes, width, height,
			s3_key, thumbnail_s3_keys, upload_status, processing_status,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13
		)`
	_, err := r.pool.Exec(ctx, q,
		a.ID, a.UserID, a.FileName, a.MimeType, a.SizeBytes, a.Width, a.Height,
		a.S3Key, a.ThumbnailKeysJSON(), a.UploadStatus, a.ProcessingStatus,
		a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert avatar: %w", err)
	}
	return nil
}

func (r *AvatarRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Avatar, error) {
	const q = `
		SELECT id, user_id, file_name, mime_type, size_bytes, width, height,
		       s3_key, COALESCE(thumbnail_s3_keys, '{}'::jsonb), upload_status, processing_status,
		       created_at, updated_at, deleted_at
		FROM avatars
		WHERE id = $1 AND deleted_at IS NULL`
	return scanAvatar(r.pool.QueryRow(ctx, q, id))
}

func (r *AvatarRepository) GetLatestByUserID(ctx context.Context, userID string) (*domain.Avatar, error) {
	const q = `
		SELECT id, user_id, file_name, mime_type, size_bytes, width, height,
		       s3_key, COALESCE(thumbnail_s3_keys, '{}'::jsonb), upload_status, processing_status,
		       created_at, updated_at, deleted_at
		FROM avatars
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1`
	return scanAvatar(r.pool.QueryRow(ctx, q, userID))
}

func (r *AvatarRepository) ListByUserID(ctx context.Context, userID string) ([]domain.Avatar, error) {
	const q = `
		SELECT id, user_id, file_name, mime_type, size_bytes, width, height,
		       s3_key, COALESCE(thumbnail_s3_keys, '{}'::jsonb), upload_status, processing_status,
		       created_at, updated_at, deleted_at
		FROM avatars
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list avatars: %w", err)
	}
	defer rows.Close()

	var out []domain.Avatar
	for rows.Next() {
		a, err := scanAvatar(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.Avatar{}
	}
	return out, nil
}

func (r *AvatarRepository) SoftDeleteOwned(ctx context.Context, id uuid.UUID, userID string) (*domain.Avatar, error) {
	const q = `
		UPDATE avatars
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		RETURNING id, user_id, file_name, mime_type, size_bytes, width, height,
		          s3_key, COALESCE(thumbnail_s3_keys, '{}'::jsonb), upload_status, processing_status,
		          created_at, updated_at, deleted_at`
	return scanAvatar(r.pool.QueryRow(ctx, q, id, userID))
}

func (r *AvatarRepository) SoftDeleteLatestOwned(ctx context.Context, userID string) (*domain.Avatar, error) {
	const q = `
		UPDATE avatars
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = (
			SELECT id FROM avatars
			WHERE user_id = $1 AND deleted_at IS NULL
			ORDER BY created_at DESC
			LIMIT 1
		)
		RETURNING id, user_id, file_name, mime_type, size_bytes, width, height,
		          s3_key, COALESCE(thumbnail_s3_keys, '{}'::jsonb), upload_status, processing_status,
		          created_at, updated_at, deleted_at`
	return scanAvatar(r.pool.QueryRow(ctx, q, userID))
}

func (r *AvatarRepository) MarkProcessing(ctx context.Context, id uuid.UUID) (bool, error) {
	const q = `
		UPDATE avatars
		SET processing_status = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL AND processing_status IN ($3, $4)
		RETURNING id`
	var got uuid.UUID
	err := r.pool.QueryRow(ctx, q, id, domain.ProcessingProcessing, domain.ProcessingPending, domain.ProcessingFailed).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mark processing: %w", err)
	}
	return true, nil
}

func (r *AvatarRepository) CompleteProcessing(ctx context.Context, id uuid.UUID, thumbs map[string]string, width, height int) error {
	if thumbs == nil {
		thumbs = map[string]string{}
	}
	raw, err := jsonBytes(thumbs)
	if err != nil {
		return err
	}
	const q = `
		UPDATE avatars
		SET thumbnail_s3_keys = $2,
		    width = CASE WHEN $3 > 0 THEN $3 ELSE width END,
		    height = CASE WHEN $4 > 0 THEN $4 ELSE height END,
		    processing_status = $5,
		    upload_status = $6,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id, raw, width, height, domain.ProcessingCompleted, domain.UploadStatusStored)
	if err != nil {
		return fmt.Errorf("complete processing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAvatarNotFound
	}
	return nil
}

func (r *AvatarRepository) FailProcessing(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE avatars
		SET processing_status = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id, domain.ProcessingFailed)
	if err != nil {
		return fmt.Errorf("fail processing: %w", err)
	}
	return nil
}

func (r *AvatarRepository) ClaimEvent(ctx context.Context, eventID string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO processed_events (event_id) VALUES ($1)
		ON CONFLICT (event_id) DO NOTHING`, eventID)
	if err != nil {
		return false, fmt.Errorf("claim event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *AvatarRepository) ReleaseEvent(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM processed_events WHERE event_id = $1`, eventID)
	return err
}

func jsonBytes(v map[string]string) ([]byte, error) {
	a := domain.Avatar{ThumbnailS3Keys: v}
	return a.ThumbnailKeysJSON(), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAvatar(row rowScanner) (*domain.Avatar, error) {
	var (
		a   domain.Avatar
		raw []byte
		del *time.Time
	)
	err := row.Scan(
		&a.ID, &a.UserID, &a.FileName, &a.MimeType, &a.SizeBytes, &a.Width, &a.Height,
		&a.S3Key, &raw, &a.UploadStatus, &a.ProcessingStatus,
		&a.CreatedAt, &a.UpdatedAt, &del,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAvatarNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan avatar: %w", err)
	}
	a.ThumbnailS3Keys = domain.ParseThumbnailKeys(raw)
	a.DeletedAt = del
	return &a, nil
}
