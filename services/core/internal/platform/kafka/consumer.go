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

// Consumer reads from a Kafka topic inside a consumer group.
type Consumer struct {
	reader  *kafka.Reader
	log     *slog.Logger
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

// Run blocks and processes messages until ctx is cancelled.
// Manual offset commit — offset is committed only after the handler succeeds.
func (c *Consumer) Run(ctx context.Context, handler MessageHandler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // graceful shutdown
			}
			return fmt.Errorf("fetching kafka message: %w", err)
		}

		var event Event
		if err = json.Unmarshal(msg.Value, &event); err != nil {
			c.log.ErrorContext(ctx, "failed to unmarshal kafka message",
				"error", err,
				"offset", msg.Offset,
			)
			// Commit and skip malformed messages to avoid infinite loops.
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		if err = handler(ctx, event); err != nil {
			c.log.ErrorContext(ctx, "kafka message handler error",
				"error", err,
				"event_type", event.EventType,
				"event_id", event.EventID,
			)
			// Do not commit — message will be redelivered (at-least-once).
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
