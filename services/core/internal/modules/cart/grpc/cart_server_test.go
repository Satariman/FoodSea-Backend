package grpc_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/foodsea/proto/core"

	"github.com/foodsea/core/internal/modules/cart/domain"
	cartgrpc "github.com/foodsea/core/internal/modules/cart/grpc"
	"github.com/foodsea/core/internal/modules/cart/usecase"
)

const bufSize = 1 << 20

// --- mocks ---

type mockCartRepo struct{ mock.Mock }

func (m *mockCartRepo) GetByUser(ctx context.Context, userID uuid.UUID) (*domain.Cart, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(*domain.Cart)
	return v, args.Error(1)
}

func (m *mockCartRepo) AddOrIncrementItem(ctx context.Context, userID, productID uuid.UUID, qty int16) error {
	args := m.Called(ctx, userID, productID, qty)
	return args.Error(0)
}

func (m *mockCartRepo) UpdateItemQuantity(ctx context.Context, userID, productID uuid.UUID, qty int16) error {
	args := m.Called(ctx, userID, productID, qty)
	return args.Error(0)
}

func (m *mockCartRepo) RemoveItem(ctx context.Context, userID, productID uuid.UUID) error {
	args := m.Called(ctx, userID, productID)
	return args.Error(0)
}

func (m *mockCartRepo) Clear(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockCartRepo) Restore(ctx context.Context, userID uuid.UUID, items []domain.CartItem) error {
	args := m.Called(ctx, userID, items)
	return args.Error(0)
}

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) ItemAdded(ctx context.Context, userID, productID uuid.UUID, quantity int16) error {
	return nil
}
func (m *mockPublisher) ItemUpdated(ctx context.Context, userID, productID uuid.UUID, quantity int16) error {
	return nil
}
func (m *mockPublisher) ItemRemoved(ctx context.Context, userID, productID uuid.UUID) error {
	return nil
}
func (m *mockPublisher) Cleared(ctx context.Context, userID uuid.UUID) error {
	return nil
}

// --- helpers ---

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func startServer(t *testing.T, repo *mockCartRepo) pb.CartServiceClient {
	t.Helper()

	log := silentLogger()
	getCartUC := usecase.NewGetCart(repo)
	clearCartUC := usecase.NewClearCart(repo, &mockPublisher{}, log)
	restoreCartUC := usecase.NewRestoreCart(repo, log)

	srv := cartgrpc.NewCartServer(getCartUC, clearCartUC, restoreCartUC, log)

	lis := bufconn.Listen(bufSize)
	gs := googlegrpc.NewServer()
	pb.RegisterCartServiceServer(gs, srv)

	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := googlegrpc.NewClient("passthrough://bufnet",
		googlegrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pb.NewCartServiceClient(conn)
}

// --- tests ---

func TestGetCartItems_EmptyUserID_InvalidArgument(t *testing.T) {
	repo := &mockCartRepo{}
	client := startServer(t, repo)

	_, err := client.GetCartItems(context.Background(), &pb.GetCartItemsRequest{UserId: ""})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetCartItems_Valid(t *testing.T) {
	repo := &mockCartRepo{}
	userID := uuid.New()
	productID := uuid.New()

	repo.On("GetByUser", mock.Anything, userID).Return(&domain.Cart{
		ID:     uuid.New(),
		UserID: userID,
		Items: []domain.CartItem{
			{ID: uuid.New(), ProductID: productID, ProductName: "Milk", Quantity: 2, AddedAt: time.Now()},
		},
	}, nil)

	client := startServer(t, repo)
	resp, err := client.GetCartItems(context.Background(), &pb.GetCartItemsRequest{UserId: userID.String()})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, productID.String(), resp.Items[0].ProductId)
	assert.Equal(t, int32(2), resp.Items[0].Quantity)
}

func TestClearCart_Success(t *testing.T) {
	repo := &mockCartRepo{}
	userID := uuid.New()

	repo.On("Clear", mock.Anything, userID).Return(nil)

	client := startServer(t, repo)
	resp, err := client.ClearCart(context.Background(), &pb.ClearCartRequest{UserId: userID.String()})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestClearCart_EmptyCart_Idempotent(t *testing.T) {
	repo := &mockCartRepo{}
	userID := uuid.New()

	repo.On("Clear", mock.Anything, userID).Return(nil)

	client := startServer(t, repo)
	resp, err := client.ClearCart(context.Background(), &pb.ClearCartRequest{UserId: userID.String()})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestRestoreCart_Success(t *testing.T) {
	repo := &mockCartRepo{}
	userID := uuid.New()
	productID := uuid.New()

	repo.On("Restore", mock.Anything, userID, mock.MatchedBy(func(items []domain.CartItem) bool {
		return len(items) == 1 && items[0].ProductID == productID && items[0].Quantity == 2
	})).Return(nil)

	client := startServer(t, repo)
	resp, err := client.RestoreCart(context.Background(), &pb.RestoreCartRequest{
		UserId: userID.String(),
		Items: []*pb.CartItemProto{
			{ProductId: productID.String(), Quantity: 2},
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}
