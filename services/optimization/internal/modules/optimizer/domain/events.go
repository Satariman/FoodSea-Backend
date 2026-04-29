package domain

import (
	"context"

	"github.com/google/uuid"
)

// OptimizationEventPublisher emits optimization lifecycle events.
type OptimizationEventPublisher interface {
	ResultCreated(ctx context.Context, result *OptimizationResult) error
	ResultLocked(ctx context.Context, resultID uuid.UUID) error
	ResultUnlocked(ctx context.Context, resultID uuid.UUID) error
}
