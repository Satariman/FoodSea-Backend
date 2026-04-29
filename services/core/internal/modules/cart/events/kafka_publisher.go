package events

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/platform/kafka"
)

const source = "core-service"

// producer is a narrow interface over kafka.Producer for testability.
type producer interface {
	Publish(ctx context.Context, event kafka.Event) error
}

type itemAddedPayload struct {
	UserID    string `json:"user_id"`
	ProductID string `json:"product_id"`
	Quantity  int16  `json:"quantity"`
}

type itemRemovedPayload struct {
	UserID    string `json:"user_id"`
	ProductID string `json:"product_id"`
}

type clearedPayload struct {
	UserID string `json:"user_id"`
}

// KafkaPublisher implements domain.CartEventPublisher via kafka.Producer.
type KafkaPublisher struct {
	producer producer
}

func NewKafkaPublisher(p *kafka.Producer) *KafkaPublisher {
	return &KafkaPublisher{producer: p}
}

// NewKafkaPublisherWithInterface creates a publisher from any producer implementation (used in tests).
func NewKafkaPublisherWithInterface(p producer) *KafkaPublisher {
	return &KafkaPublisher{producer: p}
}

func (p *KafkaPublisher) ItemAdded(ctx context.Context, userID, productID uuid.UUID, quantity int16) error {
	return p.publish(ctx, "cart.item_added", itemAddedPayload{
		UserID:    userID.String(),
		ProductID: productID.String(),
		Quantity:  quantity,
	})
}

func (p *KafkaPublisher) ItemUpdated(ctx context.Context, userID, productID uuid.UUID, quantity int16) error {
	return p.publish(ctx, "cart.item_updated", itemAddedPayload{
		UserID:    userID.String(),
		ProductID: productID.String(),
		Quantity:  quantity,
	})
}

func (p *KafkaPublisher) ItemRemoved(ctx context.Context, userID, productID uuid.UUID) error {
	return p.publish(ctx, "cart.item_removed", itemRemovedPayload{
		UserID:    userID.String(),
		ProductID: productID.String(),
	})
}

func (p *KafkaPublisher) Cleared(ctx context.Context, userID uuid.UUID) error {
	return p.publish(ctx, "cart.cleared", clearedPayload{
		UserID: userID.String(),
	})
}

func (p *KafkaPublisher) publish(ctx context.Context, eventType string, payload any) error {
	event, err := kafka.NewEvent(eventType, source, payload)
	if err != nil {
		return fmt.Errorf("creating event %q: %w", eventType, err)
	}
	return p.producer.Publish(ctx, event)
}
