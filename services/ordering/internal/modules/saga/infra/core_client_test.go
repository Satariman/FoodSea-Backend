package infra_test

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb_core "github.com/foodsea/proto/core"

	"github.com/foodsea/ordering/internal/modules/saga/domain"
	"github.com/foodsea/ordering/internal/modules/saga/infra"
)

// ─── fake CartService server ──────────────────────────────────────────────────

type fakeCartServer struct {
	pb_core.UnimplementedCartServiceServer
	clearCartErr   error
	restoreCartErr error
}

func (f *fakeCartServer) ClearCart(_ context.Context, _ *pb_core.ClearCartRequest) (*pb_core.ClearCartResponse, error) {
	if f.clearCartErr != nil {
		return nil, f.clearCartErr
	}
	return &pb_core.ClearCartResponse{Success: true}, nil
}

func (f *fakeCartServer) RestoreCart(_ context.Context, _ *pb_core.RestoreCartRequest) (*pb_core.RestoreCartResponse, error) {
	if f.restoreCartErr != nil {
		return nil, f.restoreCartErr
	}
	return &pb_core.RestoreCartResponse{Success: true}, nil
}

// ─── bufconn helper ───────────────────────────────────────────────────────────

func startFakeCartServer(t *testing.T, srv pb_core.CartServiceServer) pb_core.CartServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	pb_core.RegisterCartServiceServer(s, srv)
	go s.Serve(lis) //nolint:errcheck
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return pb_core.NewCartServiceClient(conn)
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestCoreClient_ClearCart_Success(t *testing.T) {
	client := startFakeCartServer(t, &fakeCartServer{})
	c := infra.NewCoreClient(client)
	err := c.ClearCart(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestCoreClient_ClearCart_Unavailable_MapsToTransient(t *testing.T) {
	srv := &fakeCartServer{clearCartErr: status.Error(codes.Unavailable, "down")}
	client := startFakeCartServer(t, srv)
	c := infra.NewCoreClient(client)

	err := c.ClearCart(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrTransient)
}

func TestCoreClient_ClearCart_NotFound_MapsToNotFound(t *testing.T) {
	srv := &fakeCartServer{clearCartErr: status.Error(codes.NotFound, "not found")}
	client := startFakeCartServer(t, srv)
	c := infra.NewCoreClient(client)

	err := c.ClearCart(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestCoreClient_RestoreCart_Success(t *testing.T) {
	client := startFakeCartServer(t, &fakeCartServer{})
	c := infra.NewCoreClient(client)

	items := []domain.OrderItemSnapshot{
		{ProductID: uuid.New(), StoreID: uuid.New(), Quantity: 2, PriceKopecks: 500},
	}
	err := c.RestoreCart(context.Background(), uuid.New(), items)
	require.NoError(t, err)
}

func TestCoreClient_RestoreCart_Unavailable_MapsToTransient(t *testing.T) {
	srv := &fakeCartServer{restoreCartErr: status.Error(codes.Unavailable, "down")}
	client := startFakeCartServer(t, srv)
	c := infra.NewCoreClient(client)

	err := c.RestoreCart(context.Background(), uuid.New(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrTransient)
}
