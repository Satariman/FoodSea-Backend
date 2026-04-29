package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/partners/domain"
	"github.com/foodsea/core/internal/platform/cache"
)

const offerCacheTTL = 10 * time.Minute

type OfferCache struct {
	c cache.Cache
}

func NewOfferCache(c cache.Cache) *OfferCache {
	return &OfferCache{c: c}
}

func (oc *OfferCache) GetOffersByProduct(ctx context.Context, productID uuid.UUID) ([]domain.Offer, error) {
	var offers []domain.Offer
	ok, err := oc.c.Get(ctx, offerKey(productID), &offers)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return offers, nil
}

func (oc *OfferCache) SetOffersByProduct(ctx context.Context, productID uuid.UUID, offers []domain.Offer) error {
	return oc.c.Set(ctx, offerKey(productID), offers, offerCacheTTL)
}

func (oc *OfferCache) Invalidate(ctx context.Context, productID uuid.UUID) error {
	return oc.c.Delete(ctx, offerKey(productID))
}

func offerKey(productID uuid.UUID) string {
	return fmt.Sprintf("%s%s", cache.KeyOffersProduct, productID.String())
}
