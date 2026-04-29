package events_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/cart/events"
	"github.com/foodsea/core/internal/platform/kafka"
)

// mockProducer captures published events.
type mockProducer struct {
	mock.Mock
	published []kafka.Event
}

func (m *mockProducer) Publish(ctx context.Context, event kafka.Event) error {
	m.published = append(m.published, event)
	args := m.Called(ctx, event)
	return args.Error(0)
}

func TestItemAdded_PublishesCorrectEvent(t *testing.T) {
	mp := &mockProducer{}
	mp.On("Publish", mock.Anything, mock.MatchedBy(func(e kafka.Event) bool {
		return e.EventType == "cart.item_added" && e.Source == "core-service"
	})).Return(nil)

	pub := events.NewKafkaPublisherWithInterface(mp)
	userID := uuid.New()
	productID := uuid.New()

	err := pub.ItemAdded(context.Background(), userID, productID, 2)
	require.NoError(t, err)

	require.Len(t, mp.published, 1)
	ev := mp.published[0]
	assert.Equal(t, "cart.item_added", ev.EventType)
	assert.Equal(t, "core-service", ev.Source)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(ev.Payload, &payload))
	assert.Equal(t, userID.String(), payload["user_id"])
	assert.Equal(t, productID.String(), payload["product_id"])
	assert.Equal(t, float64(2), payload["quantity"])
}

func TestCleared_PublishesCorrectEvent(t *testing.T) {
	mp := &mockProducer{}
	mp.On("Publish", mock.Anything, mock.MatchedBy(func(e kafka.Event) bool {
		return e.EventType == "cart.cleared"
	})).Return(nil)

	pub := events.NewKafkaPublisherWithInterface(mp)
	userID := uuid.New()

	err := pub.Cleared(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, "cart.cleared", mp.published[0].EventType)
}

func TestItemAdded_ProducerError_Returned(t *testing.T) {
	mp := &mockProducer{}
	mp.On("Publish", mock.Anything, mock.Anything).Return(assert.AnError)

	pub := events.NewKafkaPublisherWithInterface(mp)
	err := pub.ItemAdded(context.Background(), uuid.New(), uuid.New(), 1)
	assert.Error(t, err)
}
