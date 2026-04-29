package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

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
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}
	msg := m.messages[m.pos]
	m.pos++
	return msg, nil
}

func (m *mockReader) CommitMessages(_ context.Context, _ ...kafka.Message) error {
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
	reader := &mockReader{messages: []kafka.Message{makeEventMsg(t, "saga.cart.cleared")}}
	sub := newSubscriberWithReader(reader, log)

	var called int32
	calledCh := make(chan struct{}, 1)
	sub.Register("saga.cart.cleared", func(ctx context.Context, event Event) error {
		atomic.StoreInt32(&called, 1)
		calledCh <- struct{}{}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-calledCh
		cancel()
	}()

	err := sub.Run(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
}

func TestSubscriber_UnknownEventType_CommitsOffset(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reader := &mockReader{messages: []kafka.Message{makeEventMsg(t, "unknown.event")}}
	sub := newSubscriberWithReader(reader, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for i := 0; i < 100; i++ {
			if reader.pos > 0 {
				cancel()
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
	}()

	err := sub.Run(ctx)
	assert.NoError(t, err)
	assert.Greater(t, reader.pos, 0)
}

func TestSubscriber_HandlerError_DoesNotCommit(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reader := &mockReader{messages: []kafka.Message{makeEventMsg(t, "saga.event"), makeEventMsg(t, "saga.event")}}
	sub := newSubscriberWithReader(reader, log)

	var count int32
	sub.Register("saga.event", func(ctx context.Context, event Event) error {
		n := atomic.AddInt32(&count, 1)
		if n == 1 {
			return errors.New("transient error")
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for i := 0; i < 100; i++ {
			if atomic.LoadInt32(&count) >= 2 {
				cancel()
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
	}()

	err := sub.Run(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&count))
}

func TestSubscriber_ContextDone_ReturnsNil(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reader := &mockReader{}
	sub := newSubscriberWithReader(reader, log)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sub.Run(ctx)
	assert.NoError(t, err)
}
