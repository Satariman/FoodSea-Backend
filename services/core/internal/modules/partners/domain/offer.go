package domain

import (
	"time"

	"github.com/google/uuid"
)

type Offer struct {
	ID                   uuid.UUID
	ProductID            uuid.UUID
	StoreID              uuid.UUID
	PriceKopecks         int64
	OriginalPriceKopecks *int64
	DiscountPercent      int8
	InStock              bool
	UpdatedAt            time.Time
}

func (o Offer) HasDiscount() bool {
	return o.DiscountPercent > 0 && o.OriginalPriceKopecks != nil
}

// OfferWithStore pairs an offer with its store card for HTTP responses.
type OfferWithStore struct {
	Offer
	Store Store
}
