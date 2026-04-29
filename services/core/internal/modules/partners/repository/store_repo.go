package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	entstore "github.com/foodsea/core/ent/store"
	"github.com/foodsea/core/internal/modules/partners/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type StoreRepo struct {
	client *ent.Client
}

func NewStoreRepo(client *ent.Client) *StoreRepo {
	return &StoreRepo{client: client}
}

func (r *StoreRepo) ListActive(ctx context.Context) ([]domain.Store, error) {
	rows, err := r.client.Store.Query().
		Where(entstore.IsActive(true)).
		Order(ent.Asc(entstore.FieldName)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing active stores: %w", err)
	}
	result := make([]domain.Store, len(rows))
	for i, row := range rows {
		result[i] = toDomainStore(row)
	}
	return result, nil
}

func (r *StoreRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Store, error) {
	row, err := r.client.Store.Query().
		Where(entstore.ID(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting store by id: %w", err)
	}
	s := toDomainStore(row)
	return &s, nil
}

func toDomainStore(e *ent.Store) domain.Store {
	return domain.Store{
		ID:       e.ID,
		Name:     e.Name,
		Slug:     e.Slug,
		LogoURL:  e.LogoURL,
		IsActive: e.IsActive,
	}
}
