//go:build integration

package repository_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/partners/repository"
	"github.com/foodsea/core/internal/platform/config"
	"github.com/foodsea/core/internal/platform/database"
	sherrors "github.com/foodsea/core/internal/shared/errors"

	_ "github.com/foodsea/core/ent/runtime"
)

func startPostgres(t *testing.T) *ent.Client {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("partners_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	dbCfg := config.DatabaseConfig{
		URL:          connStr,
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	client, _, err := database.Open(ctx, dbCfg, slog.Default())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Schema.Create(ctx))
	return client
}

func seedStore(t *testing.T, client *ent.Client, name string, active bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	s, err := client.Store.Create().
		SetID(uuid.New()).
		SetName(name).
		SetSlug(name).
		SetIsActive(active).
		SetCreatedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)
	return s.ID
}

func seedProduct(t *testing.T, client *ent.Client) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	cat, err := client.Category.Create().
		SetID(uuid.New()).
		SetName("TestCat").
		SetSlug("test-cat").
		SetSortOrder(0).
		SetCreatedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)

	p, err := client.Product.Create().
		SetID(uuid.New()).
		SetName("TestProduct").
		SetInStock(true).
		SetCategoryID(cat.ID).
		SetCreatedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)
	return p.ID
}

func seedOffer(t *testing.T, client *ent.Client, productID, storeID uuid.UUID, price int) {
	t.Helper()
	ctx := context.Background()
	_, err := client.Offer.Create().
		SetID(uuid.New()).
		SetProductID(productID).
		SetStoreID(storeID).
		SetPriceKopecks(price).
		SetDiscountPercent(0).
		SetInStock(true).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)
}

func seedDelivery(t *testing.T, client *ent.Client, storeID uuid.UUID, minOrder, cost int) {
	t.Helper()
	ctx := context.Background()
	_, err := client.DeliveryCondition.Create().
		SetID(uuid.New()).
		SetStoreID(storeID).
		SetMinOrderKopecks(minOrder).
		SetDeliveryCostKopecks(cost).
		Save(ctx)
	require.NoError(t, err)
}

func TestStoreRepo_Integration(t *testing.T) {
	client := startPostgres(t)
	ctx := context.Background()
	repo := repository.NewStoreRepo(client)

	t.Run("ListActive returns only active stores", func(t *testing.T) {
		_ = seedStore(t, client, "active-store-"+uuid.NewString()[:6], true)
		_ = seedStore(t, client, "inactive-store-"+uuid.NewString()[:6], false)

		stores, err := repo.ListActive(ctx)
		require.NoError(t, err)
		for _, s := range stores {
			assert.True(t, s.IsActive)
		}
	})

	t.Run("GetByID returns store", func(t *testing.T) {
		id := seedStore(t, client, "byid-store-"+uuid.NewString()[:6], true)
		s, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, s.ID)
	})

	t.Run("GetByID not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, sherrors.ErrNotFound)
	})
}

func TestOfferRepo_Integration(t *testing.T) {
	client := startPostgres(t)
	ctx := context.Background()
	repo := repository.NewOfferRepo(client)

	productID := seedProduct(t, client)
	storeID := seedStore(t, client, "offer-store-"+uuid.NewString()[:6], true)
	seedOffer(t, client, productID, storeID, 12000)

	t.Run("ListByProduct returns offers for product", func(t *testing.T) {
		offers, err := repo.ListByProduct(ctx, productID)
		require.NoError(t, err)
		require.Len(t, offers, 1)
		assert.Equal(t, int64(12000), offers[0].PriceKopecks)
	})

	t.Run("ListByProducts returns map", func(t *testing.T) {
		m, err := repo.ListByProducts(ctx, []uuid.UUID{productID})
		require.NoError(t, err)
		assert.Contains(t, m, productID)
		assert.Len(t, m[productID], 1)
	})

	t.Run("ListByProduct unknown product returns empty", func(t *testing.T) {
		offers, err := repo.ListByProduct(ctx, uuid.New())
		require.NoError(t, err)
		assert.Empty(t, offers)
	})
}

func TestDeliveryRepo_Integration(t *testing.T) {
	client := startPostgres(t)
	ctx := context.Background()
	repo := repository.NewDeliveryRepo(client)

	storeID := seedStore(t, client, "deliv-store-"+uuid.NewString()[:6], true)
	seedDelivery(t, client, storeID, 50000, 9900)

	t.Run("ListByStores returns conditions", func(t *testing.T) {
		m, err := repo.ListByStores(ctx, []uuid.UUID{storeID})
		require.NoError(t, err)
		assert.Contains(t, m, storeID)
		assert.Equal(t, int64(50000), m[storeID].MinOrderKopecks)
	})

	t.Run("GetByStore returns condition", func(t *testing.T) {
		dc, err := repo.GetByStore(ctx, storeID)
		require.NoError(t, err)
		assert.Equal(t, int64(9900), dc.DeliveryCostKopecks)
	})

	t.Run("GetByStore not found", func(t *testing.T) {
		_, err := repo.GetByStore(ctx, uuid.New())
		assert.ErrorIs(t, err, sherrors.ErrNotFound)
	})
}
