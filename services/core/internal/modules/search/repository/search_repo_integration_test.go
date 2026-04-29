//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/core/ent"
	_ "github.com/foodsea/core/ent/runtime" // initializes ent default functions (uuid, timestamps)
	"github.com/foodsea/core/internal/modules/search/domain"
	"github.com/foodsea/core/internal/modules/search/repository"
	shared "github.com/foodsea/core/internal/shared/domain"

	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver for database/sql
)

// startSearchPostgres starts a postgres:16 container, runs schema migrations,
// creates the FTS GIN index, and returns both ent client and raw *sql.DB.
func startSearchPostgres(t *testing.T) (*ent.Client, *sql.DB) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("search_test"),
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

	// Open a shared *sql.DB via the pgx stdlib driver (same pattern as database.go).
	rawDB, err := sql.Open("pgx", connStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rawDB.Close() })

	// Wrap it in an ent client (shares the same connection pool — no second dial).
	drv := entsql.OpenDB(dialect.Postgres, rawDB)
	client := ent.NewClient(ent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	// Apply all Ent-managed schema.
	require.NoError(t, client.Schema.Create(ctx))

	// Create the FTS GIN index (not expressible via Ent schema).
	_, err = rawDB.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_products_search
		ON products USING GIN (
			to_tsvector('russian',
				name || ' ' || coalesce(description,'') || ' ' || coalesce(composition,'')
			)
		)
	`)
	require.NoError(t, err)

	return client, rawDB
}

type searchSeed struct {
	milkCatID    uuid.UUID
	energyCatID  uuid.UUID
	storeAID     uuid.UUID
	storeBID     uuid.UUID
	milk1ID      uuid.UUID
	milk2ID      uuid.UUID
	energyID     uuid.UUID
	butterID     uuid.UUID
	yogurtID     uuid.UUID
}

func seedSearchData(t *testing.T, client *ent.Client) searchSeed {
	t.Helper()
	ctx := context.Background()

	now := time.Now()

	// Categories
	milkCat, err := client.Category.Create().
		SetName("Молочные продукты").SetSlug("dairy").SetSortOrder(1).SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	energyCat, err := client.Category.Create().
		SetName("Энергетики").SetSlug("energy").SetSortOrder(2).SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	// Stores
	storeA, err := client.Store.Create().
		SetName("Магазин А").SetSlug("store-a").SetLogoURL("https://example.com/a.png").SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	storeB, err := client.Store.Create().
		SetName("Магазин Б").SetSlug("store-b").SetLogoURL("https://example.com/b.png").SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	desc32 := "Молоко пастеризованное 3.2% жирности"
	// Products
	milk1, err := client.Product.Create().
		SetName("Молоко 3.2%").SetDescription(desc32).SetCategoryID(milkCat.ID).SetInStock(true).
		Save(ctx)
	require.NoError(t, err)

	desc25 := "Молоко пастеризованное 2.5% жирности"
	milk2, err := client.Product.Create().
		SetName("Молоко 2.5%").SetDescription(desc25).SetCategoryID(milkCat.ID).SetInStock(true).
		Save(ctx)
	require.NoError(t, err)

	energy, err := client.Product.Create().
		SetName("Энергетик Burn Classic").SetCategoryID(energyCat.ID).SetInStock(true).
		Save(ctx)
	require.NoError(t, err)

	descButter := "Масло сливочное традиционное"
	butter, err := client.Product.Create().
		SetName("Масло сливочное").SetDescription(descButter).SetCategoryID(milkCat.ID).SetInStock(true).
		Save(ctx)
	require.NoError(t, err)

	yogurt, err := client.Product.Create().
		SetName("Йогурт персиковый").SetCategoryID(milkCat.ID).SetInStock(false).
		Save(ctx)
	require.NoError(t, err)

	// Offers: milk1 in both stores, milk2 in storeA only, butter in storeB.
	_, err = client.Offer.Create().
		SetProductID(milk1.ID).SetStoreID(storeA.ID).SetPriceKopecks(9000).SetInStock(true).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Offer.Create().
		SetProductID(milk1.ID).SetStoreID(storeB.ID).SetPriceKopecks(9500).SetInStock(true).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Offer.Create().
		SetProductID(milk2.ID).SetStoreID(storeA.ID).SetPriceKopecks(7500).SetInStock(true).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Offer.Create().
		SetProductID(butter.ID).SetStoreID(storeB.ID).SetPriceKopecks(15000).SetInStock(true).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Offer.Create().
		SetProductID(energy.ID).SetStoreID(storeA.ID).SetPriceKopecks(12000).SetInStock(true).
		Save(ctx)
	require.NoError(t, err)

	return searchSeed{
		milkCatID:   milkCat.ID,
		energyCatID: energyCat.ID,
		storeAID:    storeA.ID,
		storeBID:    storeB.ID,
		milk1ID:     milk1.ID,
		milk2ID:     milk2.ID,
		energyID:    energy.ID,
		butterID:    butter.ID,
		yogurtID:    yogurt.ID,
	}
}

func ptr[T any](v T) *T { return &v }

func TestSearchRepo_MilkQuery_FindsBothMilks(t *testing.T) {
	client, db := startSearchPostgres(t)
	seed := seedSearchData(t, client)
	repo := repository.NewSearchRepo(db)

	result, err := repo.Search(context.Background(), domain.SearchQuery{
		Text:       "молоко",
		Sort:       domain.SortRelevance,
		Pagination: shared.NewPagination(1, 20),
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, 2, "should find at least 2 milk products")

	var names []string
	for _, item := range result.Items {
		names = append(names, item.Product.Name)
	}
	assert.Contains(t, names, "Молоко 3.2%")
	assert.Contains(t, names, "Молоко 2.5%")

	// yogurt is out_of_stock=false, should still appear (in_stock_only not set)
	_ = seed.yogurtID
}

func TestSearchRepo_EnergyQuery_FindsEnergyDrink(t *testing.T) {
	client, db := startSearchPostgres(t)
	seedSearchData(t, client)
	repo := repository.NewSearchRepo(db)

	result, err := repo.Search(context.Background(), domain.SearchQuery{
		Text:       "энергетик",
		Sort:       domain.SortRelevance,
		Pagination: shared.NewPagination(1, 20),
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Total)
	assert.Equal(t, "Энергетик Burn Classic", result.Items[0].Product.Name)
}

func TestSearchRepo_MinPrice_FiltersResults(t *testing.T) {
	client, db := startSearchPostgres(t)
	seedSearchData(t, client)
	repo := repository.NewSearchRepo(db)

	// butter costs 150 ₽ (15000 kopecks); milk costs ~90 ₽
	result, err := repo.Search(context.Background(), domain.SearchQuery{
		Text:            "масло",
		MinPriceKopecks: ptr(int64(10000)),
		Sort:            domain.SortRelevance,
		Pagination:      shared.NewPagination(1, 20),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "Масло сливочное", result.Items[0].Product.Name)
	assert.GreaterOrEqual(t, result.Items[0].MinPriceKopecks, int64(10000))
}

func TestSearchRepo_SortPriceAsc_OrderedByPrice(t *testing.T) {
	client, db := startSearchPostgres(t)
	seedSearchData(t, client)
	repo := repository.NewSearchRepo(db)

	result, err := repo.Search(context.Background(), domain.SearchQuery{
		Text:       "молоко",
		Sort:       domain.SortPriceAsc,
		Pagination: shared.NewPagination(1, 20),
	})

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(result.Items), 2)
	for i := 1; i < len(result.Items); i++ {
		assert.LessOrEqual(t, result.Items[i-1].MinPriceKopecks, result.Items[i].MinPriceKopecks,
			"items should be sorted by price ascending")
	}
}

func TestSearchRepo_SortNameAsc_OrderedAlphabetically(t *testing.T) {
	client, db := startSearchPostgres(t)
	seedSearchData(t, client)
	repo := repository.NewSearchRepo(db)

	result, err := repo.Search(context.Background(), domain.SearchQuery{
		Text:       "молоко",
		Sort:       domain.SortNameAsc,
		Pagination: shared.NewPagination(1, 20),
	})

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(result.Items), 2)
	for i := 1; i < len(result.Items); i++ {
		assert.LessOrEqual(t, result.Items[i-1].Product.Name, result.Items[i].Product.Name,
			"items should be sorted by name ascending")
	}
}

func TestSearchRepo_CategoryFilter_ExcludesOtherCategories(t *testing.T) {
	client, db := startSearchPostgres(t)
	seed := seedSearchData(t, client)
	repo := repository.NewSearchRepo(db)

	// Search for "молоко" but filter by energy category — should return empty.
	result, err := repo.Search(context.Background(), domain.SearchQuery{
		Text:       "молоко",
		CategoryID: &seed.energyCatID,
		Sort:       domain.SortRelevance,
		Pagination: shared.NewPagination(1, 20),
	})

	require.NoError(t, err)
	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.Items)
}

func TestSearchRepo_InStockOnly_ExcludesOutOfStock(t *testing.T) {
	client, db := startSearchPostgres(t)
	seedSearchData(t, client)
	repo := repository.NewSearchRepo(db)

	// yogurt is in_stock=false. Search milk-related terms with in_stock=true.
	result, err := repo.Search(context.Background(), domain.SearchQuery{
		Text:        "йогурт",
		InStockOnly: true,
		Sort:        domain.SortRelevance,
		Pagination:  shared.NewPagination(1, 20),
	})

	require.NoError(t, err)
	for _, item := range result.Items {
		assert.True(t, item.Product.InStock, "in_stock_only should exclude out-of-stock products")
	}
}

func TestSearchRepo_StoreFilter_LimitsMinPriceToStore(t *testing.T) {
	client, db := startSearchPostgres(t)
	seed := seedSearchData(t, client)
	repo := repository.NewSearchRepo(db)

	// milk1 has offers in storeA (9000) and storeB (9500).
	// When filtering by storeA, min_price should be 9000.
	resultA, err := repo.Search(context.Background(), domain.SearchQuery{
		Text:       "молоко",
		StoreID:    &seed.storeAID,
		Sort:       domain.SortRelevance,
		Pagination: shared.NewPagination(1, 20),
	})
	require.NoError(t, err)

	for _, item := range resultA.Items {
		if item.Product.ID == seed.milk1ID {
			assert.Equal(t, int64(9000), item.MinPriceKopecks,
				"min_price for milk1 in storeA should be 9000 kopecks")
		}
	}
}

func TestSearchRepo_EmptyResults_NoError(t *testing.T) {
	client, db := startSearchPostgres(t)
	seedSearchData(t, client)
	repo := repository.NewSearchRepo(db)

	result, err := repo.Search(context.Background(), domain.SearchQuery{
		Text:       "несуществующийтовар",
		Sort:       domain.SortRelevance,
		Pagination: shared.NewPagination(1, 20),
	})

	require.NoError(t, err)
	assert.Equal(t, 0, result.Total)
	assert.NotNil(t, result.Items)
	assert.Empty(t, result.Items)
}
