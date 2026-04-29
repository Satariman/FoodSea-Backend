package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/catalog/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// BrandRepo implements domain.BrandRepository using Ent.
type BrandRepo struct {
	client *ent.Client
}

func NewBrandRepo(client *ent.Client) *BrandRepo {
	return &BrandRepo{client: client}
}

func (r *BrandRepo) ListAll(ctx context.Context) ([]domain.Brand, error) {
	rows, err := r.client.Brand.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing brands: %w", err)
	}

	result := make([]domain.Brand, len(rows))
	for i, row := range rows {
		result[i] = toDomainBrand(row)
	}
	return result, nil
}

func (r *BrandRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Brand, error) {
	row, err := r.client.Brand.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting brand by id: %w", err)
	}
	b := toDomainBrand(row)
	return &b, nil
}

func toDomainBrand(e *ent.Brand) domain.Brand {
	return domain.Brand{
		ID:   e.ID,
		Name: e.Name,
	}
}
