package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
)

// LockResult marks result as locked for ordering saga.
type LockResult struct {
	repo   domain.ResultRepository
	events domain.OptimizationEventPublisher
	log    *slog.Logger
}

func NewLockResult(repo domain.ResultRepository, events domain.OptimizationEventPublisher, log *slog.Logger) *LockResult {
	return &LockResult{repo: repo, events: events, log: log}
}

func (uc *LockResult) Execute(ctx context.Context, resultID uuid.UUID) error {
	if err := uc.repo.Lock(ctx, resultID); err != nil {
		return err
	}
	if err := uc.events.ResultLocked(ctx, resultID); err != nil {
		uc.log.WarnContext(ctx, "failed to publish result locked event", "result_id", resultID, "error", err)
	}
	return nil
}
