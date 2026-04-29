package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/platform/cache"
)

const (
	productCacheTTL    = 15 * time.Minute
	categoriesCacheTTL = 30 * time.Minute
)

// ProductCache implements domain.ProductCache on top of platform/cache.Cache.
type ProductCache struct {
	c cache.Cache
}

func NewProductCache(c cache.Cache) *ProductCache {
	return &ProductCache{c: c}
}

func (pc *ProductCache) GetProduct(ctx context.Context, id uuid.UUID) (*domain.ProductDetail, error) {
	var detail domain.ProductDetail
	ok, err := pc.c.Get(ctx, productKey(id), &detail)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &detail, nil
}

func (pc *ProductCache) SetProduct(ctx context.Context, product *domain.ProductDetail) error {
	return pc.c.Set(ctx, productKey(product.ID), product, productCacheTTL)
}

func (pc *ProductCache) GetCategoriesTree(ctx context.Context) ([]domain.Category, error) {
	var tree []domain.Category
	ok, err := pc.c.Get(ctx, cache.KeyCatalogCategories, &tree)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return tree, nil
}

func (pc *ProductCache) SetCategoriesTree(ctx context.Context, tree []domain.Category) error {
	return pc.c.Set(ctx, cache.KeyCatalogCategories, tree, categoriesCacheTTL)
}

func (pc *ProductCache) Invalidate(ctx context.Context, productID uuid.UUID) error {
	return pc.c.Delete(ctx, productKey(productID))
}

func productKey(id uuid.UUID) string {
	return fmt.Sprintf("%s%s", cache.KeyCatalogProduct, id.String())
}
