package broker

import (
	"testing"

	"github.com/segmentio/kafka-go"
)

func TestEventIDFrom(t *testing.T) {
	t.Parallel()
	msg := kafka.Message{Headers: []kafka.Header{{Key: "event_id", Value: []byte("upload:1")}}}
	if got := EventIDFrom(msg, "fallback"); got != "upload:1" {
		t.Fatalf("%s", got)
	}
	if got := EventIDFrom(kafka.Message{}, "fallback"); got != "fallback" {
		t.Fatalf("%s", got)
	}
}
