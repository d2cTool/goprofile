package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/d2cTool/goprofile/internal/domain"
	"github.com/d2cTool/goprofile/internal/services"
)

func TestNewWorker(t *testing.T) {
	w := New(nil, nil, nil, nil)
	if w == nil || w.attempts != 5 || w.log == nil {
		t.Fatal("defaults")
	}
}

func TestWithRetry(t *testing.T) {
	w := &Worker{log: slog.New(slog.NewTextHandler(io.Discard, nil)), attempts: 3}
	n := 0
	err := w.withRetry(context.Background(), func() error {
		n++
		if n < 3 {
			return errors.New("tmp")
		}
		return nil
	})
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}

	n = 0
	err = w.withRetry(context.Background(), func() error {
		n++
		return errors.New("always")
	})
	if err == nil || n != 3 {
		t.Fatalf("expected fail n=%d err=%v", n, err)
	}

	n = 0
	err = w.withRetry(context.Background(), func() error {
		n++
		return domain.Permanent(errors.New("bad json"))
	})
	if !domain.IsPermanent(err) || n != 1 {
		t.Fatalf("permanent should not retry n=%d err=%v", n, err)
	}
}

type stubCursor struct {
	mu      sync.Mutex
	msgs    []kafka.Message
	commits []kafka.Message
	fetch   error
	commit  error
}

func (s *stubCursor) Fetch(ctx context.Context) (kafka.Message, error) {
	s.mu.Lock()
	if s.fetch != nil {
		err := s.fetch
		s.mu.Unlock()
		return kafka.Message{}, err
	}
	if len(s.msgs) == 0 {
		s.mu.Unlock()
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}
	msg := s.msgs[0]
	s.msgs = s.msgs[1:]
	s.mu.Unlock()
	return msg, nil
}

func (s *stubCursor) Commit(_ context.Context, msg kafka.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.commit != nil {
		return s.commit
	}
	s.commits = append(s.commits, msg)
	return nil
}

func (s *stubCursor) committed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.commits)
}

func TestConsumeCommitsPoisonAndExhaustedRetries(t *testing.T) {
	cur := &stubCursor{msgs: []kafka.Message{
		{Topic: "avatar.uploaded", Offset: 1, Value: []byte("{")},
		{Topic: "avatar.uploaded", Offset: 2, Value: []byte(`{"avatar_id":"x"}`)},
	}}
	w := &Worker{
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		attempts: 2,
		svc:      services.NewAvatarService(&claimRepo{}, &delObj{}, nopPub{}, "http://x", 10),
	}
	n := 0
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- w.consume(ctx, cur, func(ctx context.Context, msg kafka.Message) error {
			if msg.Offset == 1 {
				return w.handleUpload(ctx, msg)
			}
			n++
			return errors.New("s3 timeout")
		})
	}()

	deadline := time.Now().Add(time.Second)
	for cur.committed() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if cur.committed() != 2 {
		t.Fatalf("commits %d", cur.committed())
	}
	if n != 2 {
		t.Fatalf("transient retries %d", n)
	}
}

func TestRunCancelsOnFirstError(t *testing.T) {
	w := &Worker{
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		attempts: 1,
		svc:      services.NewAvatarService(&claimRepo{}, &delObj{}, nopPub{}, "http://x", 10),
		upload:   &stubCursor{fetch: errors.New("broker down")},
		delete:   &stubCursor{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	started := time.Now()
	err := w.Run(ctx)
	if err == nil || err.Error() != "broker down" {
		t.Fatalf("err=%v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("Run must stop siblings instead of waiting for shutdown timeout")
	}
}
