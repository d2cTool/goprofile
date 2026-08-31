package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/d2cTool/goprofile/internal/config"
	"github.com/d2cTool/goprofile/internal/domain"
)

type Producer struct {
	upload  *kafka.Writer
	delete  *kafka.Writer
	process *kafka.Writer
	brokers []string
}

func NewProducer(cfg config.Config) *Producer {
	return &Producer{
		upload:  newWriter(cfg.KafkaBrokers, cfg.TopicUpload),
		delete:  newWriter(cfg.KafkaBrokers, cfg.TopicDelete),
		process: newWriter(cfg.KafkaBrokers, cfg.TopicProcess),
		brokers: cfg.KafkaBrokers,
	}
}

func newWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		BatchTimeout: 10 * time.Millisecond,
	}
}

func (p *Producer) Close() error {
	var first error
	for _, w := range []*kafka.Writer{p.upload, p.delete, p.process} {
		if err := w.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (p *Producer) Ping(ctx context.Context) error {
	d := kafka.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", p.brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Brokers()
	return err
}

func (p *Producer) PublishUpload(ctx context.Context, event domain.AvatarUploadEvent) error {
	return writeJSON(ctx, p.upload, event.AvatarID, event.EventID, event)
}

func (p *Producer) PublishDelete(ctx context.Context, event domain.AvatarDeleteEvent) error {
	return writeJSON(ctx, p.delete, event.AvatarID, event.EventID, event)
}

func (p *Producer) PublishProcess(ctx context.Context, event domain.AvatarProcessEvent) error {
	return writeJSON(ctx, p.process, event.AvatarID, event.EventID, event)
}

func writeJSON(ctx context.Context, w *kafka.Writer, key, eventID string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	msg := kafka.Message{
		Key:   []byte(key),
		Value: body,
		Time:  time.Now().UTC(),
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte(eventID)},
		},
	}
	if err := w.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publish %s: %w", w.Topic, err)
	}
	return nil
}

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(cfg config.Config, topic string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        cfg.KafkaBrokers,
			GroupID:        cfg.KafkaGroup,
			Topic:          topic,
			MinBytes:       1,
			MaxBytes:       10 << 20,
			MaxWait:        500 * time.Millisecond,
			CommitInterval: 0,
			StartOffset:    kafka.FirstOffset,
		}),
	}
}

func (c *Consumer) Fetch(ctx context.Context) (kafka.Message, error) {
	return c.reader.FetchMessage(ctx)
}

func (c *Consumer) Commit(ctx context.Context, msg kafka.Message) error {
	return c.reader.CommitMessages(ctx, msg)
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func EnsureTopics(ctx context.Context, cfg config.Config) error {
	topics := []string{cfg.TopicUpload, cfg.TopicDelete, cfg.TopicProcess}
	var last error
	for _, broker := range cfg.KafkaBrokers {
		if err := createTopics(ctx, broker, topics); err != nil {
			last = err
			continue
		}
		return nil
	}
	if last == nil {
		return fmt.Errorf("no kafka brokers configured")
	}
	return last
}

func createTopics(ctx context.Context, broker string, topics []string) error {
	dialer := &kafka.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", broker)
	if err != nil {
		return err
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return err
	}
	ctrlAddr := net.JoinHostPort(controller.Host, fmt.Sprintf("%d", controller.Port))
	ctrl, err := dialer.DialContext(ctx, "tcp", ctrlAddr)
	if err != nil {
		return err
	}
	defer ctrl.Close()

	configs := make([]kafka.TopicConfig, 0, len(topics))
	for _, t := range topics {
		configs = append(configs, kafka.TopicConfig{
			Topic:             t,
			NumPartitions:     3,
			ReplicationFactor: 1,
		})
	}
	return ctrl.CreateTopics(configs...)
}

func EventIDFrom(msg kafka.Message, fallback string) string {
	for _, h := range msg.Headers {
		if h.Key == "event_id" && len(h.Value) > 0 {
			return string(h.Value)
		}
	}
	return fallback
}
