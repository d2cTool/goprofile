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
	upload   kafkaCursor
	delete   kafkaCursor
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 3)
	run := func(fn func() error) {
		go func() {
			err := fn()
			if err != nil {
				cancel()
			}
			errCh <- err
		}()
	}
	run(func() error { return w.consume(ctx, w.upload, w.handleUpload) })
	run(func() error { return w.consume(ctx, w.delete, w.handleDelete) })
	run(func() error { return w.maintenance(ctx) })

	var first error
	for i := 0; i < 3; i++ {
		if err := <-errCh; err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (w *Worker) maintenance(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	w.runMaintenance(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.runMaintenance(ctx)
		}
	}
}

func (w *Worker) runMaintenance(ctx context.Context) {
	if err := w.svc.FlushOutbox(ctx); err != nil {
		w.log.Warn("flush outbox", "err", err)
	}
	if err := w.svc.RecoverStuck(ctx); err != nil {
		w.log.Warn("recover stuck", "err", err)
	}
}

type kafkaCursor interface {
	Fetch(ctx context.Context) (kafka.Message, error)
	Commit(ctx context.Context, msg kafka.Message) error
}

func (w *Worker) consume(ctx context.Context, c kafkaCursor, handle func(context.Context, kafka.Message) error) error {
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
			if domain.IsPermanent(err) {
				w.log.Error("dropping poison message", "topic", msg.Topic, "offset", msg.Offset, "err", err)
			} else {
				w.log.Error("dropping after retries", "topic", msg.Topic, "offset", msg.Offset, "err", err)
			}
			if cerr := c.Commit(ctx, msg); cerr != nil {
				return fmt.Errorf("commit dropped message: %w", cerr)
			}
			continue
		}
		if err := c.Commit(ctx, msg); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
	}
}

func (w *Worker) handleUpload(ctx context.Context, msg kafka.Message) error {
	var event domain.AvatarUploadEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return domain.Permanent(fmt.Errorf("decode upload event: %w", err))
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
		return domain.Permanent(fmt.Errorf("decode delete event: %w", err))
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
		if domain.IsPermanent(last) || i == w.attempts-1 {
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
