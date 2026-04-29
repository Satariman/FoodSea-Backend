package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/partners/domain"
)

type ListOffersByProduct struct {
	offers domain.OfferRepository
	stores domain.StoreRepository
	cache  domain.OfferCache
	log    *slog.Logger
}

func NewListOffersByProduct(
	offers domain.OfferRepository,
	stores domain.StoreRepository,
	cache domain.OfferCache,
	log *slog.Logger,
) *ListOffersByProduct {
	return &ListOffersByProduct{offers: offers, stores: stores, cache: cache, log: log}
}

func (uc *ListOffersByProduct) Execute(ctx context.Context, productID uuid.UUID, hasDiscountOnly bool) ([]domain.OfferWithStore, error) {
	cached, err := uc.cache.GetOffersByProduct(ctx, productID)
	if err != nil {
		uc.log.WarnContext(ctx, "offer cache get failed", "error", err)
	}

	var raw []domain.Offer
	if cached != nil {
		raw = cached
	} else {
		raw, err = uc.offers.ListByProduct(ctx, productID)
		if err != nil {
			return nil, fmt.Errorf("partners.ListOffersByProduct: %w", err)
		}
		if setErr := uc.cache.SetOffersByProduct(ctx, productID, raw); setErr != nil {
			uc.log.WarnContext(ctx, "offer cache set failed", "error", setErr)
		}
	}

	if hasDiscountOnly {
		filtered := raw[:0]
		for _, o := range raw {
			if o.HasDiscount() {
				filtered = append(filtered, o)
			}
		}
		raw = filtered
	}

	storeIDs := make([]uuid.UUID, 0, len(raw))
	seen := make(map[uuid.UUID]struct{}, len(raw))
	for _, o := range raw {
		if _, ok := seen[o.StoreID]; !ok {
			storeIDs = append(storeIDs, o.StoreID)
			seen[o.StoreID] = struct{}{}
		}
	}

	storeMap := make(map[uuid.UUID]domain.Store, len(storeIDs))
	for _, sid := range storeIDs {
		s, sErr := uc.stores.GetByID(ctx, sid)
		if sErr != nil {
			uc.log.WarnContext(ctx, "could not fetch store for offer", "store_id", sid, "error", sErr)
			continue
		}
		storeMap[sid] = *s
	}

	result := make([]domain.OfferWithStore, 0, len(raw))
	for _, o := range raw {
		s, ok := storeMap[o.StoreID]
		if !ok {
			continue
		}
		result = append(result, domain.OfferWithStore{Offer: o, Store: s})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].PriceKopecks < result[j].PriceKopecks
	})

	return result, nil
}
