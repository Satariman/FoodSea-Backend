package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/foodsea/optimization/internal/modules/analogs/domain"
	"github.com/foodsea/optimization/internal/platform/cache"
)

const analogsCacheTTL = time.Hour

// GetAnalogsForProduct resolves analogs for product card flow.
type GetAnalogsForProduct struct {
	provider domain.AnalogProvider
	cache    cache.Cache
	log      *slog.Logger
}

func NewGetAnalogsForProduct(provider domain.AnalogProvider, cache cache.Cache, log *slog.Logger) *GetAnalogsForProduct {
	return &GetAnalogsForProduct{provider: provider, cache: cache, log: log}
}

func (uc *GetAnalogsForProduct) Execute(ctx context.Context, productID uuid.UUID, topK int) ([]domain.Analog, error) {
	if topK <= 0 {
		topK = 5
	}

	cacheKey := fmt.Sprintf("analogs:simple:%s:%d", productID.String(), topK)
	if uc.cache != nil {
		var cached []domain.Analog
		hit, err := uc.cache.Get(ctx, cacheKey, &cached)
		if err != nil {
			uc.log.WarnContext(ctx, "analogs cache get failed", "key", cacheKey, "error", err)
		} else if hit {
			return cached, nil
		}
	}

	analogs, err := uc.provider.GetAnalogs(ctx, productID, topK)
	if err != nil {
		return nil, err
	}

	if uc.cache != nil {
		if err = uc.cache.Set(ctx, cacheKey, analogs, analogsCacheTTL); err != nil {
			uc.log.WarnContext(ctx, "analogs cache set failed", "key", cacheKey, "error", err)
		}
	}

	return analogs, nil
}
