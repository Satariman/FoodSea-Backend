package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
)

// GetResult retrieves persisted optimization snapshot.
type GetResult struct {
	repo domain.ResultRepository
	log  *slog.Logger
}

func NewGetResult(repo domain.ResultRepository, log *slog.Logger) *GetResult {
	return &GetResult{repo: repo, log: log}
}

func (uc *GetResult) Execute(ctx context.Context, resultID uuid.UUID) (*domain.OptimizationResult, error) {
	result, err := uc.repo.GetByID(ctx, resultID)
	if err != nil {
		return nil, err
	}
	return result, nil
}
