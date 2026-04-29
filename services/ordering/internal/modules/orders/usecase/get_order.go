package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
)

// GetOrder retrieves an order that belongs to the authenticated user.
type GetOrder struct {
	repo domain.OrderRepository
}

func NewGetOrder(repo domain.OrderRepository) *GetOrder {
	return &GetOrder{repo: repo}
}

// Execute returns ErrNotFound if the order does not exist or belongs to a different user.
func (uc *GetOrder) Execute(ctx context.Context, orderID, userID uuid.UUID) (*domain.Order, error) {
	return uc.repo.GetByIDForUser(ctx, orderID, userID)
}
