package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/partners/domain"
)

type GetOffersForProducts struct {
	offers domain.OfferRepository
	stores domain.StoreRepository
	log    *slog.Logger
}

func NewGetOffersForProducts(offers domain.OfferRepository, stores domain.StoreRepository, log *slog.Logger) *GetOffersForProducts {
	return &GetOffersForProducts{offers: offers, stores: stores, log: log}
}

func (uc *GetOffersForProducts) Execute(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]domain.Offer, map[uuid.UUID]domain.Store, error) {
	offerMap, err := uc.offers.ListByProducts(ctx, productIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("partners.GetOffersForProducts: %w", err)
	}

	storeIDSet := make(map[uuid.UUID]struct{})
	for _, offers := range offerMap {
		for _, o := range offers {
			storeIDSet[o.StoreID] = struct{}{}
		}
	}

	storeIDs := make([]uuid.UUID, 0, len(storeIDSet))
	for id := range storeIDSet {
		storeIDs = append(storeIDs, id)
	}

	storeMap := make(map[uuid.UUID]domain.Store, len(storeIDs))
	for _, sid := range storeIDs {
		s, sErr := uc.stores.GetByID(ctx, sid)
		if sErr != nil {
			uc.log.WarnContext(ctx, "could not fetch store", "store_id", sid, "error", sErr)
			continue
		}
		storeMap[sid] = *s
	}

	return offerMap, storeMap, nil
}
