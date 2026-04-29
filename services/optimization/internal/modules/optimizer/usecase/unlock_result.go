package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
)

// UnlockResult marks result as active after saga compensation.
type UnlockResult struct {
	repo   domain.ResultRepository
	events domain.OptimizationEventPublisher
	log    *slog.Logger
}

func NewUnlockResult(repo domain.ResultRepository, events domain.OptimizationEventPublisher, log *slog.Logger) *UnlockResult {
	return &UnlockResult{repo: repo, events: events, log: log}
}

func (uc *UnlockResult) Execute(ctx context.Context, resultID uuid.UUID) error {
	if err := uc.repo.Unlock(ctx, resultID); err != nil {
		return err
	}
	if err := uc.events.ResultUnlocked(ctx, resultID); err != nil {
		uc.log.WarnContext(ctx, "failed to publish result unlocked event", "result_id", resultID, "error", err)
	}
	return nil
}
