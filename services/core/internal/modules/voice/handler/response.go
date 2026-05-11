package handler

import "github.com/foodsea/core/internal/modules/voice/domain"

type VoiceItemDTO struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int32   `json:"quantity"`
	Unit        string  `json:"unit"`
	Confidence  float64 `json:"confidence"`
	RawQuery    string  `json:"raw_query"`
}

type ParseVoiceResponseDTO struct {
	Items            []VoiceItemDTO `json:"items"`
	UnmatchedQueries []string       `json:"unmatched_queries"`
}

func ToResponseDTO(items []domain.VoiceItem, unmatched []string) ParseVoiceResponseDTO {
	dtoItems := make([]VoiceItemDTO, len(items))
	for i, it := range items {
		dtoItems[i] = VoiceItemDTO{
			ProductID:   it.ProductID,
			ProductName: it.ProductName,
			Quantity:    it.Quantity,
			Unit:        it.Unit,
			Confidence:  it.Confidence,
			RawQuery:    it.RawQuery,
		}
	}
	if unmatched == nil {
		unmatched = []string{}
	}
	return ParseVoiceResponseDTO{Items: dtoItems, UnmatchedQueries: unmatched}
}
