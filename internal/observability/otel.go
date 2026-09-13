package observability

import (
	"context"
	"errors"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

const TracerName = "gophprofile"

var (
	logMu       sync.RWMutex
	logProvider *sdklog.LoggerProvider
)

func Setup(ctx context.Context, service, endpoint string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	if strings.TrimSpace(endpoint) == "" {
		setLogProvider(nil)
		return func(context.Context) error { return nil }, nil
	}

	host, insecure := parseOTLPEndpoint(endpoint)
	traceOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(host)}
	logOpts := []otlploghttp.Option{otlploghttp.WithEndpoint(host)}
	if insecure {
		traceOpts = append(traceOpts, otlptracehttp.WithInsecure())
		logOpts = append(logOpts, otlploghttp.WithInsecure())
	}

	traceExp, err := otlptracehttp.New(ctx, traceOpts...)
	if err != nil {
		return nil, err
	}
	logExp, err := otlploghttp.New(ctx, logOpts...)
	if err != nil {
		_ = traceExp.Shutdown(ctx)
		return nil, err
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(service),
		attribute.String("container", service),
	))
	if err != nil {
		_ = traceExp.Shutdown(ctx)
		_ = logExp.Shutdown(ctx)
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	setLogProvider(lp)

	return func(ctx context.Context) error {
		err := errors.Join(tp.Shutdown(ctx), lp.Shutdown(ctx))
		setLogProvider(nil)
		return err
	}, nil
}

func setLogProvider(lp *sdklog.LoggerProvider) {
	logMu.Lock()
	logProvider = lp
	logMu.Unlock()
}

func currentLogProvider() *sdklog.LoggerProvider {
	logMu.RLock()
	defer logMu.RUnlock()
	return logProvider
}

func Tracer() trace.Tracer {
	return otel.Tracer(TracerName)
}

func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, name)
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx, span
}

func RecordError(span trace.Span, err error) {
	if err == nil || !span.IsRecording() {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func parseOTLPEndpoint(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	insecure := true
	switch {
	case strings.HasPrefix(raw, "https://"):
		insecure = false
		raw = strings.TrimPrefix(raw, "https://")
	case strings.HasPrefix(raw, "http://"):
		raw = strings.TrimPrefix(raw, "http://")
	}
	return strings.TrimRight(raw, "/"), insecure
}
