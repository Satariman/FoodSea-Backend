package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/foodsea/core/internal/modules/catalog/domain"
)

// ListProducts returns a paginated product list according to the given filter.
type ListProducts struct {
	products   domain.ProductRepository
	categories domain.CategoryRepository
	log        *slog.Logger
}

func NewListProducts(products domain.ProductRepository, categories domain.CategoryRepository, log *slog.Logger) *ListProducts {
	return &ListProducts{products: products, categories: categories, log: log}
}

func (uc *ListProducts) Execute(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int, error) {
	if filter.Pagination.PageSize > 100 {
		filter.Pagination.PageSize = 100
	}

	// When only subcategory is set, resolve the parent category.
	if filter.SubcategoryID != nil && filter.CategoryID == nil {
		sub, err := uc.categories.GetByID(ctx, *filter.SubcategoryID)
		if err != nil {
			return nil, 0, fmt.Errorf("catalog.ListProducts resolve parent: %w", err)
		}
		filter.CategoryID = sub.ParentID
	}

	items, total, err := uc.products.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("catalog.ListProducts: %w", err)
	}

	return items, total, nil
}
