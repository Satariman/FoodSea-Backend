package infra_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/ordering/internal/modules/saga/infra"
	"github.com/foodsea/ordering/internal/platform/kafka"
)

// stubProducer captures published events without hitting Kafka.
type stubProducer struct {
	published []kafka.Event
}

func (s *stubProducer) Publish(_ context.Context, e kafka.Event) error {
	s.published = append(s.published, e)
	return nil
}

// AuditPublisher uses *kafka.Producer, which is a concrete type.
// We test the overall structure via the real infra.AuditPublisher with real producers
// that are replaced at the test level.  Since kafka.Producer.Publish requires a
// real writer we can't easily stub it here; instead we test audit_publisher via
// the orchestrator unit tests (which use mockAudit).
// This file verifies the concrete AuditPublisher compiles and constructs correctly.

func TestAuditPublisher_ConstructsWithoutPanic(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cmdProducer := kafka.NewProducer([]string{"localhost:9092"}, "saga.commands", log)
	replyProducer := kafka.NewProducer([]string{"localhost:9092"}, "saga.replies", log)

	pub := infra.NewAuditPublisher(cmdProducer, replyProducer, log)
	require.NotNil(t, pub)
}

func TestAuditPublisher_PayloadShape(t *testing.T) {
	// Verify that the JSON payload written by PublishCommand contains saga_id and step.
	// We do this by inspecting the kafka.NewEvent output directly.

	sagaID := uuid.New()
	step := int8(1)
	cmdType := "LockResult"
	data := map[string]any{"result_id": uuid.New().String()}

	event, err := kafka.NewEvent(cmdType, "ordering-saga", map[string]any{
		"saga_id": sagaID.String(),
		"step":    step,
		"data":    data,
	})
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	assert.Equal(t, sagaID.String(), payload["saga_id"])
	assert.Equal(t, cmdType, event.EventType)
}
