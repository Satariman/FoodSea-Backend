package domain

import "context"

// OptimizationResultRepository определяет интерфейс для работы с результатами оптимизации
type OptimizationResultRepository interface {
	Save(ctx context.Context, result *OptimizationResult) error
	GetByID(ctx context.Context, id int64) (*OptimizationResult, error)
	GetByClientID(ctx context.Context, clientID string) ([]*OptimizationResult, error)
}

