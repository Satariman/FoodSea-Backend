package grpc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/foodsea/core/internal/modules/photo_search/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
	pbml "github.com/foodsea/proto/ml"
)

type MLClient struct {
	client pbml.AnalogServiceClient
}

func NewMLClient(client pbml.AnalogServiceClient) *MLClient {
	return &MLClient{client: client}
}

func (c *MLClient) SearchByPhoto(ctx context.Context, req domain.SearchByPhotoRequest) (domain.SearchByPhotoResult, error) {
	resp, err := c.client.SearchByPhoto(ctx, &pbml.SearchByPhotoRequest{
		Image:         req.Image,
		ImageMimeType: req.ImageMimeType,
		OcrText:       req.OCRText,
		TopK:          int32(req.TopK),
	})
	if err != nil {
		st, ok := grpcstatus.FromError(err)
		if ok && (st.Code() == codes.Unavailable || st.Code() == codes.FailedPrecondition) {
			return domain.SearchByPhotoResult{}, fmt.Errorf("%w: ml search by photo unavailable", sherrors.ErrUnavailable)
		}
		return domain.SearchByPhotoResult{}, fmt.Errorf("ml search by photo: %w", err)
	}

	out := domain.SearchByPhotoResult{
		MatchedName:  resp.GetMatchedName(),
		MatchedBrand: resp.GetMatchedBrand(),
		Candidates:   make([]domain.Candidate, 0, len(resp.GetCandidates())),
	}

	for _, candidate := range resp.GetCandidates() {
		id, parseErr := uuid.Parse(candidate.GetProductId())
		if parseErr != nil {
			continue
		}
		out.Candidates = append(out.Candidates, domain.Candidate{
			ProductID: id,
			Score:     candidate.GetScore(),
		})
	}

	return out, nil
}
