package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	"github.com/foodsea/ordering/internal/modules/orders/usecase"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCreateOrderPending_Success(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewCreateOrderPending(repo, pub, discardLogger())

	input := usecase.CreatePendingInput{
		UserID:          uuid.New(),
		TotalKopecks:    5000,
		DeliveryKopecks: 200,
		Items: []usecase.OrderItemSnapshot{
			{ProductID: uuid.New(), ProductName: "Apple", StoreID: uuid.New(), StoreName: "Shop", Quantity: 2, PriceKopecks: 2500},
		},
	}

	repo.On("CreatePending", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
	pub.On("OrderCreated", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)

	order, err := uc.Execute(context.Background(), input)
	require.NoError(t, err)
	assert.NotNil(t, order)
	repo.AssertCalled(t, "CreatePending", mock.Anything, mock.Anything)
	pub.AssertCalled(t, "OrderCreated", mock.Anything, mock.Anything)
}

func TestCreateOrderPending_RepoError_NoPublish(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewCreateOrderPending(repo, pub, discardLogger())

	repo.On("CreatePending", mock.Anything, mock.Anything).Return(errors.New("db error"))

	_, err := uc.Execute(context.Background(), usecase.CreatePendingInput{UserID: uuid.New()})
	assert.Error(t, err)
	pub.AssertNotCalled(t, "OrderCreated")
}

func TestCreateOrderPending_KafkaError_OrderStillSaved(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewCreateOrderPending(repo, pub, discardLogger())

	repo.On("CreatePending", mock.Anything, mock.Anything).Return(nil)
	pub.On("OrderCreated", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(errors.New("kafka error"))

	// Kafka error is non-fatal: use case returns nil
	order, err := uc.Execute(context.Background(), usecase.CreatePendingInput{UserID: uuid.New()})
	assert.NoError(t, err)
	assert.NotNil(t, order)
}

func TestCreateOrderPending_WithOptimizationResultID(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewCreateOrderPending(repo, pub, discardLogger())

	optID := uuid.New()
	input := usecase.CreatePendingInput{
		UserID:               uuid.New(),
		OptimizationResultID: &optID,
		TotalKopecks:         1000,
	}

	repo.On("CreatePending", mock.Anything, mock.MatchedBy(func(o *domain.Order) bool {
		return o.OptimizationResultID != nil && *o.OptimizationResultID == optID
	})).Return(nil)
	pub.On("OrderCreated", mock.Anything, mock.Anything).Return(nil)

	_, err := uc.Execute(context.Background(), input)
	require.NoError(t, err)
}
