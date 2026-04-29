package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/foodsea/core/internal/modules/catalog/domain"
)

// ListBrands returns all brands for the filter screen.
type ListBrands struct {
	brands domain.BrandRepository
	log    *slog.Logger
}

func NewListBrands(brands domain.BrandRepository, log *slog.Logger) *ListBrands {
	return &ListBrands{brands: brands, log: log}
}

func (uc *ListBrands) Execute(ctx context.Context) ([]domain.Brand, error) {
	brands, err := uc.brands.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("catalog.ListBrands: %w", err)
	}
	return brands, nil
}
