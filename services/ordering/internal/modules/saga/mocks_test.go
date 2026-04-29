package saga_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/ordering/internal/modules/saga/domain"
)

// ─── SagaRepository mock ──────────────────────────────────────────────────────

type mockSagaRepo struct{ mock.Mock }

func (m *mockSagaRepo) Create(ctx context.Context, s *domain.SagaState) error {
	args := m.Called(ctx, s)
	s.ID = uuid.New() // simulate DB-generated ID
	return args.Error(0)
}

func (m *mockSagaRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.SagaState, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SagaState), args.Error(1)
}

func (m *mockSagaRepo) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.SagaState, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SagaState), args.Error(1)
}

func (m *mockSagaRepo) UpdateState(ctx context.Context, id uuid.UUID, step int8, status domain.SagaStatus, patch func(*domain.SagaPayload)) error {
	args := m.Called(ctx, id, step, status, patch)
	return args.Error(0)
}

func (m *mockSagaRepo) ListPending(ctx context.Context) ([]*domain.SagaState, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SagaState), args.Error(1)
}

// ─── CartParticipant mock ─────────────────────────────────────────────────────

type mockCart struct{ mock.Mock }

func (m *mockCart) ClearCart(ctx context.Context, userID uuid.UUID) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *mockCart) RestoreCart(ctx context.Context, userID uuid.UUID, items []domain.OrderItemSnapshot) error {
	return m.Called(ctx, userID, items).Error(0)
}

// ─── OptimizationParticipant mock ─────────────────────────────────────────────

type mockOpt struct{ mock.Mock }

func (m *mockOpt) LockResult(ctx context.Context, resultID uuid.UUID) error {
	return m.Called(ctx, resultID).Error(0)
}

func (m *mockOpt) UnlockResult(ctx context.Context, resultID uuid.UUID) error {
	return m.Called(ctx, resultID).Error(0)
}

func (m *mockOpt) GetResult(ctx context.Context, resultID uuid.UUID) (*domain.OptimizationResult, error) {
	args := m.Called(ctx, resultID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.OptimizationResult), args.Error(1)
}

// ─── OrdersParticipant mock ───────────────────────────────────────────────────

type mockOrders struct{ mock.Mock }

func (m *mockOrders) CreatePending(ctx context.Context, input domain.CreatePendingInput) (*domain.Order, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Order), args.Error(1)
}

func (m *mockOrders) Confirm(ctx context.Context, orderID uuid.UUID) error {
	return m.Called(ctx, orderID).Error(0)
}

func (m *mockOrders) Cancel(ctx context.Context, orderID uuid.UUID, reason string) error {
	return m.Called(ctx, orderID, reason).Error(0)
}

// ─── SagaAuditPublisher mock ─────────────────────────────────────────────────

type mockAudit struct{ mock.Mock }

func (m *mockAudit) PublishCommand(ctx context.Context, sagaID uuid.UUID, step int8, cmdType string, payload any) error {
	return m.Called(ctx, sagaID, step, cmdType, payload).Error(0)
}

func (m *mockAudit) PublishReply(ctx context.Context, sagaID uuid.UUID, step int8, status string, payload any) error {
	return m.Called(ctx, sagaID, step, status, payload).Error(0)
}
