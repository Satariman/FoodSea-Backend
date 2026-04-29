package usecase_test

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/cart/domain"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// MockCartRepository mocks domain.CartRepository.
type MockCartRepository struct{ mock.Mock }

func (m *MockCartRepository) GetByUser(ctx context.Context, userID uuid.UUID) (*domain.Cart, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(*domain.Cart)
	return v, args.Error(1)
}

func (m *MockCartRepository) AddOrIncrementItem(ctx context.Context, userID, productID uuid.UUID, qty int16) error {
	args := m.Called(ctx, userID, productID, qty)
	return args.Error(0)
}

func (m *MockCartRepository) UpdateItemQuantity(ctx context.Context, userID, productID uuid.UUID, qty int16) error {
	args := m.Called(ctx, userID, productID, qty)
	return args.Error(0)
}

func (m *MockCartRepository) RemoveItem(ctx context.Context, userID, productID uuid.UUID) error {
	args := m.Called(ctx, userID, productID)
	return args.Error(0)
}

func (m *MockCartRepository) Clear(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockCartRepository) Restore(ctx context.Context, userID uuid.UUID, items []domain.CartItem) error {
	args := m.Called(ctx, userID, items)
	return args.Error(0)
}

// MockCartEventPublisher mocks domain.CartEventPublisher.
type MockCartEventPublisher struct{ mock.Mock }

func (m *MockCartEventPublisher) ItemAdded(ctx context.Context, userID, productID uuid.UUID, quantity int16) error {
	args := m.Called(ctx, userID, productID, quantity)
	return args.Error(0)
}

func (m *MockCartEventPublisher) ItemUpdated(ctx context.Context, userID, productID uuid.UUID, quantity int16) error {
	args := m.Called(ctx, userID, productID, quantity)
	return args.Error(0)
}

func (m *MockCartEventPublisher) ItemRemoved(ctx context.Context, userID, productID uuid.UUID) error {
	args := m.Called(ctx, userID, productID)
	return args.Error(0)
}

func (m *MockCartEventPublisher) Cleared(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}
