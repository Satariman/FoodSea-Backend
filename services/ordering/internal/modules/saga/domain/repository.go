package domain

import (
	"context"

	"github.com/google/uuid"
)

// SagaRepository persists and retrieves saga lifecycle state.
type SagaRepository interface {
	Create(ctx context.Context, s *SagaState) error
	GetByID(ctx context.Context, id uuid.UUID) (*SagaState, error)
	GetByOrderID(ctx context.Context, orderID uuid.UUID) (*SagaState, error)
	// UpdateState atomically reads the current payload, applies payloadPatch (if non-nil),
	// then writes the new step, status, and merged payload in one transaction.
	UpdateState(ctx context.Context, id uuid.UUID, step int8, status SagaStatus, payloadPatch func(*SagaPayload)) error
	// ListPending returns all sagas with status pending or compensating (for recovery on startup).
	ListPending(ctx context.Context) ([]*SagaState, error)
}
