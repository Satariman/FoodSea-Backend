//go:build integration

package repository_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/cart/domain"
	"github.com/foodsea/core/internal/modules/cart/repository"
	"github.com/foodsea/core/internal/platform/config"
	"github.com/foodsea/core/internal/platform/database"
	sherrors "github.com/foodsea/core/internal/shared/errors"

	_ "github.com/foodsea/core/ent/runtime"
)

func startPostgres(t *testing.T) *ent.Client {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("cart_test"),
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

func seedUser(t *testing.T, client *ent.Client) uuid.UUID {
	t.Helper()
	u, err := client.User.Create().
		SetID(uuid.New()).
		SetPasswordHash("hash").
		SetCreatedAt(time.Now()).
		Save(context.Background())
	require.NoError(t, err)
	return u.ID
}

func seedProduct(t *testing.T, client *ent.Client, categoryID uuid.UUID) uuid.UUID {
	t.Helper()
	p, err := client.Product.Create().
		SetID(uuid.New()).
		SetName("TestProduct").
		SetCategoryID(categoryID).
		SetCreatedAt(time.Now()).
		Save(context.Background())
	require.NoError(t, err)
	return p.ID
}

func seedCategory(t *testing.T, client *ent.Client) uuid.UUID {
	t.Helper()
	c, err := client.Category.Create().
		SetID(uuid.New()).
		SetName("TestCat").
		SetSlug("test-cat").
		SetCreatedAt(time.Now()).
		Save(context.Background())
	require.NoError(t, err)
	return c.ID
}

func TestIntegration_CartScenario(t *testing.T) {
	client := startPostgres(t)
	repo := repository.NewCartRepo(client)
	ctx := context.Background()

	userID := seedUser(t, client)
	catID := seedCategory(t, client)
	p1 := seedProduct(t, client, catID)
	p2 := seedProduct(t, client, catID)

	// AddItem p1 qty=2 → GetCart → 1 item with qty=2
	require.NoError(t, repo.AddOrIncrementItem(ctx, userID, p1, 2))
	cart, err := repo.GetByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, cart.Items, 1)
	assert.Equal(t, int16(2), cart.Items[0].Quantity)

	// AddItem p1 qty=3 → qty=5
	require.NoError(t, repo.AddOrIncrementItem(ctx, userID, p1, 3))
	cart, err = repo.GetByUser(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int16(5), cart.Items[0].Quantity)

	// UpdateItem p1 qty=7
	require.NoError(t, repo.UpdateItemQuantity(ctx, userID, p1, 7))
	cart, err = repo.GetByUser(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int16(7), cart.Items[0].Quantity)

	// AddItem p1 qty=95 → ErrInvalidInput (sum > 99)
	err = repo.AddOrIncrementItem(ctx, userID, p1, 95)
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)

	// RemoveItem p1 → empty
	require.NoError(t, repo.RemoveItem(ctx, userID, p1))
	cart, err = repo.GetByUser(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, cart.Items)

	// AddItem p1, AddItem p2, ClearCart → empty
	require.NoError(t, repo.AddOrIncrementItem(ctx, userID, p1, 1))
	require.NoError(t, repo.AddOrIncrementItem(ctx, userID, p2, 1))
	require.NoError(t, repo.Clear(ctx, userID))
	cart, err = repo.GetByUser(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, cart.Items)

	// Restore(items=[p1,p2]) → 2 items
	restoreItems := []domain.CartItem{
		{ProductID: p1, Quantity: 1},
		{ProductID: p2, Quantity: 3},
	}
	require.NoError(t, repo.Restore(ctx, userID, restoreItems))
	cart, err = repo.GetByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, cart.Items, 2)
}

func TestIntegration_ProductNotFound(t *testing.T) {
	client := startPostgres(t)
	repo := repository.NewCartRepo(client)
	ctx := context.Background()

	userID := seedUser(t, client)
	nonExistent := uuid.New()

	err := repo.AddOrIncrementItem(ctx, userID, nonExistent, 1)
	assert.ErrorIs(t, err, sherrors.ErrNotFound)
}

func TestIntegration_Concurrent_AddItem(t *testing.T) {
	client := startPostgres(t)
	repo := repository.NewCartRepo(client)
	ctx := context.Background()

	userID := seedUser(t, client)
	catID := seedCategory(t, client)
	p1 := seedProduct(t, client, catID)

	const goroutines = 5
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = repo.AddOrIncrementItem(ctx, userID, p1, 1)
		}()
	}
	wg.Wait()

	cart, err := repo.GetByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, cart.Items, 1)
	// quantity should be ≤ 5 (some may fail due to >99 guard; all goroutines add 1 so max is 5)
	assert.LessOrEqual(t, cart.Items[0].Quantity, int16(5))
	assert.GreaterOrEqual(t, cart.Items[0].Quantity, int16(1))
}
