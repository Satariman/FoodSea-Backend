package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/cart/domain"
	"github.com/foodsea/core/internal/modules/cart/usecase"
)

func TestGetCart_Empty(t *testing.T) {
	repo := &MockCartRepository{}
	uc := usecase.NewGetCart(repo)

	userID := uuid.New()
	expected := &domain.Cart{ID: uuid.New(), UserID: userID, Items: []domain.CartItem{}}
	repo.On("GetByUser", mock.Anything, userID).Return(expected, nil)

	got, err := uc.Execute(context.Background(), userID)
	assert.NoError(t, err)
	assert.Empty(t, got.Items)
}

func TestGetCart_WithItems(t *testing.T) {
	repo := &MockCartRepository{}
	uc := usecase.NewGetCart(repo)

	userID := uuid.New()
	expected := &domain.Cart{
		ID:     uuid.New(),
		UserID: userID,
		Items: []domain.CartItem{
			{ID: uuid.New(), ProductID: uuid.New(), ProductName: "Apple", Quantity: 3, AddedAt: time.Now()},
		},
	}
	repo.On("GetByUser", mock.Anything, userID).Return(expected, nil)

	got, err := uc.Execute(context.Background(), userID)
	assert.NoError(t, err)
	assert.Len(t, got.Items, 1)
	assert.Equal(t, "Apple", got.Items[0].ProductName)
}
