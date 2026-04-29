package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	"github.com/foodsea/ordering/internal/modules/orders/usecase"
	shared "github.com/foodsea/ordering/internal/shared/domain"
	sherrors "github.com/foodsea/ordering/internal/shared/errors"
)

func TestConfirmOrder_Success(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewConfirmOrder(repo, pub, discardLogger())

	orderID := uuid.New()
	repo.On("TransitionStatus", mock.Anything, orderID, shared.StatusConfirmed, (*string)(nil)).Return(nil)
	pub.On("OrderConfirmed", mock.Anything, orderID).Return(nil)
	pub.On("OrderStatusChanged", mock.Anything, orderID, shared.StatusCreated, shared.StatusConfirmed).Return(nil)

	err := uc.Execute(context.Background(), orderID)
	require.NoError(t, err)
}

func TestConfirmOrder_AlreadyConfirmed_ErrConflict(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewConfirmOrder(repo, pub, discardLogger())

	orderID := uuid.New()
	repo.On("TransitionStatus", mock.Anything, orderID, shared.StatusConfirmed, (*string)(nil)).Return(sherrors.ErrConflict)

	err := uc.Execute(context.Background(), orderID)
	assert.ErrorIs(t, err, sherrors.ErrConflict)
	pub.AssertNotCalled(t, "OrderConfirmed")
}

func TestCancelOrder_Success(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewCancelOrder(repo, pub, discardLogger())

	orderID := uuid.New()
	reason := "user request"

	repo.On("GetByID", mock.Anything, orderID).Return(&domain.Order{ID: orderID, Status: shared.StatusCreated}, nil)
	repo.On("TransitionStatus", mock.Anything, orderID, shared.StatusCancelled, &reason).Return(nil)
	pub.On("OrderCancelled", mock.Anything, orderID, reason).Return(nil)
	pub.On("OrderStatusChanged", mock.Anything, orderID, shared.StatusCreated, shared.StatusCancelled).Return(nil)

	err := uc.Execute(context.Background(), orderID, reason)
	require.NoError(t, err)
}

func TestCancelOrder_EmptyReason_ErrInvalidInput(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewCancelOrder(repo, pub, discardLogger())

	err := uc.Execute(context.Background(), uuid.New(), "")
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
}

func TestCancelOrder_DeliveredOrder_ErrConflict(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewCancelOrder(repo, pub, discardLogger())

	orderID := uuid.New()
	reason := "want to cancel"

	repo.On("GetByID", mock.Anything, orderID).Return(&domain.Order{ID: orderID, Status: shared.StatusDelivered}, nil)
	repo.On("TransitionStatus", mock.Anything, orderID, shared.StatusCancelled, &reason).Return(sherrors.ErrConflict)

	err := uc.Execute(context.Background(), orderID, reason)
	assert.ErrorIs(t, err, sherrors.ErrConflict)
}

func TestGetOrder_OwnOrder_OK(t *testing.T) {
	repo := &mockOrderRepository{}
	uc := usecase.NewGetOrder(repo)

	orderID, userID := uuid.New(), uuid.New()
	expected := &domain.Order{ID: orderID, UserID: userID}
	repo.On("GetByIDForUser", mock.Anything, orderID, userID).Return(expected, nil)

	got, err := uc.Execute(context.Background(), orderID, userID)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestGetOrder_OtherUser_NotFound(t *testing.T) {
	repo := &mockOrderRepository{}
	uc := usecase.NewGetOrder(repo)

	orderID, userID := uuid.New(), uuid.New()
	repo.On("GetByIDForUser", mock.Anything, orderID, userID).Return(nil, sherrors.ErrNotFound)

	_, err := uc.Execute(context.Background(), orderID, userID)
	assert.ErrorIs(t, err, sherrors.ErrNotFound)
}

func TestListOrders_Empty(t *testing.T) {
	repo := &mockOrderRepository{}
	uc := usecase.NewListOrders(repo)

	userID := uuid.New()
	p := shared.NewPagination(1, 20)
	repo.On("ListByUser", mock.Anything, userID, p).Return([]domain.Order{}, 0, nil)

	orders, total, err := uc.Execute(context.Background(), userID, p)
	require.NoError(t, err)
	assert.Empty(t, orders)
	assert.Equal(t, 0, total)
}

func TestUpdateStatus_ValidTransition(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewUpdateStatus(repo, pub, discardLogger())

	orderID := uuid.New()
	repo.On("GetByID", mock.Anything, orderID).Return(&domain.Order{ID: orderID, Status: shared.StatusCreated}, nil)
	repo.On("TransitionStatus", mock.Anything, orderID, shared.StatusConfirmed, (*string)(nil)).Return(nil)
	pub.On("OrderStatusChanged", mock.Anything, orderID, shared.StatusCreated, shared.StatusConfirmed).Return(nil)

	err := uc.Execute(context.Background(), orderID, shared.StatusConfirmed, nil)
	require.NoError(t, err)
}

func TestUpdateStatus_InvalidTransition(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewUpdateStatus(repo, pub, discardLogger())

	orderID := uuid.New()
	repo.On("GetByID", mock.Anything, orderID).Return(&domain.Order{ID: orderID, Status: shared.StatusCreated}, nil)

	// created → delivered is not allowed by FSM
	err := uc.Execute(context.Background(), orderID, shared.StatusDelivered, nil)
	assert.ErrorIs(t, err, sherrors.ErrConflict)
	repo.AssertNotCalled(t, "TransitionStatus")
}

func TestUpdateStatus_NotFound(t *testing.T) {
	repo := &mockOrderRepository{}
	pub := &mockOrderEventPublisher{}
	uc := usecase.NewUpdateStatus(repo, pub, discardLogger())

	orderID := uuid.New()
	repo.On("GetByID", mock.Anything, orderID).Return(nil, sherrors.ErrNotFound)

	err := uc.Execute(context.Background(), orderID, shared.StatusConfirmed, nil)
	assert.ErrorIs(t, err, sherrors.ErrNotFound)
}
