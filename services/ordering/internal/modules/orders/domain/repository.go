package domain

import (
	"context"

	"github.com/google/uuid"

	shared "github.com/foodsea/ordering/internal/shared/domain"
)

// OrderRepository defines the persistence contract for orders.
type OrderRepository interface {
	// CreatePending transactionally creates order + order_items + initial history (status=created).
	CreatePending(ctx context.Context, order *Order) error

	GetByID(ctx context.Context, id uuid.UUID) (*Order, error)

	// GetByIDForUser returns ErrNotFound if the order does not belong to userID (hides existence).
	GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*Order, error)

	// ListByUser returns orders for userID sorted by created_at DESC with total count.
	ListByUser(ctx context.Context, userID uuid.UUID, p shared.Pagination) ([]Order, int, error)

	// TransitionStatus atomically transitions order status and appends a history entry.
	// Returns ErrConflict if the FSM does not allow the transition.
	TransitionStatus(ctx context.Context, id uuid.UUID, to shared.OrderStatus, comment *string) error
}
