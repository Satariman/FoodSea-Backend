package events_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	"github.com/foodsea/ordering/internal/modules/orders/events"
	"github.com/foodsea/ordering/internal/platform/kafka"
	shared "github.com/foodsea/ordering/internal/shared/domain"
)

// captureProducer is a fake kafka.Producer that records published events.
type captureProducer struct {
	events []kafka.Event
}

func (p *captureProducer) Publish(_ context.Context, event kafka.Event) error {
	p.events = append(p.events, event)
	return nil
}

// fakePublisher wraps captureProducer as *kafka.Producer-compatible by using
// the events.NewKafkaPublisher with a real Producer that uses a mock writer.
// For simplicity, we test through the publisher's actual Publish logic using
// a testable producer wrapper.
//
// Since kafka.Producer is a concrete struct, we test by creating a real producer
// and checking output via the captureProducer. We need to adjust this slightly:
// we'll create a thin test publisher directly.

func makeLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// testPublisher wraps events.KafkaPublisher with an internal capture mechanism.
// We test via the published Kafka events directly by examining the payload field.
func TestOrderCreated_PublishesEventWithItems(t *testing.T) {
	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()
	productID := uuid.New()
	storeID := uuid.New()

	order := &domain.Order{
		ID:              orderID,
		UserID:          userID,
		TotalKopecks:    5000,
		DeliveryKopecks: 200,
		CreatedAt:       time.Now(),
		Items: []domain.OrderItem{
			{
				ID:           uuid.New(),
				ProductID:    productID,
				ProductName:  "Apple",
				StoreID:      storeID,
				StoreName:    "FoodStore",
				Quantity:     3,
				PriceKopecks: 150,
			},
		},
	}

	// Build publisher with a real Producer backed by a mock writer.
	// Use a real producer but test the event emission by hooking into the
	// producer via testPublisher pattern.
	captured := &capturedPublisherHelper{}
	pub := events.NewTestablePublisher(captured, makeLog())

	err := pub.OrderCreated(ctx, order)
	require.NoError(t, err)

	require.Len(t, captured.events, 1)
	ev := captured.events[0]
	assert.Equal(t, "order.created", ev.EventType)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(ev.Payload, &payload))
	assert.Equal(t, orderID.String(), payload["order_id"])
	assert.Equal(t, float64(5000), payload["total_kopecks"])

	items, ok := payload["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
}

func TestOrderStatusChanged_PayloadContainsBothStatuses(t *testing.T) {
	captured := &capturedPublisherHelper{}
	pub := events.NewTestablePublisher(captured, makeLog())

	orderID := uuid.New()
	err := pub.OrderStatusChanged(context.Background(), orderID, shared.StatusCreated, shared.StatusConfirmed)
	require.NoError(t, err)

	require.Len(t, captured.events, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(captured.events[0].Payload, &payload))
	assert.Equal(t, "created", payload["old_status"])
	assert.Equal(t, "confirmed", payload["new_status"])
}

func TestOrderCancelled_PayloadHasReason(t *testing.T) {
	captured := &capturedPublisherHelper{}
	pub := events.NewTestablePublisher(captured, makeLog())

	orderID := uuid.New()
	err := pub.OrderCancelled(context.Background(), orderID, "user cancelled")
	require.NoError(t, err)

	require.Len(t, captured.events, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(captured.events[0].Payload, &payload))
	assert.Equal(t, "user cancelled", payload["reason"])
}

// capturedPublisherHelper captures events published via the EventSink interface.
type capturedPublisherHelper struct {
	events []kafka.Event
}

func (c *capturedPublisherHelper) Capture(event kafka.Event) {
	c.events = append(c.events, event)
}
