//go:build integration

package database_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/foodsea/core/ent/runtime"
	"github.com/foodsea/core/internal/platform/config"
	"github.com/foodsea/core/internal/platform/database"
)

func TestOpen_Integration(t *testing.T) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("core_test"),
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

	// Apply schema
	require.NoError(t, client.Schema.Create(ctx))

	// Verify basic operations
	brand, err := client.Brand.Create().
		SetName("TestBrand").
		SetCreatedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)
	assert.Equal(t, "TestBrand", brand.Name)
}
