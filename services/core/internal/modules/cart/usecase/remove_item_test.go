package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/cart/usecase"
)

func TestRemoveItem_NotInCart_Idempotent(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewRemoveItem(repo, pub, silentLogger())

	userID := uuid.New()
	productID := uuid.New()
	repo.On("RemoveItem", mock.Anything, userID, productID).Return(nil)
	pub.On("ItemRemoved", mock.Anything, userID, productID).Return(nil)

	err := uc.Execute(context.Background(), userID, productID)
	assert.NoError(t, err)
}

func TestRemoveItem_Success_Publishes(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewRemoveItem(repo, pub, silentLogger())

	userID := uuid.New()
	productID := uuid.New()
	repo.On("RemoveItem", mock.Anything, userID, productID).Return(nil)
	pub.On("ItemRemoved", mock.Anything, userID, productID).Return(nil)

	err := uc.Execute(context.Background(), userID, productID)
	assert.NoError(t, err)
	pub.AssertExpectations(t)
}
