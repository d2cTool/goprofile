package observability

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestKafkaHeaderCarrier(t *testing.T) {
	t.Parallel()
	c := KafkaHeaderCarrier{{Key: "event_id", Value: []byte("e1")}}
	if c.Get("event_id") != "e1" {
		t.Fatal("get")
	}
	c.Set("traceparent", "tp")
	if c.Get("traceparent") != "tp" {
		t.Fatal("set")
	}
	c.Set("traceparent", "tp2")
	if c.Get("Traceparent") != "tp2" {
		t.Fatal("overwrite")
	}
	if len(c.Keys()) != 2 {
		t.Fatalf("keys %v", c.Keys())
	}
}

func TestKafkaPropagate(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))
	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	otel.SetTracerProvider(tp)

	ctx, span := tp.Tracer("test").Start(context.Background(), "produce")
	headers := InjectKafka(ctx, []kafka.Header{{Key: "event_id", Value: []byte("e1")}})
	want := span.SpanContext().TraceID()
	span.End()

	got := trace.SpanContextFromContext(ExtractKafka(context.Background(), headers))
	if !got.IsValid() || got.TraceID() != want {
		t.Fatalf("propagated %s want %s", got.TraceID(), want)
	}
}
