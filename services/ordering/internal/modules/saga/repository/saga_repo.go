package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/ent"
	"github.com/foodsea/ordering/ent/sagastate"
	"github.com/foodsea/ordering/internal/modules/saga/domain"
)

// SagaRepo is the Ent-backed implementation of domain.SagaRepository.
type SagaRepo struct {
	ent *ent.Client
}

// NewSagaRepo creates a SagaRepo.
func NewSagaRepo(client *ent.Client) *SagaRepo {
	return &SagaRepo{ent: client}
}

// Create inserts a new SagaState and fills s.ID, s.CreatedAt, s.UpdatedAt from the result.
func (r *SagaRepo) Create(ctx context.Context, s *domain.SagaState) error {
	entity, err := r.ent.SagaState.Create().
		SetOrderID(s.OrderID).
		SetUserID(s.UserID).
		SetCurrentStep(s.CurrentStep).
		SetStatus(string(s.Status)).
		SetPayload(marshalPayload(s.Payload)).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("creating saga state: %w", err)
	}
	s.ID = entity.ID
	s.CreatedAt = entity.CreatedAt
	s.UpdatedAt = entity.UpdatedAt
	return nil
}

// GetByID returns the saga by primary key.
func (r *SagaRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.SagaState, error) {
	entity, err := r.ent.SagaState.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("getting saga state: %w", err)
	}
	return toSagaState(entity), nil
}

// GetByOrderID returns the saga that corresponds to the given order_id.
func (r *SagaRepo) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.SagaState, error) {
	entity, err := r.ent.SagaState.Query().
		Where(sagastate.OrderID(orderID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrNotFound
		}
		if ent.IsNotSingular(err) {
			return nil, fmt.Errorf("multiple sagas for order_id %s: %w", orderID, err)
		}
		return nil, fmt.Errorf("getting saga state by order_id: %w", err)
	}
	return toSagaState(entity), nil
}

// UpdateState atomically reads the current row, applies payloadPatch, then writes
// the new step/status/payload in one transaction.
func (r *SagaRepo) UpdateState(ctx context.Context, id uuid.UUID, step int8, status domain.SagaStatus, payloadPatch func(*domain.SagaPayload)) error {
	tx, err := r.ent.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	entity, err := tx.SagaState.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("locking saga state: %w", err)
	}

	payload := unmarshalPayload(entity.Payload)
	if payloadPatch != nil {
		payloadPatch(&payload)
	}

	upd := tx.SagaState.UpdateOneID(id).
		SetCurrentStep(step).
		SetStatus(string(status)).
		SetPayload(marshalPayload(payload))

	// Sync the top-level order_id column when payload.OrderID is set.
	if payload.OrderID != nil {
		upd = upd.SetOrderID(*payload.OrderID)
	}

	if _, err = upd.Save(ctx); err != nil {
		return fmt.Errorf("updating saga state: %w", err)
	}

	return tx.Commit()
}

// ListPending returns all sagas with status pending or compensating.
func (r *SagaRepo) ListPending(ctx context.Context) ([]*domain.SagaState, error) {
	entities, err := r.ent.SagaState.Query().
		Where(sagastate.StatusIn(
			string(domain.SagaStatusPending),
			string(domain.SagaStatusCompensating),
		)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing pending sagas: %w", err)
	}
	result := make([]*domain.SagaState, len(entities))
	for i, e := range entities {
		result[i] = toSagaState(e)
	}
	return result, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func toSagaState(e *ent.SagaState) *domain.SagaState {
	return &domain.SagaState{
		ID:          e.ID,
		OrderID:     e.OrderID,
		UserID:      e.UserID,
		CurrentStep: e.CurrentStep,
		Status:      domain.SagaStatus(e.Status),
		Payload:     unmarshalPayload(e.Payload),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func marshalPayload(p domain.SagaPayload) map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func unmarshalPayload(m map[string]any) domain.SagaPayload {
	b, _ := json.Marshal(m)
	var p domain.SagaPayload
	_ = json.Unmarshal(b, &p)
	return p
}
