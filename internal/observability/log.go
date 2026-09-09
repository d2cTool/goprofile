package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/trace"
)

type traceHandler struct {
	slog.Handler
}

func NewLogger(service string, level slog.Level) *slog.Logger {
	jsonH := &traceHandler{Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})}
	if lp := currentLogProvider(); lp != nil {
		otelH := &traceHandler{Handler: otelslog.NewHandler(service, otelslog.WithLoggerProvider(lp))}
		return slog.New(slog.NewMultiHandler(jsonH, otelH)).With("service", service)
	}
	return slog.New(jsonH).With("service", service)
}

func ParseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (h *traceHandler) Handle(ctx context.Context, rec slog.Record) error {
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		rec.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, rec)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}

func Logger(ctx context.Context) *slog.Logger {
	return slog.Default()
}
