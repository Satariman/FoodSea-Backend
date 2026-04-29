package domain

import (
	"context"

	"github.com/google/uuid"
)

// ProductCache is the Cache-Aside abstraction for catalog data.
// A cache miss is represented by (nil, nil) — not an error.
type ProductCache interface {
	GetProduct(ctx context.Context, id uuid.UUID) (*ProductDetail, error)
	SetProduct(ctx context.Context, product *ProductDetail) error
	GetCategoriesTree(ctx context.Context) ([]Category, error)
	SetCategoriesTree(ctx context.Context, tree []Category) error
	Invalidate(ctx context.Context, productID uuid.UUID) error
}
