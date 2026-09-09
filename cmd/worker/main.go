package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/d2cTool/goprofile/internal/app"
	"github.com/d2cTool/goprofile/internal/broker"
	"github.com/d2cTool/goprofile/internal/config"
	"github.com/d2cTool/goprofile/internal/observability"
	"github.com/d2cTool/goprofile/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.ServiceName == "" || cfg.ServiceName == "gophprofile" {
		cfg.ServiceName = "gophprofile-worker"
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

	metricsSrv := &http.Server{Addr: cfg.MetricsAddr, Handler: observability.MetricsHandler()}
	go func() {
		log.Info("metrics started", "addr", cfg.MetricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics http", "err", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout())
		defer cancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	uploadC := broker.NewConsumer(cfg, cfg.TopicUpload)
	deleteC := broker.NewConsumer(cfg, cfg.TopicDelete)
	defer func() { _ = uploadC.Close() }()
	defer func() { _ = deleteC.Close() }()

	log.Info("worker started", "upload_topic", cfg.TopicUpload, "delete_topic", cfg.TopicDelete)
	if err := worker.New(deps.Service, uploadC, deleteC, log).Run(ctx); err != nil {
		log.Error("worker", "err", err)
		os.Exit(1)
	}
}
