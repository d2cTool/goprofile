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
	"github.com/d2cTool/goprofile/internal/observability"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.ServiceName == "" || cfg.ServiceName == "gophprofile" {
		cfg.ServiceName = "gophprofile-server"
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	otelStop, err := observability.Setup(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("otel", "err", err)
		os.Exit(1)
	}
	log := observability.NewLogger(cfg.ServiceName, observability.ParseLevel(cfg.LogLevel))
	slog.SetDefault(log)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout())
		defer cancel()
		if err := otelStop(shutdownCtx); err != nil {
			log.Error("otel shutdown", "err", err)
		}
	}()

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
