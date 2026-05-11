package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/photo_search/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

const (
	defaultTopK           = 5
	maxTopK               = 10
	minOCRLen             = 3
	maxOCRLen             = 4000
	defaultMLCallTimeout  = 5 * time.Second
)

type SearchByPhoto struct {
	client domain.PhotoSearchClient
	loader domain.ProductLoader
}

func NewSearchByPhoto(client domain.PhotoSearchClient, loader domain.ProductLoader) *SearchByPhoto {
	return &SearchByPhoto{client: client, loader: loader}
}

func (uc *SearchByPhoto) Execute(ctx context.Context, req domain.SearchByPhotoRequest) (domain.SearchResult, error) {
	normalized, err := normalizeRequest(req)
	if err != nil {
		return domain.SearchResult{}, err
	}

	mlCtx, cancel := context.WithTimeout(ctx, defaultMLCallTimeout)
	defer cancel()

	mlResult, err := uc.client.SearchByPhoto(mlCtx, normalized)
	if err != nil {
		return domain.SearchResult{}, err
	}

	out := domain.SearchResult{
		MatchedName:  mlResult.MatchedName,
		MatchedBrand: mlResult.MatchedBrand,
		Candidates:   make([]domain.ProductCandidate, 0, len(mlResult.Candidates)),
	}

	for _, candidate := range mlResult.Candidates {
		if candidate.ProductID == uuid.Nil {
			continue
		}

		product, loadErr := uc.loader.Execute(ctx, candidate.ProductID)
		if loadErr != nil {
			if errors.Is(loadErr, sherrors.ErrNotFound) {
				continue
			}
			return domain.SearchResult{}, fmt.Errorf("load product %s: %w", candidate.ProductID, loadErr)
		}
		out.Candidates = append(out.Candidates, domain.ProductCandidate{
			Product: product,
			Score:   candidate.Score,
			Source:  "image_ocr",
		})
	}

	return out, nil
}

func normalizeRequest(req domain.SearchByPhotoRequest) (domain.SearchByPhotoRequest, error) {
	if len(req.Image) == 0 {
		return domain.SearchByPhotoRequest{}, fmt.Errorf("%w: image is required", sherrors.ErrInvalidInput)
	}

	ocrText := strings.TrimSpace(req.OCRText)
	if len(ocrText) < minOCRLen || len(ocrText) > maxOCRLen {
		return domain.SearchByPhotoRequest{}, &sherrors.ValidationError{
			Field:   "ocr_text",
			Message: "must be between 3 and 4000 characters",
		}
	}

	topK := req.TopK
	if topK == 0 {
		topK = defaultTopK
	}
	if topK < 1 || topK > maxTopK {
		return domain.SearchByPhotoRequest{}, &sherrors.ValidationError{
			Field:   "top_k",
			Message: "must be between 1 and 10",
		}
	}

	mime := strings.TrimSpace(strings.ToLower(req.ImageMimeType))
	if mime != "image/jpeg" && mime != "image/png" {
		return domain.SearchByPhotoRequest{}, fmt.Errorf("%w: unsupported image mime type %q", sherrors.ErrInvalidInput, req.ImageMimeType)
	}

	req.OCRText = ocrText
	req.TopK = topK
	req.ImageMimeType = mime
	return req, nil
}
