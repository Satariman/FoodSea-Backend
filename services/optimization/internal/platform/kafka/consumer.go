package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

// MessageHandler processes a single decoded Event. At-least-once: handler is
// called before the offset is committed. If it returns an error the offset is
// NOT committed and the message may be redelivered.
type MessageHandler func(ctx context.Context, event Event) error

// Reader is the minimal interface over kafka.Reader needed for testing.
type Reader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// Consumer reads from a Kafka topic inside a consumer group.
type Consumer struct {
	reader Reader
	log    *slog.Logger
}

// NewConsumer creates a Consumer for the given topic/group.
func NewConsumer(brokers []string, topic, groupID string, log *slog.Logger) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   topic,
			GroupID: groupID,
		}),
		log: log,
	}
}

// newConsumerWithReader creates a Consumer with a custom reader (for testing).
func newConsumerWithReader(r Reader, log *slog.Logger) *Consumer {
	return &Consumer{reader: r, log: log}
}

// Run blocks and processes messages until ctx is cancelled.
// Manual offset commit — offset is committed only after the handler succeeds.
func (c *Consumer) Run(ctx context.Context, handler MessageHandler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetching kafka message: %w", err)
		}

		var event Event
		if err = json.Unmarshal(msg.Value, &event); err != nil {
			c.log.ErrorContext(ctx, "failed to unmarshal kafka message",
				"error", err,
				"offset", msg.Offset,
			)
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		if err = handler(ctx, event); err != nil {
			c.log.ErrorContext(ctx, "kafka message handler error",
				"error", err,
				"event_type", event.EventType,
				"event_id", event.EventID,
			)
			continue
		}

		if err = c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.WarnContext(ctx, "failed to commit kafka offset",
				"error", err,
				"offset", msg.Offset,
			)
		}
	}
}

// Close shuts down the reader.
func (c *Consumer) Close() error { return c.reader.Close() }

// Subscriber wraps Consumer and routes messages by event_type to registered handlers.
// Unregistered event types are logged at WARN and their offsets are committed.
type Subscriber struct {
	consumer *Consumer
	handlers map[string]MessageHandler
	log      *slog.Logger
}

// NewSubscriber creates a Subscriber for the given topic/group.
func NewSubscriber(brokers []string, topic, groupID string, log *slog.Logger) *Subscriber {
	return &Subscriber{
		consumer: NewConsumer(brokers, topic, groupID, log),
		handlers: make(map[string]MessageHandler),
		log:      log,
	}
}

// newSubscriberWithReader creates a Subscriber with a custom reader (for testing).
func newSubscriberWithReader(r Reader, log *slog.Logger) *Subscriber {
	return &Subscriber{
		consumer: newConsumerWithReader(r, log),
		handlers: make(map[string]MessageHandler),
		log:      log,
	}
}

// Register adds a handler for the given event_type.
func (s *Subscriber) Register(eventType string, handler MessageHandler) {
	s.handlers[eventType] = handler
}

// Run starts the consumer loop, routing each event to its handler.
func (s *Subscriber) Run(ctx context.Context) error {
	return s.consumer.Run(ctx, func(ctx context.Context, event Event) error {
		handler, ok := s.handlers[event.EventType]
		if !ok {
			s.log.WarnContext(ctx, "no handler for event type",
				"event_type", event.EventType,
				"event_id", event.EventID,
			)
			return nil // commit offset and skip unknown events
		}
		return handler(ctx, event)
	})
}

// Close shuts down the underlying consumer.
func (s *Subscriber) Close() error { return s.consumer.Close() }
