//go:build integration

package database_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	_ "github.com/foodsea/optimization/ent/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/optimization/internal/platform/config"
	"github.com/foodsea/optimization/internal/platform/database"
)

func TestOpen_Integration_CRUDOptimizationEntities(t *testing.T) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("optimization_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	cfg := config.DatabaseConfig{
		URL:          connStr,
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	client, _, err := database.Open(ctx, cfg, slog.Default())
	require.NoError(t, err)
	defer client.Close()

	require.NoError(t, client.Schema.Create(ctx))

	result, err := client.OptimizationResult.Create().
		SetUserID(uuid.New()).
		SetCartHash("hash-v1").
		SetTotalKopecks(10000).
		SetDeliveryKopecks(500).
		SetSavingsKopecks(1200).
		SetStatus("active").
		SetIsApproximate(true).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.OptimizationItem.Create().
		SetResultID(result.ID).
		SetProductID(uuid.New()).
		SetProductName("Milk").
		SetStoreID(uuid.New()).
		SetStoreName("Store A").
		SetQuantity(2).
		SetPriceKopecks(6400).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Substitution.Create().
		SetResultID(result.ID).
		SetOriginalProductID(uuid.New()).
		SetOriginalProductName("Milk").
		SetAnalogProductID(uuid.New()).
		SetAnalogProductName("Milk Analog").
		SetOriginalStoreID(uuid.New()).
		SetNewStoreID(uuid.New()).
		SetNewStoreName("Store B").
		SetOldPriceKopecks(3500).
		SetNewPriceKopecks(3100).
		SetPriceDeltaKopecks(-400).
		SetDeliveryDeltaKopecks(0).
		SetTotalSavingKopecks(400).
		SetScore(0.875).
		SetIsCrossStore(true).
		Save(ctx)
	require.NoError(t, err)

	stored, err := client.OptimizationResult.Query().
		WithItems().
		WithSubstitutions().
		Only(ctx)
	require.NoError(t, err)

	assert.Equal(t, "hash-v1", stored.CartHash)
	assert.True(t, stored.IsApproximate)
	require.Len(t, stored.Edges.Items, 1)
	require.Len(t, stored.Edges.Substitutions, 1)
	assert.Equal(t, int16(2), stored.Edges.Items[0].Quantity)
	assert.Equal(t, int64(400), stored.Edges.Substitutions[0].TotalSavingKopecks)
}
