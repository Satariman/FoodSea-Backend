package handler

import "github.com/foodsea/optimization/internal/modules/optimizer/domain"

type OptimizationResultResponse struct {
	ID              string            `json:"id"`
	TotalKopecks    int64             `json:"total_kopecks"`
	DeliveryKopecks int64             `json:"delivery_kopecks"`
	SavingsKopecks  int64             `json:"savings_kopecks"`
	Status          string            `json:"status"`
	IsApproximate   bool              `json:"is_approximate"`
	Items           []AssignmentDTO   `json:"items"`
	Substitutions   []SubstitutionDTO `json:"substitutions"`
}

type AssignmentDTO struct {
	ProductID    string `json:"product_id"`
	ProductName  string `json:"product_name"`
	StoreID      string `json:"store_id"`
	StoreName    string `json:"store_name"`
	Quantity     int    `json:"quantity"`
	PriceKopecks int64  `json:"price_kopecks"`
}

type SubstitutionDTO struct {
	OriginalProductID    string  `json:"original_product_id"`
	OriginalProductName  string  `json:"original_product_name"`
	AnalogProductID      string  `json:"analog_product_id"`
	AnalogProductName    string  `json:"analog_product_name"`
	OriginalStoreID      string  `json:"original_store_id"`
	NewStoreID           string  `json:"new_store_id"`
	NewStoreName         string  `json:"new_store_name"`
	OldPriceKopecks      int64   `json:"old_price_kopecks"`
	NewPriceKopecks      int64   `json:"new_price_kopecks"`
	PriceDeltaKopecks    int64   `json:"price_delta_kopecks"`
	DeliveryDeltaKopecks int64   `json:"delivery_delta_kopecks"`
	TotalSavingKopecks   int64   `json:"total_saving_kopecks"`
	Score                float64 `json:"score"`
	IsCrossStore         bool    `json:"is_cross_store"`
}

type AnalogDTO struct {
	ProductID       string  `json:"product_id"`
	ProductName     string  `json:"product_name"`
	Score           float64 `json:"score"`
	MinPriceKopecks int64   `json:"min_price_kopecks"`
}

type AnalogsResponse struct {
	Analogs []AnalogDTO `json:"analogs"`
}

func toOptimizationResultResponse(result *domain.OptimizationResult) OptimizationResultResponse {
	items := make([]AssignmentDTO, len(result.Items))
	for i, item := range result.Items {
		items[i] = AssignmentDTO{
			ProductID:    item.ProductID.String(),
			ProductName:  item.ProductName,
			StoreID:      item.StoreID.String(),
			StoreName:    item.StoreName,
			Quantity:     item.Quantity,
			PriceKopecks: item.Price,
		}
	}

	substitutions := make([]SubstitutionDTO, len(result.Substitutions))
	for i := range result.Substitutions {
		sub := &result.Substitutions[i]
		substitutions[i] = SubstitutionDTO{
			OriginalProductID:    sub.OriginalID.String(),
			OriginalProductName:  sub.OriginalProductName,
			AnalogProductID:      sub.AnalogID.String(),
			AnalogProductName:    sub.AnalogProductName,
			OriginalStoreID:      sub.OriginalStoreID.String(),
			NewStoreID:           sub.NewStoreID.String(),
			NewStoreName:         sub.NewStoreName,
			OldPriceKopecks:      sub.OldPriceKopecks,
			NewPriceKopecks:      sub.NewPriceKopecks,
			PriceDeltaKopecks:    sub.PriceDeltaKopecks,
			DeliveryDeltaKopecks: sub.DeliveryDeltaKopecks,
			TotalSavingKopecks:   sub.TotalSavingKopecks,
			Score:                sub.Score,
			IsCrossStore:         sub.IsCrossStore,
		}
	}

	return OptimizationResultResponse{
		ID:              result.ID.String(),
		TotalKopecks:    result.TotalKopecks,
		DeliveryKopecks: result.DeliveryKopecks,
		SavingsKopecks:  result.SavingsKopecks,
		Status:          result.Status,
		IsApproximate:   result.IsApproximate,
		Items:           items,
		Substitutions:   substitutions,
	}
}
