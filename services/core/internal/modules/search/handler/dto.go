package handler

import "github.com/foodsea/core/internal/modules/search/domain"

// SearchResultItemResponse is the HTTP representation of a single search result.
type SearchResultItemResponse struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	ImageURL           *string `json:"image_url,omitempty"`
	Barcode            *string `json:"barcode,omitempty"`
	InStock            bool    `json:"in_stock"`
	CategoryID         string  `json:"category_id"`
	SubcategoryID      *string `json:"subcategory_id,omitempty"`
	BrandID            *string `json:"brand_id,omitempty"`
	MinPriceKopecks    int64   `json:"min_price_kopecks"`
	MaxDiscountPercent *int8   `json:"max_discount_percent,omitempty"`
	Score              float64 `json:"score"`
	OffersCount        int16   `json:"offers_count"`
}

func toSearchResultItemResponse(item domain.SearchResultItem) SearchResultItemResponse {
	resp := SearchResultItemResponse{
		ID:              item.Product.ID.String(),
		Name:            item.Product.Name,
		ImageURL:        item.Product.ImageURL,
		Barcode:         item.Product.Barcode,
		InStock:         item.Product.InStock,
		CategoryID:      item.Product.CategoryID.String(),
		MinPriceKopecks: item.MinPriceKopecks,
		Score:           item.Score,
		OffersCount:     item.OffersCount,
	}
	if item.MaxDiscountPercent > 0 {
		dp := item.MaxDiscountPercent
		resp.MaxDiscountPercent = &dp
	}
	if item.Product.SubcategoryID != nil {
		s := item.Product.SubcategoryID.String()
		resp.SubcategoryID = &s
	}
	if item.Product.BrandID != nil {
		b := item.Product.BrandID.String()
		resp.BrandID = &b
	}
	return resp
}
