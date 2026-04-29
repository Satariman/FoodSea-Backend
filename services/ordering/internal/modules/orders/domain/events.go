package domain

import (
	"context"

	"github.com/google/uuid"

	shared "github.com/foodsea/ordering/internal/shared/domain"
)

// OrderEventPublisher publishes domain events to the order.events Kafka topic.
type OrderEventPublisher interface {
	OrderCreated(ctx context.Context, order *Order) error
	OrderConfirmed(ctx context.Context, orderID uuid.UUID) error
	OrderStatusChanged(ctx context.Context, orderID uuid.UUID, old, new shared.OrderStatus) error
	OrderCancelled(ctx context.Context, orderID uuid.UUID, reason string) error
}
