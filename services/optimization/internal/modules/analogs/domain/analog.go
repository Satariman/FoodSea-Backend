package domain

import (
	"context"

	"github.com/google/uuid"
)

// Analog is an ML-provided substitute candidate for a product.
type Analog struct {
	ProductID       uuid.UUID
	ProductName     string
	Score           float64
	MinPriceKopecks int64
}

// AnalogProvider retrieves analog products from ml-service.
type AnalogProvider interface {
	// GetAnalogs returns top-k analogs for a single product.
	GetAnalogs(ctx context.Context, productID uuid.UUID, topK int) ([]Analog, error)

	// GetBatchAnalogsForStores returns analogs for many products filtered by stores.
	GetBatchAnalogsForStores(
		ctx context.Context,
		productIDs []uuid.UUID,
		topK int,
		storeIDs []uuid.UUID,
	) (map[uuid.UUID][]Analog, error)
}
