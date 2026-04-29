package kafka_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kafkapkg "github.com/foodsea/optimization/internal/platform/kafka"
)

func TestNewEvent_FieldsPopulated(t *testing.T) {
	type payload struct {
		UserID string `json:"user_id"`
	}

	before := time.Now()
	event, err := kafkapkg.NewEvent("cart.item_added", "core-service", payload{UserID: "u1"})
	after := time.Now()

	require.NoError(t, err)
	assert.NotEmpty(t, event.EventID, "event_id should be a non-empty UUID")
	assert.Equal(t, "cart.item_added", event.EventType)
	assert.Equal(t, "core-service", event.Source)
	assert.False(t, event.Timestamp.Before(before), "timestamp should be >= before")
	assert.False(t, event.Timestamp.After(after), "timestamp should be <= after")
	assert.NotEmpty(t, event.Payload)
}

func TestNewEvent_PayloadSerialization(t *testing.T) {
	type inner struct {
		Amount int `json:"amount"`
	}
	event, err := kafkapkg.NewEvent("test.event", "svc", inner{Amount: 500})
	require.NoError(t, err)

	var decoded inner
	require.NoError(t, json.Unmarshal(event.Payload, &decoded))
	assert.Equal(t, 500, decoded.Amount)
}

func TestNewEvent_RoundTrip(t *testing.T) {
	original, err := kafkapkg.NewEvent("order.created", "ordering", map[string]int{"total": 1000})
	require.NoError(t, err)

	// Marshal the whole Event envelope and unmarshal it back.
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded kafkapkg.Event
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original.EventID, decoded.EventID)
	assert.Equal(t, original.EventType, decoded.EventType)
	assert.Equal(t, original.Source, decoded.Source)
	assert.JSONEq(t, string(original.Payload), string(decoded.Payload))
}

func TestNewEvent_InvalidPayload(t *testing.T) {
	// channels are not JSON serialisable
	_, err := kafkapkg.NewEvent("bad", "svc", make(chan int))
	assert.Error(t, err)
}
