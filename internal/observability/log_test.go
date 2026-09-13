package observability

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	if ParseLevel("debug") != slog.LevelDebug {
		t.Fatal("debug")
	}
	if ParseLevel("warn") != slog.LevelWarn {
		t.Fatal("warn")
	}
	if ParseLevel("") != slog.LevelInfo {
		t.Fatal("default")
	}
}

func TestTraceHandlerAddsIDs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(&traceHandler{Handler: slog.NewJSONHandler(&buf, nil)})

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	ctx, span := tp.Tracer("test").Start(context.Background(), "unit")
	logger.InfoContext(ctx, "uploading avatar", "user_id", "alice", "file_size", 128)
	span.End()

	out := buf.String()
	if !strings.Contains(out, `"msg":"uploading avatar"`) || !strings.Contains(out, `"user_id":"alice"`) {
		t.Fatalf("log %s", out)
	}
	if !strings.Contains(out, `"trace_id":"`) || !strings.Contains(out, `"span_id":"`) {
		t.Fatalf("missing correlation %s", out)
	}
}

func TestSetupEmptyEndpointLeavesJSONLogger(t *testing.T) {
	stop, err := Setup(context.Background(), "gophprofile-test", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop(context.Background()) })
	if currentLogProvider() != nil {
		t.Fatal("expected no log provider")
	}
	if NewLogger("gophprofile-test", slog.LevelInfo) == nil {
		t.Fatal("logger")
	}
}

func TestParseOTLPEndpoint(t *testing.T) {
	t.Parallel()
	host, insecure := parseOTLPEndpoint("http://otel-collector:4318")
	if host != "otel-collector:4318" || !insecure {
		t.Fatalf("%s %v", host, insecure)
	}
	host, insecure = parseOTLPEndpoint("https://otel.example:4318/")
	if host != "otel.example:4318" || insecure {
		t.Fatalf("%s %v", host, insecure)
	}
}

func TestStatusOK(t *testing.T) {
	t.Parallel()
	if StatusOK(nil) != "ok" || StatusOK(context.Canceled) != "error" {
		t.Fatal("status")
	}
}
