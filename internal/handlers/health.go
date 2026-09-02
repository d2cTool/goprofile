package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	DB    Pinger
	S3    Pinger
	Kafka Pinger
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	type result struct {
		name string
		err  error
	}

	checks := map[string]Pinger{
		"postgres": h.DB,
		"s3":       h.S3,
		"kafka":    h.Kafka,
	}

	ch := make(chan result, len(checks))
	var wg sync.WaitGroup
	for name, p := range checks {
		wg.Add(1)
		go func(name string, p Pinger) {
			defer wg.Done()
			if p == nil {
				ch <- result{name: name, err: errMissing}
				return
			}
			ch <- result{name: name, err: p.Ping(ctx)}
		}(name, p)
	}
	wg.Wait()
	close(ch)

	components := map[string]string{}
	status := "ok"
	code := http.StatusOK
	for res := range ch {
		if res.err != nil {
			components[res.name] = "fail"
			status = "fail"
			code = http.StatusServiceUnavailable
			continue
		}
		components[res.name] = "ok"
	}

	writeJSON(w, code, map[string]any{
		"status":     status,
		"components": components,
	})
}

var errMissing = errString("component is not configured")

type errString string

func (e errString) Error() string { return string(e) }
