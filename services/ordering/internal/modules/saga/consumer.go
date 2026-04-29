package saga

import (
	"context"
	"log/slog"

	"github.com/foodsea/ordering/internal/platform/kafka"
)

// ReplyConsumer listens on saga.replies for audit and cross-system visibility.
// In this implementation it logs every reply; extend by registering handlers
// in the Subscriber for specific event types.
type ReplyConsumer struct {
	consumer *kafka.Consumer
	log      *slog.Logger
}

// NewReplyConsumer creates a ReplyConsumer.
func NewReplyConsumer(consumer *kafka.Consumer, log *slog.Logger) *ReplyConsumer {
	return &ReplyConsumer{consumer: consumer, log: log}
}

// Run starts the consumer loop. Blocks until ctx is cancelled.
func (rc *ReplyConsumer) Run(ctx context.Context) error {
	return rc.consumer.Run(ctx, rc.handleReply)
}

func (rc *ReplyConsumer) handleReply(ctx context.Context, e kafka.Event) error {
	rc.log.InfoContext(ctx, "saga reply received",
		"event_type", e.EventType,
		"event_id", e.EventID,
		"payload_size", len(e.Payload),
	)
	return nil
}
