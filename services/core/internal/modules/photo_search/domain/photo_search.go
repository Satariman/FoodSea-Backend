package domain

import (
	"context"

	"github.com/google/uuid"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
)

type SearchByPhotoRequest struct {
	Image         []byte
	ImageMimeType string
	OCRText       string
	TopK          int
}

type Candidate struct {
	ProductID uuid.UUID
	Score     float64
}

type SearchByPhotoResult struct {
	MatchedName  string
	MatchedBrand string
	Candidates   []Candidate
}

type ProductCandidate struct {
	Product *catalogdomain.ProductDetail
	Score   float64
	Source  string
}

type SearchResult struct {
	MatchedName  string
	MatchedBrand string
	Candidates   []ProductCandidate
}

type PhotoSearchClient interface {
	SearchByPhoto(ctx context.Context, req SearchByPhotoRequest) (SearchByPhotoResult, error)
}

type ProductLoader interface {
	Execute(ctx context.Context, id uuid.UUID) (*catalogdomain.ProductDetail, error)
}
