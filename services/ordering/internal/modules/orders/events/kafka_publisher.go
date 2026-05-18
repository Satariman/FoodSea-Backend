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

type orderEventProducer interface {
	Publish(ctx context.Context, event kafka.Event) error
	PublishWithKey(ctx context.Context, key string, event kafka.Event) error
}

// KafkaPublisher implements domain.OrderEventPublisher using the Kafka producer.
type KafkaPublisher struct {
	producer orderEventProducer
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
	Status     shared.OrderStatus  `json:"status"`
	OccurredAt time.Time           `json:"occurred_at"`
	Total      int64               `json:"total_kopecks"`
	Delivery   int64               `json:"delivery_kopecks"`
	Items      []orderItemSnapshot `json:"items"`
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
	OrderID    string             `json:"order_id"`
	UserID     string             `json:"user_id"`
	Status     shared.OrderStatus `json:"status"`
	OccurredAt time.Time          `json:"occurred_at"`
}

type orderStatusChangedPayload struct {
	OrderID    string             `json:"order_id"`
	UserID     string             `json:"user_id"`
	Status     shared.OrderStatus `json:"status"`
	OccurredAt time.Time          `json:"occurred_at"`
	OldStatus  string             `json:"old_status"`
	NewStatus  string             `json:"new_status"`
}

type orderCancelledPayload struct {
	OrderID    string             `json:"order_id"`
	UserID     string             `json:"user_id"`
	Status     shared.OrderStatus `json:"status"`
	OccurredAt time.Time          `json:"occurred_at"`
	Reason     string             `json:"reason"`
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
		OrderID:    order.ID.String(),
		UserID:     order.UserID.String(),
		Status:     shared.StatusCreated,
		OccurredAt: order.CreatedAt.UTC(),
		Total:      order.TotalKopecks,
		Delivery:   order.DeliveryKopecks,
		Items:      items,
	}
	return p.publish(ctx, EventOrderCreated, order.ID, payload)
}

func (p *KafkaPublisher) OrderConfirmed(ctx context.Context, orderID, userID uuid.UUID) error {
	return p.publish(ctx, EventOrderConfirmed, orderID, orderConfirmedPayload{
		OrderID:    orderID.String(),
		UserID:     userID.String(),
		Status:     shared.StatusConfirmed,
		OccurredAt: time.Now().UTC(),
	})
}

func (p *KafkaPublisher) OrderStatusChanged(ctx context.Context, orderID, userID uuid.UUID, old, new shared.OrderStatus) error {
	return p.publish(ctx, EventOrderStatusChanged, orderID, orderStatusChangedPayload{
		OrderID:    orderID.String(),
		UserID:     userID.String(),
		Status:     new,
		OccurredAt: time.Now().UTC(),
		OldStatus:  old.String(),
		NewStatus:  new.String(),
	})
}

func (p *KafkaPublisher) OrderCancelled(ctx context.Context, orderID, userID uuid.UUID, reason string) error {
	return p.publish(ctx, EventOrderCancelled, orderID, orderCancelledPayload{
		OrderID:    orderID.String(),
		UserID:     userID.String(),
		Status:     shared.StatusCancelled,
		OccurredAt: time.Now().UTC(),
		Reason:     reason,
	})
}

func (p *KafkaPublisher) publish(ctx context.Context, eventType string, orderID uuid.UUID, payload any) error {
	event, err := kafka.NewEvent(eventType, source, payload)
	if err != nil {
		return err
	}
	if p.sink != nil {
		p.sink.Capture(event)
		return nil
	}
	if err = p.producer.PublishWithKey(ctx, orderID.String(), event); err != nil {
		p.log.WarnContext(ctx, "failed to publish order event",
			"event_type", eventType,
			"order_id", orderID,
			"error", err,
		)
		return err
	}
	return nil
}
