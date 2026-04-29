package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/foodsea/core/internal/modules/search/domain"
	"github.com/foodsea/core/internal/platform/cache"
)

const searchCacheTTL = 5 * time.Minute

// SearchCache implements domain.SearchCache using the platform Redis cache.
type SearchCache struct {
	c cache.Cache
}

func NewSearchCache(c cache.Cache) *SearchCache {
	return &SearchCache{c: c}
}

func (s *SearchCache) Get(ctx context.Context, hash string) (*domain.SearchResult, error) {
	var result domain.SearchResult
	hit, err := s.c.Get(ctx, cache.KeySearch+hash, &result)
	if err != nil {
		return nil, fmt.Errorf("search cache get: %w", err)
	}
	if !hit {
		return nil, nil
	}
	return &result, nil
}

func (s *SearchCache) Set(ctx context.Context, hash string, result *domain.SearchResult) error {
	if err := s.c.Set(ctx, cache.KeySearch+hash, result, searchCacheTTL); err != nil {
		return fmt.Errorf("search cache set: %w", err)
	}
	return nil
}
