package handler

import "github.com/foodsea/core/internal/modules/catalog/domain"

// CategoryBriefResponse is the minimal category representation used inside product responses.
type CategoryBriefResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// BrandBriefResponse is the minimal brand representation.
type BrandBriefResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NutritionResponse serialises nutritional values.
type NutritionResponse struct {
	Calories      float64 `json:"calories"`
	Protein       float64 `json:"protein"`
	Fat           float64 `json:"fat"`
	Carbohydrates float64 `json:"carbohydrates"`
}

// ProductBriefResponse is used in list responses.
type ProductBriefResponse struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Weight             *string `json:"weight,omitempty"`
	ImageURL           *string `json:"image_url,omitempty"`
	InStock            bool    `json:"in_stock"`
	MinPriceKopecks    *int64  `json:"min_price_kopecks,omitempty"`
	MaxDiscountPercent *int8   `json:"max_discount_percent,omitempty"`
}

// BestOfferResponse is the condensed best offer embedded in the product card.
type BestOfferResponse struct {
	StoreName            string  `json:"store_name"`
	StoreSlug            string  `json:"store_slug"`
	PriceKopecks         int64   `json:"price_kopecks"`
	OriginalPriceKopecks *int64  `json:"original_price_kopecks,omitempty"`
	DiscountPercent      *int8   `json:"discount_percent,omitempty"`
}

// ProductDetailResponse is the full product card response (§4.2 of 02-tech-stack.md).
type ProductDetailResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Composition *string                `json:"composition,omitempty"`
	Weight      *string                `json:"weight,omitempty"`
	Barcode     *string                `json:"barcode,omitempty"`
	ImageURL    *string                `json:"image_url,omitempty"`
	InStock     bool                   `json:"in_stock"`
	Category    CategoryBriefResponse  `json:"category"`
	Subcategory *CategoryBriefResponse `json:"subcategory,omitempty"`
	Brand       *BrandBriefResponse    `json:"brand,omitempty"`
	Nutrition   *NutritionResponse     `json:"nutrition,omitempty"`
	BestOffer   *BestOfferResponse     `json:"best_offer,omitempty"`
}

// CategoryResponse is the full category node used in the tree response.
type CategoryResponse struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Slug      string             `json:"slug"`
	SortOrder int                `json:"sort_order"`
	Children  []CategoryResponse `json:"children,omitempty"`
}

// BrandResponse is the list-level brand DTO.
type BrandResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// --- mappers ---

func toCategoryBrief(c domain.Category) CategoryBriefResponse {
	return CategoryBriefResponse{ID: c.ID.String(), Name: c.Name}
}

func toBrandBrief(b *domain.Brand) BrandBriefResponse {
	return BrandBriefResponse{ID: b.ID.String(), Name: b.Name}
}

func toNutritionResponse(n *domain.Nutrition) NutritionResponse {
	return NutritionResponse{
		Calories:      n.Calories,
		Protein:       n.Protein,
		Fat:           n.Fat,
		Carbohydrates: n.Carbohydrates,
	}
}

func toProductDetailResponse(p *domain.ProductDetail) ProductDetailResponse {
	resp := ProductDetailResponse{
		ID:          p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		Composition: p.Composition,
		Weight:      p.Weight,
		Barcode:     p.Barcode,
		ImageURL:    p.ImageURL,
		InStock:     p.InStock,
		Category:    toCategoryBrief(p.Category),
	}
	if p.Subcategory != nil {
		s := toCategoryBrief(*p.Subcategory)
		resp.Subcategory = &s
	}
	if p.Brand != nil {
		b := toBrandBrief(p.Brand)
		resp.Brand = &b
	}
	if p.Nutrition != nil {
		n := toNutritionResponse(p.Nutrition)
		resp.Nutrition = &n
	}
	if p.BestOffer != nil {
		bo := toBestOfferResponse(p.BestOffer)
		resp.BestOffer = &bo
	}
	return resp
}

func toBestOfferResponse(bo *domain.BestOffer) BestOfferResponse {
	resp := BestOfferResponse{
		StoreName:    bo.StoreName,
		StoreSlug:    bo.StoreSlug,
		PriceKopecks: bo.PriceKopecks,
	}
	if bo.DiscountPercent > 0 && bo.OriginalPriceKopecks != nil {
		resp.OriginalPriceKopecks = bo.OriginalPriceKopecks
		dp := bo.DiscountPercent
		resp.DiscountPercent = &dp
	}
	return resp
}

func toProductBriefResponse(p domain.Product) ProductBriefResponse {
	return ProductBriefResponse{
		ID:                 p.ID.String(),
		Name:               p.Name,
		Weight:             p.Weight,
		ImageURL:           p.ImageURL,
		InStock:            p.InStock,
		MinPriceKopecks:    p.MinPriceKopecks,
		MaxDiscountPercent: p.MaxDiscountPercent,
	}
}

func toCategoryResponse(c domain.Category) CategoryResponse {
	resp := CategoryResponse{
		ID:        c.ID.String(),
		Name:      c.Name,
		Slug:      c.Slug,
		SortOrder: c.SortOrder,
	}
	for _, child := range c.Children {
		resp.Children = append(resp.Children, toCategoryResponse(child))
	}
	return resp
}

func toBrandResponse(b domain.Brand) BrandResponse {
	return BrandResponse{ID: b.ID.String(), Name: b.Name}
}

// ToProductDetailResponse converts a ProductDetail to the JSON DTO.
// Exported for use by the barcode module.
func ToProductDetailResponse(p *domain.ProductDetail) ProductDetailResponse {
	return toProductDetailResponse(p)
}
