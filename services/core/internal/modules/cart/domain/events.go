package domain

import (
	"context"

	"github.com/google/uuid"
)

type CartEventPublisher interface {
	ItemAdded(ctx context.Context, userID, productID uuid.UUID, quantity int16) error
	ItemUpdated(ctx context.Context, userID, productID uuid.UUID, quantity int16) error
	ItemRemoved(ctx context.Context, userID, productID uuid.UUID) error
	Cleared(ctx context.Context, userID uuid.UUID) error
}
