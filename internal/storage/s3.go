package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel/attribute"

	"github.com/d2cTool/goprofile/internal/config"
	"github.com/d2cTool/goprofile/internal/observability"
)

type S3 struct {
	client *minio.Client
	bucket string
}

func NewS3(cfg config.Config) (*S3, error) {
	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
		Region: cfg.S3Region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return &S3{client: client, bucket: cfg.S3Bucket}, nil
}

func (s *S3) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("bucket exists: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("make bucket: %w", err)
	}
	return nil
}

func (s *S3) Ping(ctx context.Context) error {
	_, err := s.client.BucketExists(ctx, s.bucket)
	return err
}

func (s *S3) Upload(ctx context.Context, key, contentType string, data []byte) error {
	ctx, span := observability.Start(ctx, "s3.put",
		attribute.String("s3.bucket", s.bucket),
		attribute.String("s3.key", key),
		attribute.Int("s3.size", len(data)),
	)
	defer span.End()
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		observability.RecordError(span, err)
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

func (s *S3) Download(ctx context.Context, key string) ([]byte, string, error) {
	ctx, span := observability.Start(ctx, "s3.get",
		attribute.String("s3.bucket", s.bucket),
		attribute.String("s3.key", key),
	)
	defer span.End()
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		observability.RecordError(span, err)
		return nil, "", fmt.Errorf("get object %s: %w", key, err)
	}
	defer func() { _ = obj.Close() }()

	info, err := obj.Stat()
	if err != nil {
		observability.RecordError(span, err)
		return nil, "", fmt.Errorf("stat object %s: %w", key, err)
	}
	data, err := io.ReadAll(obj)
	if err != nil {
		observability.RecordError(span, err)
		return nil, "", fmt.Errorf("read object %s: %w", key, err)
	}
	span.SetAttributes(attribute.Int("s3.size", len(data)))
	return data, info.ContentType, nil
}

func (s *S3) Delete(ctx context.Context, keys []string) error {
	ctx, span := observability.Start(ctx, "s3.delete",
		attribute.String("s3.bucket", s.bucket),
		attribute.Int("s3.keys", len(keys)),
	)
	defer span.End()
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
		if err != nil {
			observability.RecordError(span, err)
			return fmt.Errorf("remove object %s: %w", key, err)
		}
	}
	return nil
}
