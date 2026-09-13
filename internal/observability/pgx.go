package observability

import (
	"context"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type pgxSpanKey struct{}

type PGXTracer struct{}

func NewPGXTracer() *PGXTracer {
	return &PGXTracer{}
}

func (t *PGXTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	sql := data.SQL
	if len(sql) > 512 {
		sql = sql[:512]
	}
	ctx, span := Tracer().Start(ctx, "db.query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "query"),
			attribute.String("db.statement", sql),
		),
	)
	return context.WithValue(ctx, pgxSpanKey{}, span)
}

func (t *PGXTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, _ := ctx.Value(pgxSpanKey{}).(trace.Span)
	if span == nil {
		return
	}
	RecordError(span, data.Err)
	span.End()
}
