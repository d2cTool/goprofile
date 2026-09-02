package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/d2cTool/goprofile/internal/broker"
	"github.com/d2cTool/goprofile/internal/config"
	"github.com/d2cTool/goprofile/internal/migrate"
	"github.com/d2cTool/goprofile/internal/repository"
	"github.com/d2cTool/goprofile/internal/services"
	"github.com/d2cTool/goprofile/internal/storage"
)

type Deps struct {
	Cfg      config.Config
	Pool     *pgxpool.Pool
	Repo     *repository.AvatarRepository
	S3       *storage.S3
	Producer *broker.Producer
	Service  *services.AvatarService
	Log      *slog.Logger
}

func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*Deps, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	if err := migrate.Up(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	s3, err := storage.NewS3(cfg)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if err := s3.EnsureBucket(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("s3 bucket: %w", err)
	}

	if err := broker.EnsureTopics(ctx, cfg); err != nil {
		log.Warn("kafka topics", "err", err)
	}

	repo := repository.NewAvatarRepository(pool)
	producer := broker.NewProducer(cfg)
	svc := services.NewAvatarService(repo, s3, producer, cfg.PublicBaseURL, cfg.MaxUploadBytes)

	return &Deps{
		Cfg:      cfg,
		Pool:     pool,
		Repo:     repo,
		S3:       s3,
		Producer: producer,
		Service:  svc,
		Log:      log,
	}, nil
}

func (d *Deps) Close() {
	if d.Producer != nil {
		_ = d.Producer.Close()
	}
	if d.Pool != nil {
		d.Pool.Close()
	}
}
