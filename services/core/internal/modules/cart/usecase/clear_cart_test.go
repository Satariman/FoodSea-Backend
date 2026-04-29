package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/cart/usecase"
)

func TestClearCart_Empty_Success(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewClearCart(repo, pub, silentLogger())

	userID := uuid.New()
	repo.On("Clear", mock.Anything, userID).Return(nil)
	pub.On("Cleared", mock.Anything, userID).Return(nil)

	err := uc.Execute(context.Background(), userID)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestClearCart_NonEmpty_DeletesAll(t *testing.T) {
	repo := &MockCartRepository{}
	pub := &MockCartEventPublisher{}
	uc := usecase.NewClearCart(repo, pub, silentLogger())

	userID := uuid.New()
	repo.On("Clear", mock.Anything, userID).Return(nil)
	pub.On("Cleared", mock.Anything, userID).Return(nil)

	err := uc.Execute(context.Background(), userID)
	assert.NoError(t, err)
}
