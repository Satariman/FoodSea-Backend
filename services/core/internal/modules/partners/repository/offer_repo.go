package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	entoffer "github.com/foodsea/core/ent/offer"
	entproduct "github.com/foodsea/core/ent/product"
	"github.com/foodsea/core/internal/modules/partners/domain"
)

type OfferRepo struct {
	client *ent.Client
}

func NewOfferRepo(client *ent.Client) *OfferRepo {
	return &OfferRepo{client: client}
}

func (r *OfferRepo) ListByProduct(ctx context.Context, productID uuid.UUID) ([]domain.Offer, error) {
	rows, err := r.client.Offer.Query().
		Where(entoffer.HasProductWith(entproduct.ID(productID))).
		WithStore().
		Order(ent.Asc(entoffer.FieldPriceKopecks)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing offers by product: %w", err)
	}
	result := make([]domain.Offer, len(rows))
	for i, row := range rows {
		result[i] = toDomainOffer(row)
	}
	return result, nil
}

func (r *OfferRepo) ListByProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]domain.Offer, error) {
	rows, err := r.client.Offer.Query().
		Where(entoffer.ProductIDIn(productIDs...)).
		WithStore().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing offers by products: %w", err)
	}

	result := make(map[uuid.UUID][]domain.Offer, len(productIDs))
	for _, row := range rows {
		o := toDomainOffer(row)
		result[o.ProductID] = append(result[o.ProductID], o)
	}
	return result, nil
}

func toDomainOffer(e *ent.Offer) domain.Offer {
	o := domain.Offer{
		ID:              e.ID,
		ProductID:       e.ProductID,
		StoreID:         e.StoreID,
		PriceKopecks:    int64(e.PriceKopecks),
		DiscountPercent: e.DiscountPercent,
		InStock:         e.InStock,
		UpdatedAt:       e.UpdatedAt,
	}
	if e.OriginalPriceKopecks != nil {
		v := int64(*e.OriginalPriceKopecks)
		o.OriginalPriceKopecks = &v
	}
	return o
}
