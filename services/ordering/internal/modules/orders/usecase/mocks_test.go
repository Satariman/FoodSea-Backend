package usecase_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	shared "github.com/foodsea/ordering/internal/shared/domain"
)

// mockOrderRepository is a testify mock for domain.OrderRepository.
type mockOrderRepository struct{ mock.Mock }

func (m *mockOrderRepository) CreatePending(ctx context.Context, order *domain.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}
func (m *mockOrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Order), args.Error(1)
}
func (m *mockOrderRepository) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*domain.Order, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Order), args.Error(1)
}
func (m *mockOrderRepository) ListByUser(ctx context.Context, userID uuid.UUID, p shared.Pagination) ([]domain.Order, int, error) {
	args := m.Called(ctx, userID, p)
	return args.Get(0).([]domain.Order), args.Int(1), args.Error(2)
}
func (m *mockOrderRepository) TransitionStatus(ctx context.Context, id uuid.UUID, to shared.OrderStatus, comment *string) error {
	args := m.Called(ctx, id, to, comment)
	return args.Error(0)
}

// mockOrderEventPublisher is a testify mock for domain.OrderEventPublisher.
type mockOrderEventPublisher struct{ mock.Mock }

func (m *mockOrderEventPublisher) OrderCreated(ctx context.Context, order *domain.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}
func (m *mockOrderEventPublisher) OrderConfirmed(ctx context.Context, orderID uuid.UUID) error {
	args := m.Called(ctx, orderID)
	return args.Error(0)
}
func (m *mockOrderEventPublisher) OrderStatusChanged(ctx context.Context, orderID uuid.UUID, old, new shared.OrderStatus) error {
	args := m.Called(ctx, orderID, old, new)
	return args.Error(0)
}
func (m *mockOrderEventPublisher) OrderCancelled(ctx context.Context, orderID uuid.UUID, reason string) error {
	args := m.Called(ctx, orderID, reason)
	return args.Error(0)
}
