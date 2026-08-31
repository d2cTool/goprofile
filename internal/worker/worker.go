package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/d2cTool/goprofile/internal/broker"
	"github.com/d2cTool/goprofile/internal/domain"
	"github.com/d2cTool/goprofile/internal/services"
)

type Worker struct {
	svc      *services.AvatarService
	upload   *broker.Consumer
	delete   *broker.Consumer
	log      *slog.Logger
	attempts int
}

func New(svc *services.AvatarService, upload, deleteC *broker.Consumer, log *slog.Logger) *Worker {
	if log == nil {
		log = slog.Default()
	}
	return &Worker{
		svc:      svc,
		upload:   upload,
		delete:   deleteC,
		log:      log,
		attempts: 5,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	errCh := make(chan error, 2)
	go func() {
		errCh <- w.consume(ctx, w.upload, w.handleUpload)
	}()
	go func() {
		errCh <- w.consume(ctx, w.delete, w.handleDelete)
	}()

	var first error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (w *Worker) consume(ctx context.Context, c *broker.Consumer, handle func(context.Context, kafka.Message) error) error {
	for {
		msg, err := c.Fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if err := w.withRetry(ctx, func() error {
			return handle(ctx, msg)
		}); err != nil {
			w.log.Error("handle message failed", "topic", msg.Topic, "offset", msg.Offset, "err", err)
			continue
		}
		if err := c.Commit(ctx, msg); err != nil {
			w.log.Error("commit failed", "topic", msg.Topic, "err", err)
		}
	}
}

func (w *Worker) handleUpload(ctx context.Context, msg kafka.Message) error {
	var event domain.AvatarUploadEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("decode upload event: %w", err)
	}
	event.EventID = broker.EventIDFrom(msg, event.EventID)
	if event.EventID == "" {
		event.EventID = domain.UploadEventID(event.AvatarID)
	}
	return w.svc.ProcessUpload(ctx, event)
}

func (w *Worker) handleDelete(ctx context.Context, msg kafka.Message) error {
	var event domain.AvatarDeleteEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("decode delete event: %w", err)
	}
	event.EventID = broker.EventIDFrom(msg, event.EventID)
	if event.EventID == "" {
		event.EventID = domain.DeleteEventID(event.AvatarID)
	}
	return w.svc.ProcessDelete(ctx, event)
}

func (w *Worker) withRetry(ctx context.Context, fn func() error) error {
	backoff := 200 * time.Millisecond
	var last error
	for i := 0; i < w.attempts; i++ {
		last = fn()
		if last == nil {
			return nil
		}
		if i == w.attempts-1 {
			break
		}
		w.log.Warn("retrying", "attempt", i+1, "err", last)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	return last
}
