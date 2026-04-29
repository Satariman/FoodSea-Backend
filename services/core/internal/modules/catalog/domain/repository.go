package domain

import (
	"context"

	"github.com/google/uuid"
)

// CategoryRepository defines read operations for categories.
type CategoryRepository interface {
	ListAll(ctx context.Context) ([]Category, error)
	GetBySlug(ctx context.Context, slug string) (*Category, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Category, error)
}

// BrandRepository defines read operations for brands.
type BrandRepository interface {
	ListAll(ctx context.Context) ([]Brand, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Brand, error)
}

// ProductRepository defines read operations for products.
type ProductRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Product, error)
	GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*ProductDetail, error)
	GetByBarcode(ctx context.Context, barcode string) (*ProductDetail, error)
	ListAllForML(ctx context.Context) ([]ProductMLData, error)
	// List returns a page of products matching filter, plus the total count.
	List(ctx context.Context, filter ProductFilter) ([]Product, int, error)
}
