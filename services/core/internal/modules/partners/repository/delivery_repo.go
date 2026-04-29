package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	entdc "github.com/foodsea/core/ent/deliverycondition"
	entstore "github.com/foodsea/core/ent/store"
	"github.com/foodsea/core/internal/modules/partners/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type DeliveryRepo struct {
	client *ent.Client
}

func NewDeliveryRepo(client *ent.Client) *DeliveryRepo {
	return &DeliveryRepo{client: client}
}

func (r *DeliveryRepo) ListByStores(ctx context.Context, storeIDs []uuid.UUID) (map[uuid.UUID]domain.DeliveryCondition, error) {
	rows, err := r.client.DeliveryCondition.Query().
		Where(entdc.HasStoreWith(entstore.IDIn(storeIDs...))).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing delivery conditions: %w", err)
	}

	result := make(map[uuid.UUID]domain.DeliveryCondition, len(rows))
	for _, row := range rows {
		result[row.StoreID] = toDomainDelivery(row)
	}
	return result, nil
}

func (r *DeliveryRepo) GetByStore(ctx context.Context, storeID uuid.UUID) (*domain.DeliveryCondition, error) {
	row, err := r.client.DeliveryCondition.Query().
		Where(entdc.HasStoreWith(entstore.ID(storeID))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting delivery condition: %w", err)
	}
	dc := toDomainDelivery(row)
	return &dc, nil
}

func toDomainDelivery(e *ent.DeliveryCondition) domain.DeliveryCondition {
	dc := domain.DeliveryCondition{
		StoreID:             e.StoreID,
		MinOrderKopecks:     int64(e.MinOrderKopecks),
		DeliveryCostKopecks: int64(e.DeliveryCostKopecks),
	}
	if e.FreeFromKopecks != nil {
		v := int64(*e.FreeFromKopecks)
		dc.FreeFromKopecks = &v
	}
	if e.EstimatedMinutes != nil {
		v := int32(*e.EstimatedMinutes)
		dc.EstimatedMinutes = &v
	}
	return dc
}
