package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ResultRepository persists optimization snapshots.
type ResultRepository interface {
	Save(ctx context.Context, result *OptimizationResult) error
	GetByID(ctx context.Context, id uuid.UUID) (*OptimizationResult, error)
	FindByCartHash(ctx context.Context, cartHash string) (*OptimizationResult, error)
	Lock(ctx context.Context, id uuid.UUID) error
	Unlock(ctx context.Context, id uuid.UUID) error
	ExpireOld(ctx context.Context, olderThan time.Time) (int, error)
	DeleteByUserCartHash(ctx context.Context, userID uuid.UUID, cartHash string) error
}
