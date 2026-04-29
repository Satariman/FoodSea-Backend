package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/partners/domain"
)

type GetDeliveryConditions struct {
	delivery domain.DeliveryRepository
	log      *slog.Logger
}

func NewGetDeliveryConditions(delivery domain.DeliveryRepository, log *slog.Logger) *GetDeliveryConditions {
	return &GetDeliveryConditions{delivery: delivery, log: log}
}

func (uc *GetDeliveryConditions) Execute(ctx context.Context, storeIDs []uuid.UUID) (map[uuid.UUID]domain.DeliveryCondition, error) {
	result, err := uc.delivery.ListByStores(ctx, storeIDs)
	if err != nil {
		return nil, fmt.Errorf("partners.GetDeliveryConditions: %w", err)
	}
	return result, nil
}
