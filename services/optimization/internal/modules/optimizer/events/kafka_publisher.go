package events

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
	"github.com/foodsea/optimization/internal/platform/kafka"
)

const source = "optimization-service"

type producer interface {
	Publish(ctx context.Context, event kafka.Event) error
}

// KafkaPublisher emits optimization domain events to optimization.events topic.
type KafkaPublisher struct {
	producer producer
	log      *slog.Logger
}

func NewKafkaPublisher(p *kafka.Producer, log *slog.Logger) *KafkaPublisher {
	return &KafkaPublisher{producer: p, log: log}
}

func NewKafkaPublisherWithInterface(p producer, log *slog.Logger) *KafkaPublisher {
	return &KafkaPublisher{producer: p, log: log}
}

func (p *KafkaPublisher) ResultCreated(ctx context.Context, result *domain.OptimizationResult) error {
	payload := map[string]any{
		"result_id": result.ID.String(),
		"user_id":   result.UserID.String(),
		"total":     result.TotalKopecks,
		"savings":   result.SavingsKopecks,
	}
	return p.publish(ctx, "optimization.result_created", payload)
}

func (p *KafkaPublisher) ResultLocked(ctx context.Context, resultID uuid.UUID) error {
	return p.publish(ctx, "optimization.result_locked", map[string]any{"result_id": resultID.String()})
}

func (p *KafkaPublisher) ResultUnlocked(ctx context.Context, resultID uuid.UUID) error {
	return p.publish(ctx, "optimization.result_unlocked", map[string]any{"result_id": resultID.String()})
}

func (p *KafkaPublisher) publish(ctx context.Context, eventType string, payload any) error {
	event, err := kafka.NewEvent(eventType, source, payload)
	if err != nil {
		return fmt.Errorf("creating event %q: %w", eventType, err)
	}
	if err = p.producer.Publish(ctx, event); err != nil {
		return err
	}
	p.log.InfoContext(ctx, "optimization event published", "event_type", eventType, "event_id", event.EventID)
	return nil
}
