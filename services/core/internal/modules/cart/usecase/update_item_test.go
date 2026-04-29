package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/cart/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestUpdateItem_NotInCart_Error(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewUpdateItem(repo, pub, silentLogger())

	userID := uuid.New()
	productID := uuid.New()
	repo.On("UpdateItemQuantity", mock.Anything, userID, productID, int16(5)).Return(sherrors.ErrNotFound)

	err := uc.Execute(context.Background(), userID, productID, 5)
	assert.ErrorIs(t, err, sherrors.ErrNotFound)
}

func TestUpdateItem_QtyZero_Error(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewUpdateItem(repo, pub, silentLogger())

	err := uc.Execute(context.Background(), uuid.New(), uuid.New(), 0)
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
}

func TestUpdateItem_Success(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewUpdateItem(repo, pub, silentLogger())

	userID := uuid.New()
	productID := uuid.New()
	repo.On("UpdateItemQuantity", mock.Anything, userID, productID, int16(7)).Return(nil)
	pub.On("ItemUpdated", mock.Anything, userID, productID, int16(7)).Return(nil)

	err := uc.Execute(context.Background(), userID, productID, 7)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}
