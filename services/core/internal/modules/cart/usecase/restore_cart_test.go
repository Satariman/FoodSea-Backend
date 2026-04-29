package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/cart/domain"
	"github.com/foodsea/core/internal/modules/cart/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestRestoreCart_Success(t *testing.T) {
	repo := &MockCartRepository{}
	uc := usecase.NewRestoreCart(repo, silentLogger())

	userID := uuid.New()
	items := []domain.CartItem{
		{ProductID: uuid.New(), Quantity: 2},
		{ProductID: uuid.New(), Quantity: 3},
	}
	repo.On("Restore", mock.Anything, userID, items).Return(nil)

	err := uc.Execute(context.Background(), userID, items)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRestoreCart_EmptyList_OK(t *testing.T) {
	repo := &MockCartRepository{}
	uc := usecase.NewRestoreCart(repo, silentLogger())

	userID := uuid.New()
	repo.On("Restore", mock.Anything, userID, []domain.CartItem{}).Return(nil)

	err := uc.Execute(context.Background(), userID, []domain.CartItem{})
	assert.NoError(t, err)
}

func TestRestoreCart_QtyTooHigh_Error(t *testing.T) {
	repo := &MockCartRepository{}
	uc := usecase.NewRestoreCart(repo, silentLogger())

	items := []domain.CartItem{
		{ProductID: uuid.New(), Quantity: 100},
	}

	err := uc.Execute(context.Background(), uuid.New(), items)
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
	repo.AssertNotCalled(t, "Restore")
}

func TestRestoreCart_NoPublish(t *testing.T) {
	// RestoreCart does not use a publisher — just verifies Restore is called.
	repo := &MockCartRepository{}
	uc := usecase.NewRestoreCart(repo, silentLogger())

	userID := uuid.New()
	items := []domain.CartItem{{ProductID: uuid.New(), Quantity: 1}}
	repo.On("Restore", mock.Anything, userID, items).Return(nil)

	err := uc.Execute(context.Background(), userID, items)
	assert.NoError(t, err)
}
