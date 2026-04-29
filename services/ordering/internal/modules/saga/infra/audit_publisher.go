package infra

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/platform/kafka"
)

// AuditPublisher writes saga command/reply events to Kafka for audit trail and monitoring.
// It implements domain.SagaAuditPublisher.
// Errors are non-fatal — the orchestrator logs them and continues.
type AuditPublisher struct {
	commandProducer *kafka.Producer // saga.commands topic
	replyProducer   *kafka.Producer // saga.replies topic
	log             *slog.Logger
}

// NewAuditPublisher creates an AuditPublisher.
func NewAuditPublisher(commandProducer, replyProducer *kafka.Producer, log *slog.Logger) *AuditPublisher {
	return &AuditPublisher{
		commandProducer: commandProducer,
		replyProducer:   replyProducer,
		log:             log,
	}
}

// PublishCommand writes a saga command to the saga.commands topic.
func (a *AuditPublisher) PublishCommand(ctx context.Context, sagaID uuid.UUID, step int8, cmdType string, payload any) error {
	event, err := kafka.NewEvent(cmdType, "ordering-saga", map[string]any{
		"saga_id": sagaID.String(),
		"step":    step,
		"data":    payload,
	})
	if err != nil {
		return fmt.Errorf("build command event %q: %w", cmdType, err)
	}
	if err = a.commandProducer.Publish(ctx, event); err != nil {
		a.log.WarnContext(ctx, "saga command publish failed",
			"saga_id", sagaID,
			"cmd_type", cmdType,
			"error", err,
		)
		return err
	}
	return nil
}

// PublishReply writes a saga reply to the saga.replies topic.
func (a *AuditPublisher) PublishReply(ctx context.Context, sagaID uuid.UUID, step int8, replyStatus string, payload any) error {
	eventType := fmt.Sprintf("saga.reply.%s", replyStatus)
	event, err := kafka.NewEvent(eventType, "ordering-saga", map[string]any{
		"saga_id": sagaID.String(),
		"step":    step,
		"status":  replyStatus,
		"data":    payload,
	})
	if err != nil {
		return fmt.Errorf("build reply event: %w", err)
	}
	if err = a.replyProducer.Publish(ctx, event); err != nil {
		a.log.WarnContext(ctx, "saga reply publish failed",
			"saga_id", sagaID,
			"step", step,
			"status", replyStatus,
			"error", err,
		)
		return err
	}
	return nil
}
