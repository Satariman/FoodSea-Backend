package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/partners/domain"
)

// GetBestOffer returns the cheapest in-stock offer for a product, enriched with store data.
// Implements catalogdomain.BestOfferProvider so it can be injected into the catalog module.
type GetBestOffer struct {
	offers domain.OfferRepository
	stores domain.StoreRepository
}

func NewGetBestOffer(offers domain.OfferRepository, stores domain.StoreRepository) *GetBestOffer {
	return &GetBestOffer{offers: offers, stores: stores}
}

func (uc *GetBestOffer) GetBestOffer(ctx context.Context, productID uuid.UUID) (*catalogdomain.BestOffer, error) {
	offers, err := uc.offers.ListByProduct(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("GetBestOffer: %w", err)
	}

	// offers are ordered by price asc from the repo; pick first in-stock one.
	var best *domain.Offer
	for i := range offers {
		if offers[i].InStock {
			best = &offers[i]
			break
		}
	}
	if best == nil {
		return nil, nil
	}

	store, err := uc.stores.GetByID(ctx, best.StoreID)
	if err != nil || store == nil {
		return nil, nil
	}

	result := &catalogdomain.BestOffer{
		StoreName:    store.Name,
		StoreSlug:    store.Slug,
		PriceKopecks: best.PriceKopecks,
	}
	if best.HasDiscount() {
		result.OriginalPriceKopecks = best.OriginalPriceKopecks
		result.DiscountPercent = best.DiscountPercent
	}
	return result, nil
}
