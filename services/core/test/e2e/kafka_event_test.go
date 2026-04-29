//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformkafka "github.com/foodsea/core/internal/platform/kafka"
)

func TestKafkaCartEvent(t *testing.T) {
	ctx := context.Background()

	// Register a user.
	access := registerUser(t, "kafka-event@foodsea.test", "SuperSecret1!")

	// Create a unique consumer group so each test run reads from the start.
	groupID := "e2e-" + t.Name() + "-" + time.Now().Format("150405")

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:   []string{testKafkaBroker},
		Topic:     "cart.events",
		GroupID:   groupID,
		MinBytes:  1,
		MaxBytes:  1 << 20,
		MaxWait:   500 * time.Millisecond,
		StartOffset: kafkago.FirstOffset,
	})
	defer reader.Close()

	// POST /cart/items triggers a cart.item_added Kafka event.
	addResp, err := postJSONAuth(testBaseURL+"/api/v1/cart/items", access, map[string]any{
		"product_id": seededProductID,
		"quantity":   1,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, addResp.StatusCode)
	addResp.Body.Close()

	// Consume with a 5-second deadline.
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	msg, err := reader.ReadMessage(readCtx)
	require.NoError(t, err, "expected a Kafka message within 5 seconds")

	var event platformkafka.Event
	require.NoError(t, json.Unmarshal(msg.Value, &event))

	assert.Equal(t, "cart.item_added", event.EventType)
	assert.NotEmpty(t, event.EventID)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	assert.NotEmpty(t, payload["user_id"])
	assert.Equal(t, seededProductID, payload["product_id"])
}
