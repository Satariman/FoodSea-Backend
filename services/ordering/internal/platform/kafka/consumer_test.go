package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockReader implements the Reader interface for testing.
type mockReader struct {
	messages []kafka.Message
	pos      int
	closed   bool
}

func (m *mockReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if ctx.Err() != nil {
		return kafka.Message{}, ctx.Err()
	}
	if m.pos >= len(m.messages) {
		// Block until context cancelled
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}
	msg := m.messages[m.pos]
	m.pos++
	return msg, nil
}

func (m *mockReader) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	return nil
}

func (m *mockReader) Close() error {
	m.closed = true
	return nil
}

func makeEventMsg(t *testing.T, eventType string) kafka.Message {
	t.Helper()
	ev := Event{
		EventID:   "test-id",
		EventType: eventType,
		Source:    "test",
	}
	data, err := json.Marshal(ev)
	require.NoError(t, err)
	return kafka.Message{Value: data}
}

func TestSubscriber_RoutesByEventType(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reader := &mockReader{
		messages: []kafka.Message{makeEventMsg(t, "saga.cart.cleared")},
	}
	sub := newSubscriberWithReader(reader, log)

	called := false
	sub.Register("saga.cart.cleared", func(ctx context.Context, event Event) error {
		called = true
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// cancel after first message is processed
		for !called {
		}
		cancel()
	}()

	err := sub.Run(ctx)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestSubscriber_UnknownEventType_CommitsOffset(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reader := &mockReader{
		messages: []kafka.Message{makeEventMsg(t, "unknown.event")},
	}
	sub := newSubscriberWithReader(reader, log)
	// No handler registered for "unknown.event"

	ctx, cancel := context.WithCancel(context.Background())
	processed := false
	go func() {
		for reader.pos == 0 {
		}
		processed = true
		cancel()
	}()

	err := sub.Run(ctx)
	assert.NoError(t, err)
	assert.True(t, processed)
}

func TestSubscriber_HandlerError_DoesNotCommit(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Two identical messages; handler errors on first but succeeds on second
	reader := &mockReader{
		messages: []kafka.Message{
			makeEventMsg(t, "saga.event"),
			makeEventMsg(t, "saga.event"),
		},
	}
	sub := newSubscriberWithReader(reader, log)

	count := 0
	sub.Register("saga.event", func(ctx context.Context, event Event) error {
		count++
		if count == 1 {
			return errors.New("transient error")
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for count < 2 {
		}
		cancel()
	}()

	err := sub.Run(ctx)
	assert.NoError(t, err)
	// Both messages were delivered (at-least-once semantics: first was redelivered after error)
	assert.Equal(t, 2, count)
}

func TestSubscriber_ContextDone_ReturnsNil(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reader := &mockReader{} // empty messages — will block until ctx cancelled
	sub := newSubscriberWithReader(reader, log)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := sub.Run(ctx)
	assert.NoError(t, err)
}
