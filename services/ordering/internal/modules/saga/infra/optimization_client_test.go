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

	pb_opt "github.com/foodsea/proto/optimization"

	"github.com/foodsea/ordering/internal/modules/saga/domain"
	"github.com/foodsea/ordering/internal/modules/saga/infra"
)

// ─── fake OptimizationService server ─────────────────────────────────────────

type fakeOptServer struct {
	pb_opt.UnimplementedOptimizationServiceServer
	lockErr   error
	unlockErr error
	getResult *pb_opt.GetResultResponse
	getErr    error
}

func (f *fakeOptServer) LockResult(_ context.Context, _ *pb_opt.LockResultRequest) (*pb_opt.LockResultResponse, error) {
	if f.lockErr != nil {
		return nil, f.lockErr
	}
	return &pb_opt.LockResultResponse{Success: true}, nil
}

func (f *fakeOptServer) UnlockResult(_ context.Context, _ *pb_opt.UnlockResultRequest) (*pb_opt.UnlockResultResponse, error) {
	if f.unlockErr != nil {
		return nil, f.unlockErr
	}
	return &pb_opt.UnlockResultResponse{Success: true}, nil
}

func (f *fakeOptServer) GetResult(_ context.Context, _ *pb_opt.GetResultRequest) (*pb_opt.GetResultResponse, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResult, nil
}

func startFakeOptServer(t *testing.T, srv pb_opt.OptimizationServiceServer) pb_opt.OptimizationServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	pb_opt.RegisterOptimizationServiceServer(s, srv)
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
	return pb_opt.NewOptimizationServiceClient(conn)
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestOptimizationClient_LockResult_Success(t *testing.T) {
	client := startFakeOptServer(t, &fakeOptServer{})
	c := infra.NewOptimizationClient(client)
	require.NoError(t, c.LockResult(context.Background(), uuid.New()))
}

func TestOptimizationClient_LockResult_Unavailable_MapsToTransient(t *testing.T) {
	srv := &fakeOptServer{lockErr: status.Error(codes.Unavailable, "down")}
	client := startFakeOptServer(t, srv)
	c := infra.NewOptimizationClient(client)

	err := c.LockResult(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrTransient)
}

func TestOptimizationClient_UnlockResult_NotFound_IsSuccess(t *testing.T) {
	// NotFound on UnlockResult should be treated as idempotent success.
	srv := &fakeOptServer{unlockErr: status.Error(codes.NotFound, "not found")}
	client := startFakeOptServer(t, srv)
	c := infra.NewOptimizationClient(client)

	err := c.UnlockResult(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestOptimizationClient_GetResult_Success(t *testing.T) {
	userID := uuid.New()
	resultID := uuid.New()
	productID := uuid.New()
	storeID := uuid.New()

	srv := &fakeOptServer{
		getResult: &pb_opt.GetResultResponse{
			Result: &pb_opt.OptimizationResultProto{
				Id:              resultID.String(),
				UserId:          userID.String(),
				TotalKopecks:    1000,
				DeliveryKopecks: 100,
				Status:          "active",
				Items: []*pb_opt.OptimizationItemProto{
					{
						ProductId:    productID.String(),
						StoreId:      storeID.String(),
						StoreName:    "Store A",
						Quantity:     2,
						PriceKopecks: 500,
					},
				},
			},
		},
	}

	client := startFakeOptServer(t, srv)
	c := infra.NewOptimizationClient(client)

	result, err := c.GetResult(context.Background(), resultID)
	require.NoError(t, err)
	assert.Equal(t, resultID, result.ID)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, "active", result.Status)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, productID, result.Items[0].ProductID)
}

func TestOptimizationClient_GetResult_NotFound(t *testing.T) {
	srv := &fakeOptServer{getErr: status.Error(codes.NotFound, "not found")}
	client := startFakeOptServer(t, srv)
	c := infra.NewOptimizationClient(client)

	_, err := c.GetResult(context.Background(), uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
}
