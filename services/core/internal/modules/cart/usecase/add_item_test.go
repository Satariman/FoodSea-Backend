package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/cart/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestAddItem_QtyZero_Error(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewAddItem(repo, pub, silentLogger())

	err := uc.Execute(context.Background(), uuid.New(), uuid.New(), 0)
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
	repo.AssertNotCalled(t, "AddOrIncrementItem")
}

func TestAddItem_Qty100_Error(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewAddItem(repo, pub, silentLogger())

	err := uc.Execute(context.Background(), uuid.New(), uuid.New(), 100)
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
}

func TestAddItem_Success(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewAddItem(repo, pub, silentLogger())

	userID := uuid.New()
	productID := uuid.New()
	repo.On("AddOrIncrementItem", mock.Anything, userID, productID, int16(1)).Return(nil)
	pub.On("ItemAdded", mock.Anything, userID, productID, int16(1)).Return(nil)

	err := uc.Execute(context.Background(), userID, productID, 1)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestAddItem_ProductNotFound_NoPublish(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewAddItem(repo, pub, silentLogger())

	userID := uuid.New()
	productID := uuid.New()
	repo.On("AddOrIncrementItem", mock.Anything, userID, productID, int16(1)).Return(sherrors.ErrNotFound)

	err := uc.Execute(context.Background(), userID, productID, 1)
	assert.ErrorIs(t, err, sherrors.ErrNotFound)
	pub.AssertNotCalled(t, "ItemAdded")
}

func TestAddItem_KafkaError_ReturnsNil(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewAddItem(repo, pub, silentLogger())

	userID := uuid.New()
	productID := uuid.New()
	repo.On("AddOrIncrementItem", mock.Anything, userID, productID, int16(2)).Return(nil)
	pub.On("ItemAdded", mock.Anything, userID, productID, int16(2)).Return(errors.New("kafka down"))

	err := uc.Execute(context.Background(), userID, productID, 2)
	assert.NoError(t, err)
}

func TestAddItem_SumExceeds99_Error(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewAddItem(repo, pub, silentLogger())

	userID := uuid.New()
	productID := uuid.New()
	repo.On("AddOrIncrementItem", mock.Anything, userID, productID, int16(5)).
		Return(sherrors.ErrInvalidInput)

	err := uc.Execute(context.Background(), userID, productID, 5)
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
}
