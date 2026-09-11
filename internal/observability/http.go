package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

func HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := Tracer().Start(ctx, r.Method+" "+r.URL.Path,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.URLPath(r.URL.Path),
			),
		)
		defer span.End()

		if reqID := chimw.GetReqID(ctx); reqID != "" {
			ctx = ContextWithLogger(ctx, Logger(ctx).With("request_id", reqID))
		}

		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		req := r.WithContext(ctx)
		next.ServeHTTP(ww, req)

		route := r.URL.Path
		if rc := chi.RouteContext(req.Context()); rc != nil && rc.RoutePattern() != "" {
			route = rc.RoutePattern()
		}
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		span.SetName(r.Method + " " + route)
		span.SetAttributes(
			semconv.HTTPResponseStatusCode(status),
			attribute.String("http.route", route),
		)
		if status >= 500 {
			span.SetStatus(codes.Error, http.StatusText(status))
		}

		HTTPRequests.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
		HTTPDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		Logger(ctx).InfoContext(ctx, "http request",
			"method", r.Method,
			"path", route,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
