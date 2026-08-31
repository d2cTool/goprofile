package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/d2cTool/goprofile/internal/app"
	"github.com/d2cTool/goprofile/internal/broker"
	"github.com/d2cTool/goprofile/internal/config"
	"github.com/d2cTool/goprofile/internal/worker"
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

	uploadC := broker.NewConsumer(cfg, cfg.TopicUpload)
	deleteC := broker.NewConsumer(cfg, cfg.TopicDelete)
	defer uploadC.Close()
	defer deleteC.Close()

	log.Info("worker started", "upload_topic", cfg.TopicUpload, "delete_topic", cfg.TopicDelete)
	if err := worker.New(deps.Service, uploadC, deleteC, log).Run(ctx); err != nil {
		log.Error("worker", "err", err)
		os.Exit(1)
	}
}
