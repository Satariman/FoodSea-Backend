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
	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/catalog/repository"
	"github.com/foodsea/core/internal/platform/config"
	"github.com/foodsea/core/internal/platform/database"
	shared "github.com/foodsea/core/internal/shared/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"

	_ "github.com/foodsea/core/ent/runtime"
)

func startPostgres(t *testing.T) *ent.Client {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("catalog_test"),
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

func seedCatalog(t *testing.T, client *ent.Client) (categoryID, subcategoryID, brandID, productID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()

	root, err := client.Category.Create().
		SetName("Энергетики").
		SetSlug("energetiki").
		SetSortOrder(1).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	sub, err := client.Category.Create().
		SetName("Энергетические напитки").
		SetSlug("energeticheskie-napitki").
		SetSortOrder(1).
		SetParentID(root.ID).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	brand, err := client.Brand.Create().
		SetName("Burn").
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	product, err := client.Product.Create().
		SetName("Энергетик Burn Classic, 500 мл").
		SetBarcode("4607025390015").
		SetInStock(true).
		SetCategoryID(root.ID).
		SetSubcategoryID(sub.ID).
		SetBrandID(brand.ID).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.ProductNutrition.Create().
		SetProductID(product.ID).
		SetCalories(46).
		SetProtein(0).
		SetFat(0).
		SetCarbohydrates(11.5).
		Save(ctx)
	require.NoError(t, err)

	return root.ID, sub.ID, brand.ID, product.ID
}

func TestProductRepo_Integration(t *testing.T) {
	client := startPostgres(t)
	ctx := context.Background()

	catID, subID, brandID, productID := seedCatalog(t, client)
	productRepo := repository.NewProductRepo(client)

	t.Run("GetByIDWithDetails returns all edges", func(t *testing.T) {
		detail, err := productRepo.GetByIDWithDetails(ctx, productID)
		require.NoError(t, err)

		assert.Equal(t, productID, detail.ID)
		assert.Equal(t, "Энергетик Burn Classic, 500 мл", detail.Name)
		assert.Equal(t, catID, detail.Category.ID)
		require.NotNil(t, detail.Subcategory)
		assert.Equal(t, subID, detail.Subcategory.ID)
		require.NotNil(t, detail.Brand)
		assert.Equal(t, brandID, detail.Brand.ID)
		require.NotNil(t, detail.Nutrition)
		assert.Equal(t, 46.0, detail.Nutrition.Calories)
		assert.Equal(t, 11.5, detail.Nutrition.Carbohydrates)
	})

	t.Run("GetByBarcode returns product", func(t *testing.T) {
		detail, err := productRepo.GetByBarcode(ctx, "4607025390015")
		require.NoError(t, err)
		assert.Equal(t, productID, detail.ID)
	})

	t.Run("GetByIDWithDetails not found", func(t *testing.T) {
		_, err := productRepo.GetByIDWithDetails(ctx, uuid.New())
		assert.ErrorIs(t, err, sherrors.ErrNotFound)
	})

	t.Run("List by CategoryID returns 1 product total=1", func(t *testing.T) {
		items, total, err := productRepo.List(ctx, domain.ProductFilter{
			CategoryID: &catID,
			Pagination: shared.NewPagination(1, 20),
		})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, items, 1)
	})

	t.Run("List by BrandID returns 1 product", func(t *testing.T) {
		items, total, err := productRepo.List(ctx, domain.ProductFilter{
			BrandID:    &brandID,
			Pagination: shared.NewPagination(1, 20),
		})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, items, 1)
	})

	t.Run("List empty filter returns all products", func(t *testing.T) {
		_, total, err := productRepo.List(ctx, domain.ProductFilter{
			Pagination: shared.NewPagination(1, 100),
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 1)
	})
}

func TestCategoryRepo_Integration(t *testing.T) {
	client := startPostgres(t)
	ctx := context.Background()

	seedCatalog(t, client)

	catRepo := repository.NewCategoryRepo(client)

	t.Run("ListAll returns root and sub", func(t *testing.T) {
		cats, err := catRepo.ListAll(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(cats), 2)
	})

	t.Run("GetBySlug returns category", func(t *testing.T) {
		cat, err := catRepo.GetBySlug(ctx, "energetiki")
		require.NoError(t, err)
		assert.Equal(t, "Энергетики", cat.Name)
	})

	t.Run("GetBySlug not found", func(t *testing.T) {
		_, err := catRepo.GetBySlug(ctx, "nonexistent")
		assert.ErrorIs(t, err, sherrors.ErrNotFound)
	})
}
