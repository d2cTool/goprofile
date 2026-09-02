package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/d2cTool/goprofile/internal/api"
	"github.com/d2cTool/goprofile/internal/app"
	"github.com/d2cTool/goprofile/internal/config"
	"github.com/d2cTool/goprofile/internal/handlers"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	deps, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Error("bootstrap", "err", err)
		os.Exit(1)
	}
	defer deps.Close()

	avatars := handlers.NewAvatarHandler(deps.Service, cfg.MaxUploadBytes)
	webh, err := handlers.NewWebHandler(deps.Service, cfg.MaxUploadBytes)
	if err != nil {
		log.Error("web", "err", err)
		os.Exit(1)
	}
	health := &handlers.HealthHandler{DB: deps.Repo, S3: deps.S3, Kafka: deps.Producer}

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: api.NewRouter(cfg, avatars, health, webh),
	}

	go func() {
		log.Info("server started", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout())
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}
