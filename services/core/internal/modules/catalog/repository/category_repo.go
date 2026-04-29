package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	entcategory "github.com/foodsea/core/ent/category"
	"github.com/foodsea/core/internal/modules/catalog/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// CategoryRepo implements domain.CategoryRepository using Ent.
type CategoryRepo struct {
	client *ent.Client
}

func NewCategoryRepo(client *ent.Client) *CategoryRepo {
	return &CategoryRepo{client: client}
}

func (r *CategoryRepo) ListAll(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.client.Category.Query().
		Order(ent.Asc(entcategory.FieldSortOrder), ent.Asc(entcategory.FieldName)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}

	result := make([]domain.Category, len(rows))
	for i, row := range rows {
		result[i] = toDomainCategory(row)
	}
	return result, nil
}

func (r *CategoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	row, err := r.client.Category.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting category by id: %w", err)
	}
	c := toDomainCategory(row)
	return &c, nil
}

func (r *CategoryRepo) GetBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	row, err := r.client.Category.Query().
		Where(entcategory.SlugEQ(slug)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting category by slug: %w", err)
	}
	c := toDomainCategory(row)
	return &c, nil
}

func toDomainCategory(e *ent.Category) domain.Category {
	return domain.Category{
		ID:        e.ID,
		Name:      e.Name,
		Slug:      e.Slug,
		ParentID:  e.ParentID,
		SortOrder: e.SortOrder,
	}
}
