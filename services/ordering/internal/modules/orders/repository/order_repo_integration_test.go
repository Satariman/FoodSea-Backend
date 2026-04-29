//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/ordering/ent"
	_ "github.com/foodsea/ordering/ent/runtime"
	"github.com/foodsea/ordering/internal/modules/orders/domain"
	"github.com/foodsea/ordering/internal/modules/orders/repository"
	"github.com/foodsea/ordering/internal/platform/database"
	shared "github.com/foodsea/ordering/internal/shared/domain"
	sherrors "github.com/foodsea/ordering/internal/shared/errors"

	"github.com/foodsea/ordering/internal/platform/config"
)

var (
	testEntClient *ent.Client
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	os.Exit(runIntegration(ctx, m))
}

func runIntegration(ctx context.Context, m *testing.M) int {
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("ordering_test"),
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
	dbCfg := config.DatabaseConfig{
		URL:             dsn,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}

	entClient, _, err := database.Open(ctx, dbCfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open DB: %v\n", err)
		return 1
	}
	defer entClient.Close()

	if err := entClient.Schema.Create(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "create schema: %v\n", err)
		return 1
	}

	testEntClient = entClient
	return m.Run()
}

func makeRepo() *repository.OrderRepository {
	return repository.NewOrderRepository(testEntClient, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func buildOrder(userID uuid.UUID, nItems int) *domain.Order {
	items := make([]domain.OrderItem, nItems)
	for i := 0; i < nItems; i++ {
		items[i] = domain.OrderItem{
			ProductID:    uuid.New(),
			ProductName:  fmt.Sprintf("Product-%d", i),
			StoreID:      uuid.New(),
			StoreName:    fmt.Sprintf("Store-%d", i),
			Quantity:     int16(i + 1),
			PriceKopecks: int64((i + 1) * 100),
		}
	}
	total := int64(0)
	for _, it := range items {
		total += int64(it.Quantity) * it.PriceKopecks
	}
	return &domain.Order{
		UserID:          userID,
		TotalKopecks:    total,
		DeliveryKopecks: 200,
		Items:           items,
	}
}

// ─── CreatePending ────────────────────────────────────────────────────────────

func TestCreatePending_TransactionallyCreatesOrderItemsHistory(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	userID := uuid.New()
	order := buildOrder(userID, 3)

	err := repo.CreatePending(ctx, order)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, order.ID)

	got, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, shared.StatusCreated, got.Status)
	assert.Len(t, got.Items, 3)
	require.Len(t, got.History, 1)
	assert.Equal(t, shared.StatusCreated, got.History[0].Status)
}

func TestCreatePending_ItemsHaveCorrectSnapshot(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	userID := uuid.New()
	order := buildOrder(userID, 1)
	order.Items[0].ProductName = "Special Milk"
	order.Items[0].PriceKopecks = 9999

	require.NoError(t, repo.CreatePending(ctx, order))

	got, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.Equal(t, "Special Milk", got.Items[0].ProductName)
	assert.Equal(t, int64(9999), got.Items[0].PriceKopecks)
}

func TestCreatePending_BulkInsert_100Items(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	order := buildOrder(uuid.New(), 100)
	require.NoError(t, repo.CreatePending(ctx, order))

	got, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Len(t, got.Items, 100)
}

// ─── GetByIDForUser ───────────────────────────────────────────────────────────

func TestGetByIDForUser_OtherUser_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	owner := uuid.New()
	order := buildOrder(owner, 1)
	require.NoError(t, repo.CreatePending(ctx, order))

	otherUser := uuid.New()
	_, err := repo.GetByIDForUser(ctx, order.ID, otherUser)
	assert.ErrorIs(t, err, sherrors.ErrNotFound)
}

func TestGetByIDForUser_Owner_ReturnsOrder(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	userID := uuid.New()
	order := buildOrder(userID, 2)
	require.NoError(t, repo.CreatePending(ctx, order))

	got, err := repo.GetByIDForUser(ctx, order.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, got.ID)
}

// ─── ListByUser ───────────────────────────────────────────────────────────────

func TestListByUser_EmptyUser_ReturnsZero(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	orders, total, err := repo.ListByUser(ctx, uuid.New(), shared.NewPagination(1, 20))
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, orders)
}

func TestListByUser_PaginationWorks(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	userID := uuid.New()
	for i := 0; i < 5; i++ {
		o := buildOrder(userID, 1)
		require.NoError(t, repo.CreatePending(ctx, o))
	}

	page1, total, err := repo.ListByUser(ctx, userID, shared.NewPagination(1, 3))
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, page1, 3)

	page2, total2, err := repo.ListByUser(ctx, userID, shared.NewPagination(2, 3))
	require.NoError(t, err)
	assert.Equal(t, 5, total2)
	assert.Len(t, page2, 2)
}

func TestListByUser_SortedByCreatedAtDesc(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	userID := uuid.New()
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		o := buildOrder(userID, 1)
		require.NoError(t, repo.CreatePending(ctx, o))
		ids = append(ids, o.ID)
		time.Sleep(5 * time.Millisecond)
	}

	orders, _, err := repo.ListByUser(ctx, userID, shared.NewPagination(1, 10))
	require.NoError(t, err)
	require.Len(t, orders, 3)
	// DESC: newest first
	assert.Equal(t, ids[2], orders[0].ID)
	assert.Equal(t, ids[0], orders[2].ID)
}

// ─── TransitionStatus ─────────────────────────────────────────────────────────

func TestTransitionStatus_ValidTransition_Succeeds(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	order := buildOrder(uuid.New(), 1)
	require.NoError(t, repo.CreatePending(ctx, order))

	require.NoError(t, repo.TransitionStatus(ctx, order.ID, shared.StatusConfirmed, nil))

	got, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, shared.StatusConfirmed, got.Status)
}

func TestTransitionStatus_InvalidTransition_ReturnsConflict(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	order := buildOrder(uuid.New(), 1)
	require.NoError(t, repo.CreatePending(ctx, order))
	// created → in_delivery is not a valid FSM transition
	err := repo.TransitionStatus(ctx, order.ID, shared.StatusInDelivery, nil)
	assert.ErrorIs(t, err, sherrors.ErrConflict)
}

func TestTransitionStatus_HistoryAccumulates(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	order := buildOrder(uuid.New(), 1)
	require.NoError(t, repo.CreatePending(ctx, order))

	chain := []shared.OrderStatus{shared.StatusConfirmed, shared.StatusInDelivery, shared.StatusDelivered}
	for _, s := range chain {
		require.NoError(t, repo.TransitionStatus(ctx, order.ID, s, nil))
	}

	got, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	// created + confirmed + in_delivery + delivered = 4 history entries
	assert.Len(t, got.History, 4)
	// History is returned DESC by changed_at, so first is the latest
	assert.Equal(t, shared.StatusDelivered, got.History[0].Status)
}

func TestTransitionStatus_Concurrent10Goroutines_OnlyOneSucceeds(t *testing.T) {
	ctx := context.Background()
	repo := makeRepo()

	order := buildOrder(uuid.New(), 1)
	require.NoError(t, repo.CreatePending(ctx, order))

	var (
		wg       sync.WaitGroup
		okCount  atomic.Int64
		errCount atomic.Int64
	)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := repo.TransitionStatus(ctx, order.ID, shared.StatusConfirmed, nil)
			if err == nil {
				okCount.Add(1)
			} else {
				errCount.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(1), okCount.Load(), "exactly one goroutine should succeed")
	assert.Equal(t, int64(9), errCount.Load(), "remaining 9 should get conflict")

	got, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, shared.StatusConfirmed, got.Status)
}
