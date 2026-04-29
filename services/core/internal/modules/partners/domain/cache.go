package domain

import (
	"context"

	"github.com/google/uuid"
)

type OfferCache interface {
	GetOffersByProduct(ctx context.Context, productID uuid.UUID) ([]Offer, error)
	SetOffersByProduct(ctx context.Context, productID uuid.UUID, offers []Offer) error
	Invalidate(ctx context.Context, productID uuid.UUID) error
}
