package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	shared "github.com/foodsea/ordering/internal/shared/domain"
)

// ListOrders returns paginated orders for a user sorted by created_at DESC.
type ListOrders struct {
	repo domain.OrderRepository
}

func NewListOrders(repo domain.OrderRepository) *ListOrders {
	return &ListOrders{repo: repo}
}

func (uc *ListOrders) Execute(ctx context.Context, userID uuid.UUID, p shared.Pagination) ([]domain.Order, int, error) {
	return uc.repo.ListByUser(ctx, userID, p)
}
