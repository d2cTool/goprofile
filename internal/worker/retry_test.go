package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

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
}
