package handler

import "github.com/foodsea/core/internal/modules/partners/domain"

// StoreResponse is the public store representation.
type StoreResponse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	LogoURL  *string `json:"logo_url,omitempty"`
	IsActive bool    `json:"is_active"`
}

// StoreBriefResponse is the minimal store card embedded in offer responses.
type StoreBriefResponse struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	LogoURL *string `json:"logo_url,omitempty"`
}

// OfferResponse is the per-store price item for the "compare prices" screen.
type OfferResponse struct {
	Store                StoreBriefResponse `json:"store"`
	PriceKopecks         int64              `json:"price_kopecks"`
	OriginalPriceKopecks *int64             `json:"original_price_kopecks,omitempty"`
	DiscountPercent      *int8              `json:"discount_percent,omitempty"`
	InStock              bool               `json:"in_stock"`
}

func toStoreResponse(s domain.Store) StoreResponse {
	return StoreResponse{
		ID:       s.ID.String(),
		Name:     s.Name,
		Slug:     s.Slug,
		LogoURL:  s.LogoURL,
		IsActive: s.IsActive,
	}
}

func toOfferResponse(o domain.OfferWithStore) OfferResponse {
	resp := OfferResponse{
		Store: StoreBriefResponse{
			ID:      o.Store.ID.String(),
			Name:    o.Store.Name,
			LogoURL: o.Store.LogoURL,
		},
		PriceKopecks: o.PriceKopecks,
		InStock:      o.InStock,
	}
	if o.HasDiscount() {
		resp.OriginalPriceKopecks = o.OriginalPriceKopecks
		dp := o.DiscountPercent
		resp.DiscountPercent = &dp
	}
	return resp
}
