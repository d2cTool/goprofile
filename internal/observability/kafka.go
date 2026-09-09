package observability

import (
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

type KafkaHeaderCarrier []kafka.Header

func (c *KafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c {
		if strings.EqualFold(h.Key, key) {
			return string(h.Value)
		}
	}
	return ""
}

func (c *KafkaHeaderCarrier) Set(key, value string) {
	for i, h := range *c {
		if strings.EqualFold(h.Key, key) {
			(*c)[i].Value = []byte(value)
			return
		}
	}
	*c = append(*c, kafka.Header{Key: key, Value: []byte(value)})
}

func (c *KafkaHeaderCarrier) Keys() []string {
	out := make([]string, 0, len(*c))
	for _, h := range *c {
		out = append(out, h.Key)
	}
	return out
}

func InjectKafka(ctx context.Context, headers []kafka.Header) []kafka.Header {
	carrier := KafkaHeaderCarrier(headers)
	otel.GetTextMapPropagator().Inject(ctx, &carrier)
	return []kafka.Header(carrier)
}

func ExtractKafka(ctx context.Context, headers []kafka.Header) context.Context {
	carrier := KafkaHeaderCarrier(headers)
	return otel.GetTextMapPropagator().Extract(ctx, &carrier)
}
