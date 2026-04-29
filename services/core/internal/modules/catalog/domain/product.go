package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Product holds core product data without relationships.
type Product struct {
	ID                 uuid.UUID
	Name               string
	Description        *string
	Composition        *string
	Weight             *string
	Barcode            *string
	ImageURL           *string
	InStock            bool
	CategoryID         uuid.UUID
	SubcategoryID      *uuid.UUID
	BrandID            *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	MinPriceKopecks    *int64
	MaxDiscountPercent *int8
}

// Nutrition holds per-100g/ml nutritional values.
type Nutrition struct {
	Calories      float64
	Protein       float64
	Fat           float64
	Carbohydrates float64
}

// BestOffer holds the cheapest in-stock offer for a product, enriched with store info.
type BestOffer struct {
	StoreName            string
	StoreSlug            string
	PriceKopecks         int64
	OriginalPriceKopecks *int64
	DiscountPercent      int8
}

// OfferBrief contains only data needed by ML indexing.
type OfferBrief struct {
	StoreID      uuid.UUID
	PriceKopecks int64
}

// ProductMLData contains in-stock product data exported to ml-service.
type ProductMLData struct {
	ID            uuid.UUID
	Name          string
	Description   *string
	Composition   *string
	CategoryID    uuid.UUID
	SubcategoryID *uuid.UUID
	BrandID       *uuid.UUID
	Weight        *string
	Nutrition     *Nutrition
	Offers        []OfferBrief
}

// BestOfferProvider fetches the best in-stock offer for a product.
// Implemented by the partners module; injected into catalog via DI.
type BestOfferProvider interface {
	GetBestOffer(ctx context.Context, productID uuid.UUID) (*BestOffer, error)
}

// ProductDetail is the full product card: product + all related entities.
type ProductDetail struct {
	Product
	Category    Category
	Subcategory *Category
	Brand       *Brand
	Nutrition   *Nutrition
	BestOffer   *BestOffer
}
