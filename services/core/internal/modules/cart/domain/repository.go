package domain

import (
	"context"

	"github.com/google/uuid"
)

type CartRepository interface {
	GetByUser(ctx context.Context, userID uuid.UUID) (*Cart, error)
	AddOrIncrementItem(ctx context.Context, userID, productID uuid.UUID, qty int16) error
	UpdateItemQuantity(ctx context.Context, userID, productID uuid.UUID, qty int16) error
	RemoveItem(ctx context.Context, userID, productID uuid.UUID) error
	Clear(ctx context.Context, userID uuid.UUID) error
	Restore(ctx context.Context, userID uuid.UUID, items []CartItem) error
}
