//go:build integration

package repository

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/foodsea/optimization/ent/runtime"
	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
	"github.com/foodsea/optimization/internal/platform/config"
	"github.com/foodsea/optimization/internal/platform/database"
)

func TestResultRepo_Integration(t *testing.T) {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("optimization_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	conn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	client, _, err := database.Open(ctx, config.DatabaseConfig{URL: conn, MaxOpenConns: 5, MaxIdleConns: 2}, log)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Schema.Create(ctx))

	repo := NewResultRepo(client, log)
	userID := uuid.New()
	productID := uuid.New()
	storeID := uuid.New()
	resultID := uuid.New()

	err = repo.Save(ctx, &domain.OptimizationResult{
		ID:              resultID,
		UserID:          userID,
		CartHash:        "hash-1",
		TotalKopecks:    1200,
		DeliveryKopecks: 100,
		SavingsKopecks:  40,
		Status:          "active",
		Items: []domain.Assignment{{
			ProductID:   productID,
			ProductName: "Milk",
			StoreID:     storeID,
			StoreName:   "Store",
			Price:       1100,
			Quantity:    1,
		}},
		Substitutions: []domain.Substitution{{
			OriginalID:           productID,
			OriginalProductName:  "Milk",
			AnalogID:             uuid.New(),
			AnalogProductName:    "Alt",
			OriginalStoreID:      storeID,
			NewStoreID:           storeID,
			NewStoreName:         "Store",
			OldPriceKopecks:      1100,
			NewPriceKopecks:      1000,
			PriceDeltaKopecks:    -100,
			DeliveryDeltaKopecks: 0,
			TotalSavingKopecks:   100,
			Score:                0.95,
			IsCrossStore:         false,
		}},
	})
	require.NoError(t, err)

	stored, err := repo.GetByID(ctx, resultID)
	require.NoError(t, err)
	require.Equal(t, "hash-1", stored.CartHash)
	require.Len(t, stored.Items, 1)
	require.Len(t, stored.Substitutions, 1)

	hit, err := repo.FindByCartHash(ctx, "hash-1")
	require.NoError(t, err)
	require.NotNil(t, hit)
	require.Equal(t, resultID, hit.ID)

	require.NoError(t, repo.Lock(ctx, resultID))
	require.ErrorIs(t, repo.Lock(ctx, resultID), domain.ErrResultLocked)
	require.NoError(t, repo.Unlock(ctx, resultID))
	require.ErrorIs(t, repo.Unlock(ctx, resultID), domain.ErrResultNotLocked)

	oldID := uuid.New()
	oldTime := time.Now().Add(-2 * time.Hour)
	_, err = client.OptimizationResult.Create().
		SetID(oldID).
		SetUserID(userID).
		SetCartHash("old-hash").
		SetTotalKopecks(500).
		SetDeliveryKopecks(50).
		SetSavingsKopecks(0).
		SetStatus("active").
		SetCreatedAt(oldTime).
		SetUpdatedAt(oldTime).
		Save(ctx)
	require.NoError(t, err)

	expired, err := repo.ExpireOld(ctx, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.GreaterOrEqual(t, expired, 1)

	require.NoError(t, repo.DeleteByUserCartHash(ctx, userID, "hash-1"))
	_, err = repo.GetByID(ctx, resultID)
	require.ErrorIs(t, err, domain.ErrResultNotFound)
}
