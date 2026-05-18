package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

type messageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// Producer publishes Events to a single Kafka topic.
type Producer struct {
	writer messageWriter
	log    *slog.Logger
}

// NewProducer creates a new Producer. brokers is the list of Kafka broker addresses.
func NewProducer(brokers []string, topic string, log *slog.Logger) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
		log: log,
	}
}

// Publish serialises and writes an Event to Kafka.
func (p *Producer) Publish(ctx context.Context, event Event) error {
	return p.PublishWithKey(ctx, event.EventID, event)
}

// PublishWithKey serialises and writes an Event to Kafka using the provided key.
func (p *Producer) PublishWithKey(ctx context.Context, key string, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event %q: %w", event.EventType, err)
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: data,
	}

	if err = p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publishing event %q: %w", event.EventType, err)
	}

	p.log.InfoContext(ctx, "event published",
		"event_type", event.EventType,
		"event_id", event.EventID,
		"message_key", key,
	)
	return nil
}

// Close shuts down the underlying Kafka writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}
