package domain

import (
	"github.com/google/uuid"

	shared "github.com/foodsea/core/internal/shared/domain"
)

// SortOption defines the available search sort orderings.
type SortOption string

const (
	SortRelevance    SortOption = "relevance"
	SortPriceAsc     SortOption = "price_asc"
	SortPriceDesc    SortOption = "price_desc"
	SortNameAsc      SortOption = "name_asc"
	SortNameDesc     SortOption = "name_desc"
	SortDiscountDesc SortOption = "discount_desc"
)

var validSortOptions = map[SortOption]struct{}{
	SortRelevance:    {},
	SortPriceAsc:     {},
	SortPriceDesc:    {},
	SortNameAsc:      {},
	SortNameDesc:     {},
	SortDiscountDesc: {},
}

// IsValid reports whether the sort option is in the allowed set.
func (s SortOption) IsValid() bool {
	_, ok := validSortOptions[s]
	return ok
}

// ProductBrief is the lightweight product representation used in search results.
// It does NOT import catalog/domain to maintain module isolation.
type ProductBrief struct {
	ID            uuid.UUID
	Name          string
	ImageURL      *string
	Barcode       *string
	InStock       bool
	CategoryID    uuid.UUID
	SubcategoryID *uuid.UUID
	BrandID       *uuid.UUID
}

// SearchQuery describes a search request with all filter/sort/pagination parameters.
type SearchQuery struct {
	Text            string
	CategoryID      *uuid.UUID
	SubcategoryID   *uuid.UUID
	BrandID         *uuid.UUID
	StoreID         *uuid.UUID
	MinPriceKopecks *int64
	MaxPriceKopecks *int64
	InStockOnly     bool
	HasDiscountOnly bool
	Sort            SortOption
	Pagination      shared.Pagination
}

// SearchResultItem is a single item in a search result.
type SearchResultItem struct {
	Product            ProductBrief
	MinPriceKopecks    int64
	MaxDiscountPercent int8
	Score              float64
	OffersCount        int16
}

// SearchResult is the full paginated search response.
type SearchResult struct {
	Items []SearchResultItem
	Total int
}
