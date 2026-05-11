package handler

import (
	cataloghandler "github.com/foodsea/core/internal/modules/catalog/handler"
	"github.com/foodsea/core/internal/modules/photo_search/domain"
)

type searchByPhotoResponse struct {
	MatchedName  string                     `json:"matched_name"`
	MatchedBrand string                     `json:"matched_brand"`
	Candidates   []photoSearchCandidateItem `json:"candidates"`
}

type photoSearchCandidateItem struct {
	Product cataloghandler.ProductDetailResponse `json:"product"`
	Score   float64                              `json:"score"`
	Source  string                               `json:"source"`
}

func toResponse(result domain.SearchResult) searchByPhotoResponse {
	resp := searchByPhotoResponse{
		MatchedName:  result.MatchedName,
		MatchedBrand: result.MatchedBrand,
		Candidates:   make([]photoSearchCandidateItem, 0, len(result.Candidates)),
	}

	for _, candidate := range result.Candidates {
		resp.Candidates = append(resp.Candidates, photoSearchCandidateItem{
			Product: cataloghandler.ToProductDetailResponse(candidate.Product),
			Score:   candidate.Score,
			Source:  candidate.Source,
		})
	}

	return resp
}
