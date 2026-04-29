package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/catalog/domain"
)

// GetProduct fetches a full product card by ID, with Cache-Aside.
type GetProduct struct {
	products   domain.ProductRepository
	cache      domain.ProductCache
	bestOffers domain.BestOfferProvider // optional; nil disables best_offer enrichment
	log        *slog.Logger
}

func NewGetProduct(
	products domain.ProductRepository,
	cache domain.ProductCache,
	bestOffers domain.BestOfferProvider,
	log *slog.Logger,
) *GetProduct {
	return &GetProduct{products: products, cache: cache, bestOffers: bestOffers, log: log}
}

func (uc *GetProduct) Execute(ctx context.Context, id uuid.UUID) (*domain.ProductDetail, error) {
	cached, err := uc.cache.GetProduct(ctx, id)
	if err != nil {
		uc.log.WarnContext(ctx, "catalog: product cache get error", "error", err)
	}
	if cached != nil {
		return cached, nil
	}

	product, err := uc.products.GetByIDWithDetails(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("catalog.GetProduct(%s): %w", id, err)
	}

	if uc.bestOffers != nil {
		bo, boErr := uc.bestOffers.GetBestOffer(ctx, id)
		if boErr != nil {
			uc.log.WarnContext(ctx, "catalog: best offer fetch error", "error", boErr)
		} else {
			product.BestOffer = bo
		}
	}

	if cacheErr := uc.cache.SetProduct(ctx, product); cacheErr != nil {
		uc.log.WarnContext(ctx, "catalog: product cache set error", "error", cacheErr)
	}

	return product, nil
}
