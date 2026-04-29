//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/ordering/ent"
	_ "github.com/foodsea/ordering/ent/runtime"
	"github.com/foodsea/ordering/internal/modules/saga/domain"
	"github.com/foodsea/ordering/internal/modules/saga/repository"
	"github.com/foodsea/ordering/internal/platform/config"
	"github.com/foodsea/ordering/internal/platform/database"
)

var testEntClient *ent.Client

func TestMain(m *testing.M) {
	ctx := context.Background()
	os.Exit(runIntegration(ctx, m))
}

func runIntegration(ctx context.Context, m *testing.M) int {
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("saga_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres: %v\n", err)
		return 1
	}
	defer pg.Terminate(ctx) //nolint:errcheck

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres DSN: %v\n", err)
		return 1
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbCfg := config.DatabaseConfig{URL: dsn}
	testEntClient, _, err = database.Open(ctx, dbCfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		return 1
	}
	defer testEntClient.Close()
	if err := testEntClient.Schema.Create(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "create schema: %v\n", err)
		return 1
	}

	return m.Run()
}

func TestSagaRepo_CreateAndGetByID(t *testing.T) {
	repo := repository.NewSagaRepo(testEntClient)
	ctx := context.Background()

	userID, resultID := uuid.New(), uuid.New()
	s := &domain.SagaState{
		UserID: userID,
		Status: domain.SagaStatusPending,
		Payload: domain.SagaPayload{
			OptimizationResultID: resultID,
			TotalKopecks:         1000,
			DeliveryKopecks:      100,
			Items: []domain.OrderItemSnapshot{
				{ProductID: uuid.New(), StoreID: uuid.New(), Quantity: 1, PriceKopecks: 1000},
			},
		},
	}

	require.NoError(t, repo.Create(ctx, s))
	assert.NotEqual(t, uuid.Nil, s.ID)

	got, err := repo.GetByID(ctx, s.ID)
	require.NoError(t, err)
	assert.Equal(t, s.ID, got.ID)
	assert.Equal(t, domain.SagaStatusPending, got.Status)
	assert.EqualValues(t, 0, got.CurrentStep)
	assert.Equal(t, resultID, got.Payload.OptimizationResultID)
}

func TestSagaRepo_UpdateState_PersistsStepAndPayload(t *testing.T) {
	repo := repository.NewSagaRepo(testEntClient)
	ctx := context.Background()

	orderID := uuid.New()
	s := &domain.SagaState{
		UserID: uuid.New(),
		Status: domain.SagaStatusPending,
		Payload: domain.SagaPayload{
			OptimizationResultID: uuid.New(),
			TotalKopecks:         500,
		},
	}
	require.NoError(t, repo.Create(ctx, s))

	err := repo.UpdateState(ctx, s.ID, 2, domain.SagaStatusPending, func(p *domain.SagaPayload) {
		p.OrderID = &orderID
	})
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, s.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 2, got.CurrentStep)
	require.NotNil(t, got.Payload.OrderID)
	assert.Equal(t, orderID, *got.Payload.OrderID)
}

func TestSagaRepo_GetByOrderID(t *testing.T) {
	repo := repository.NewSagaRepo(testEntClient)
	ctx := context.Background()

	orderID := uuid.New()
	s := &domain.SagaState{
		UserID: uuid.New(),
		Status: domain.SagaStatusPending,
		Payload: domain.SagaPayload{
			OptimizationResultID: uuid.New(),
		},
	}
	require.NoError(t, repo.Create(ctx, s))
	require.NoError(t, repo.UpdateState(ctx, s.ID, 2, domain.SagaStatusPending, func(p *domain.SagaPayload) {
		p.OrderID = &orderID
	}))

	got, err := repo.GetByOrderID(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, s.ID, got.ID)
}

func TestSagaRepo_ListPending_ReturnsOnlyPendingAndCompensating(t *testing.T) {
	repo := repository.NewSagaRepo(testEntClient)
	ctx := context.Background()

	// Create two sagas: one pending, one completed (should not appear)
	pendingSaga := &domain.SagaState{
		UserID: uuid.New(), Status: domain.SagaStatusPending,
		Payload: domain.SagaPayload{OptimizationResultID: uuid.New()},
	}
	require.NoError(t, repo.Create(ctx, pendingSaga))

	// mark as completed
	completedSaga := &domain.SagaState{
		UserID: uuid.New(), Status: domain.SagaStatusPending,
		Payload: domain.SagaPayload{OptimizationResultID: uuid.New()},
	}
	require.NoError(t, repo.Create(ctx, completedSaga))
	require.NoError(t, repo.UpdateState(ctx, completedSaga.ID, 4, domain.SagaStatusCompleted, nil))

	pending, err := repo.ListPending(ctx)
	require.NoError(t, err)

	var ids []uuid.UUID
	for _, p := range pending {
		ids = append(ids, p.ID)
	}
	assert.Contains(t, ids, pendingSaga.ID)
	assert.NotContains(t, ids, completedSaga.ID)
}

func TestSagaRepo_GetByID_NotFound(t *testing.T) {
	repo := repository.NewSagaRepo(testEntClient)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
}
