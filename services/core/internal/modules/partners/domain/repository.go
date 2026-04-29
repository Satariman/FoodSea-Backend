package domain

import (
	"context"

	"github.com/google/uuid"
)

type StoreRepository interface {
	ListActive(ctx context.Context) ([]Store, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Store, error)
}

type OfferRepository interface {
	ListByProduct(ctx context.Context, productID uuid.UUID) ([]Offer, error)
	ListByProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]Offer, error)
}

type DeliveryRepository interface {
	ListByStores(ctx context.Context, storeIDs []uuid.UUID) (map[uuid.UUID]DeliveryCondition, error)
	GetByStore(ctx context.Context, storeID uuid.UUID) (*DeliveryCondition, error)
}
