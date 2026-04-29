package domain

import (
	"github.com/google/uuid"

	shareddomain "github.com/foodsea/core/internal/shared/domain"
)

// ProductSort enumerates allowed sort orders for the products list.
type ProductSort string

const (
	SortNameAsc     ProductSort = "name_asc"
	SortNameDesc    ProductSort = "name_desc"
	SortCreatedDesc ProductSort = "created_desc"
)

// ProductFilter holds all filtering, sorting, and pagination parameters for product list queries.
type ProductFilter struct {
	CategoryID    *uuid.UUID
	SubcategoryID *uuid.UUID
	BrandID       *uuid.UUID
	InStockOnly   bool
	Pagination    shareddomain.Pagination
	Sort          ProductSort
}
