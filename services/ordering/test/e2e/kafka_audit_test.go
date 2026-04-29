//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/ordering/internal/platform/kafka"
)

// kafkaEvent is the minimal envelope to check event_type.
type kafkaEvent struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

func collectEvents(ctx context.Context, brokers []string, topic, groupID string, want int) ([]kafkaEvent, error) {
	r := kafka.NewConsumer(brokers, topic, groupID, testLog)
	defer r.Close()

	events := make([]kafkaEvent, 0, want)
	var mu sync.Mutex
	var collected int32
	var doneOnce sync.Once

	done := make(chan struct{})
	go func() {
		_ = r.Run(ctx, func(_ context.Context, e kafka.Event) error {
			current := atomic.AddInt32(&collected, 1)
			mu.Lock()
			events = append(events, kafkaEvent{EventType: e.EventType, Payload: e.Payload})
			mu.Unlock()
			if int(current) >= want {
				doneOnce.Do(func() {
					close(done)
				})
			}
			return nil
		})
		doneOnce.Do(func() {
			close(done)
		})
	}()

	select {
	case <-done:
		mu.Lock()
		out := append([]kafkaEvent(nil), events...)
		mu.Unlock()
		return out, nil
	case <-ctx.Done():
		mu.Lock()
		out := append([]kafkaEvent(nil), events...)
		mu.Unlock()
		return out, nil
	}
}

func TestKafkaAudit_HappyPath(t *testing.T) {
	cartMock.reset()
	optMock.reset()

	userID := uuid.New()
	optResult := buildOptResult(userID)
	optMock.addResult(optResult)

	// Start consumers BEFORE placing order so they see the messages.
	brokers := []string{testKafkaBroker}
	auditCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	orderEvtCh := make(chan []kafkaEvent, 1)
	cmdEvtCh := make(chan []kafkaEvent, 1)
	replyEvtCh := make(chan []kafkaEvent, 1)

	go func() {
		evts, _ := collectEvents(auditCtx, brokers, "order.events", "audit-order-events-"+userID.String(), 2)
		orderEvtCh <- evts
	}()
	go func() {
		evts, _ := collectEvents(auditCtx, brokers, "saga.commands", "audit-saga-cmd-"+userID.String(), 4)
		cmdEvtCh <- evts
	}()
	go func() {
		evts, _ := collectEvents(auditCtx, brokers, "saga.replies", "audit-saga-reply-"+userID.String(), 4)
		replyEvtCh <- evts
	}()

	// Place order.
	_, code := placeOrder(t, userID, optResult)
	require.Equal(t, http.StatusCreated, code)

	// Wait for order.events: order.created + order.confirmed.
	orderEvts := <-orderEvtCh
	assert.GreaterOrEqual(t, len(orderEvts), 2, "should have at least 2 order events")
	types := eventTypes(orderEvts)
	assert.Contains(t, types, "order.created")
	assert.Contains(t, types, "order.confirmed")

	// Wait for saga.commands: 4 commands (LockResult, CreatePending, ClearCart, Confirm).
	cmdEvts := <-cmdEvtCh
	assert.GreaterOrEqual(t, len(cmdEvts), 4, "should have at least 4 saga command events")

	// Wait for saga.replies: 4 replies.
	replyEvts := <-replyEvtCh
	assert.GreaterOrEqual(t, len(replyEvts), 4, "should have at least 4 saga reply events")
}

func eventTypes(evts []kafkaEvent) []string {
	types := make([]string, len(evts))
	for i, e := range evts {
		types[i] = e.EventType
	}
	return types
}
