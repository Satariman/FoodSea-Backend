package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	"github.com/foodsea/ordering/internal/platform/kafka"
	shared "github.com/foodsea/ordering/internal/shared/domain"
)

const source = "ordering-service"

// Event type constants for order.events topic.
const (
	EventOrderCreated       = "order.created"
	EventOrderConfirmed     = "order.confirmed"
	EventOrderStatusChanged = "order.status_changed"
	EventOrderCancelled     = "order.cancelled"
)

// EventSink is the minimal interface for capturing published events (used in tests).
type EventSink interface {
	Capture(event kafka.Event)
}

// KafkaPublisher implements domain.OrderEventPublisher using the Kafka producer.
type KafkaPublisher struct {
	producer *kafka.Producer
	sink     EventSink // non-nil only in tests
	log      *slog.Logger
}

// NewKafkaPublisher creates a KafkaPublisher for the given producer.
func NewKafkaPublisher(producer *kafka.Producer, log *slog.Logger) *KafkaPublisher {
	return &KafkaPublisher{producer: producer, log: log}
}

// NewTestablePublisher creates a KafkaPublisher that captures events via EventSink (for tests).
func NewTestablePublisher(sink EventSink, log *slog.Logger) *KafkaPublisher {
	return &KafkaPublisher{sink: sink, log: log}
}

type orderCreatedPayload struct {
	OrderID    string              `json:"order_id"`
	UserID     string              `json:"user_id"`
	Total      int64               `json:"total_kopecks"`
	Delivery   int64               `json:"delivery_kopecks"`
	Items      []orderItemSnapshot `json:"items"`
	CreatedAt  time.Time           `json:"created_at"`
}

type orderItemSnapshot struct {
	ProductID    string `json:"product_id"`
	ProductName  string `json:"product_name"`
	StoreID      string `json:"store_id"`
	StoreName    string `json:"store_name"`
	Quantity     int16  `json:"quantity"`
	PriceKopecks int64  `json:"price_kopecks"`
}

type orderConfirmedPayload struct {
	OrderID string `json:"order_id"`
}

type orderStatusChangedPayload struct {
	OrderID   string `json:"order_id"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	ChangedAt string `json:"changed_at"`
	Comment   string `json:"comment,omitempty"`
}

type orderCancelledPayload struct {
	OrderID     string `json:"order_id"`
	Reason      string `json:"reason"`
	CancelledAt string `json:"cancelled_at"`
}

func (p *KafkaPublisher) OrderCreated(ctx context.Context, order *domain.Order) error {
	items := make([]orderItemSnapshot, len(order.Items))
	for i, it := range order.Items {
		items[i] = orderItemSnapshot{
			ProductID:    it.ProductID.String(),
			ProductName:  it.ProductName,
			StoreID:      it.StoreID.String(),
			StoreName:    it.StoreName,
			Quantity:     it.Quantity,
			PriceKopecks: it.PriceKopecks,
		}
	}
	payload := orderCreatedPayload{
		OrderID:   order.ID.String(),
		UserID:    order.UserID.String(),
		Total:     order.TotalKopecks,
		Delivery:  order.DeliveryKopecks,
		Items:     items,
		CreatedAt: order.CreatedAt,
	}
	return p.publish(ctx, EventOrderCreated, payload)
}

func (p *KafkaPublisher) OrderConfirmed(ctx context.Context, orderID uuid.UUID) error {
	return p.publish(ctx, EventOrderConfirmed, orderConfirmedPayload{OrderID: orderID.String()})
}

func (p *KafkaPublisher) OrderStatusChanged(ctx context.Context, orderID uuid.UUID, old, new shared.OrderStatus) error {
	return p.publish(ctx, EventOrderStatusChanged, orderStatusChangedPayload{
		OrderID:   orderID.String(),
		OldStatus: old.String(),
		NewStatus: new.String(),
		ChangedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (p *KafkaPublisher) OrderCancelled(ctx context.Context, orderID uuid.UUID, reason string) error {
	return p.publish(ctx, EventOrderCancelled, orderCancelledPayload{
		OrderID:     orderID.String(),
		Reason:      reason,
		CancelledAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (p *KafkaPublisher) publish(ctx context.Context, eventType string, payload any) error {
	event, err := kafka.NewEvent(eventType, source, payload)
	if err != nil {
		return err
	}
	if p.sink != nil {
		p.sink.Capture(event)
		return nil
	}
	if err = p.producer.Publish(ctx, event); err != nil {
		p.log.WarnContext(ctx, "failed to publish order event",
			"event_type", eventType,
			"error", err,
		)
		return err
	}
	return nil
}
