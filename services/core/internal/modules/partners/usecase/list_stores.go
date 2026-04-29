package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/foodsea/core/internal/modules/partners/domain"
)

type ListStores struct {
	stores domain.StoreRepository
	log    *slog.Logger
}

func NewListStores(stores domain.StoreRepository, log *slog.Logger) *ListStores {
	return &ListStores{stores: stores, log: log}
}

func (uc *ListStores) Execute(ctx context.Context) ([]domain.Store, error) {
	list, err := uc.stores.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("partners.ListStores: %w", err)
	}
	return list, nil
}
