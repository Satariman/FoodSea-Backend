package kafka

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	kgo "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMessageWriter struct {
	messages []kgo.Message
}

func (m *mockMessageWriter) WriteMessages(_ context.Context, msgs ...kgo.Message) error {
	m.messages = append(m.messages, msgs...)
	return nil
}

func (m *mockMessageWriter) Close() error {
	return nil
}

func TestProducerPublish_UsesEventIDAsDefaultKey(t *testing.T) {
	w := &mockMessageWriter{}
	p := &Producer{writer: w, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	event := Event{
		EventID:   "evt-1",
		EventType: "order.created",
		Timestamp: time.Now().UTC(),
		Source:    "ordering-service",
		Payload:   json.RawMessage(`{"order_id":"o1"}`),
	}

	err := p.Publish(context.Background(), event)
	require.NoError(t, err)
	require.Len(t, w.messages, 1)
	assert.Equal(t, []byte("evt-1"), w.messages[0].Key)
}

func TestProducerPublishWithKey_UsesProvidedKey(t *testing.T) {
	w := &mockMessageWriter{}
	p := &Producer{writer: w, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	event := Event{
		EventID:   "evt-2",
		EventType: "order.confirmed",
		Timestamp: time.Now().UTC(),
		Source:    "ordering-service",
		Payload:   json.RawMessage(`{"order_id":"o1"}`),
	}

	err := p.PublishWithKey(context.Background(), "order-1", event)
	require.NoError(t, err)
	require.Len(t, w.messages, 1)
	assert.Equal(t, []byte("order-1"), w.messages[0].Key)
}
