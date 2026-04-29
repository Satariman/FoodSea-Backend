package grpc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/foodsea/proto/core"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	cataloggrpc "github.com/foodsea/core/internal/modules/catalog/grpc"
)

type stubProductRepo struct {
	products []domain.ProductMLData
	err      error
}

func (r *stubProductRepo) GetByID(context.Context, uuid.UUID) (*domain.Product, error) {
	panic("unexpected call")
}

func (r *stubProductRepo) GetByIDWithDetails(context.Context, uuid.UUID) (*domain.ProductDetail, error) {
	panic("unexpected call")
}

func (r *stubProductRepo) GetByBarcode(context.Context, string) (*domain.ProductDetail, error) {
	panic("unexpected call")
}

func (r *stubProductRepo) ListAllForML(context.Context) ([]domain.ProductMLData, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.products, nil
}

func (r *stubProductRepo) List(context.Context, domain.ProductFilter) ([]domain.Product, int, error) {
	panic("unexpected call")
}

func TestCatalogServer_ListProductsForML_MapsDomainToProto(t *testing.T) {
	description := "desc"
	composition := "comp"
	weight := "500 мл"
	subcategoryID := uuid.New()
	brandID := uuid.New()
	storeID := uuid.New()

	repo := &stubProductRepo{
		products: []domain.ProductMLData{
			{
				ID:            uuid.New(),
				Name:          "Milk",
				Description:   &description,
				Composition:   &composition,
				CategoryID:    uuid.New(),
				SubcategoryID: &subcategoryID,
				BrandID:       &brandID,
				Weight:        &weight,
				Nutrition: &domain.Nutrition{
					Calories:      52.5,
					Protein:       3.2,
					Fat:           2.5,
					Carbohydrates: 4.8,
				},
				Offers: []domain.OfferBrief{{
					StoreID:      storeID,
					PriceKopecks: 10990,
				}},
			},
		},
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := cataloggrpc.NewCatalogServer(repo, log)

	resp, err := srv.ListProductsForML(context.Background(), &pb.ListProductsForMLRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Products, 1)

	p := resp.Products[0]
	assert.Equal(t, "Milk", p.Name)
	assert.Equal(t, "desc", p.Description)
	assert.Equal(t, "comp", p.Composition)
	assert.Equal(t, "500 мл", p.Weight)
	assert.Equal(t, subcategoryID.String(), p.SubcategoryId)
	assert.Equal(t, brandID.String(), p.BrandId)
	require.Len(t, p.Offers, 1)
	assert.Equal(t, storeID.String(), p.Offers[0].StoreId)
	assert.Equal(t, int64(10990), p.Offers[0].PriceKopecks)
}
