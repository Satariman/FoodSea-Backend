package grpcclient

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb_core "github.com/foodsea/proto/core"
)

const bufSize = 1024 * 1024

// fakeCartServer implements pb_core.CartServiceServer.
type fakeCartServer struct {
	pb_core.UnimplementedCartServiceServer
	clearCartErr    error
	clearCartCalled int
	receivedMD      metadata.MD
}

func (s *fakeCartServer) ClearCart(ctx context.Context, req *pb_core.ClearCartRequest) (*pb_core.ClearCartResponse, error) {
	s.clearCartCalled++
	md, _ := metadata.FromIncomingContext(ctx)
	s.receivedMD = md
	if s.clearCartErr != nil {
		return nil, s.clearCartErr
	}
	return &pb_core.ClearCartResponse{Success: true}, nil
}

func setupBufConnServer(t *testing.T, srv pb_core.CartServiceServer) pb_core.CartServiceClient {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	pb_core.RegisterCartServiceServer(s, srv)

	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() { s.Stop() })

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			requestIDInterceptor(),
			retryInterceptor(3, slog.New(slog.NewTextHandler(io.Discard, nil))),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pb_core.NewCartServiceClient(conn)
}

func TestClearCart_Success(t *testing.T) {
	fakeSrv := &fakeCartServer{}
	client := setupBufConnServer(t, fakeSrv)

	resp, err := client.ClearCart(context.Background(), &pb_core.ClearCartRequest{UserId: "user-1"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestClearCart_RetryOnUnavailable(t *testing.T) {
	attempts := 0
	fakeSrv := &fakeCartServer{
		clearCartErr: status.Errorf(codes.Unavailable, "service temporarily unavailable"),
	}
	// Override to fail 3 times (max retries)
	_ = attempts
	client := setupBufConnServer(t, fakeSrv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.ClearCart(ctx, &pb_core.ClearCartRequest{UserId: "user-1"})
	assert.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
	// Should have been called 3 times (initial + 2 retries)
	assert.Equal(t, 3, fakeSrv.clearCartCalled)
}

func TestRequestIDPropagation(t *testing.T) {
	fakeSrv := &fakeCartServer{}
	client := setupBufConnServer(t, fakeSrv)

	ctx := WithRequestID(context.Background(), "test-request-id-123")
	_, err := client.ClearCart(ctx, &pb_core.ClearCartRequest{UserId: "user-1"})
	require.NoError(t, err)

	vals := fakeSrv.receivedMD.Get("x-request-id")
	require.NotEmpty(t, vals)
	assert.Equal(t, "test-request-id-123", vals[0])
}

func TestContextTimeout_Cancelled(t *testing.T) {
	// Create a server that always blocks
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	blockingSrv := &blockingCartServer{}
	pb_core.RegisterCartServiceServer(s, blockingSrv)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() { s.Stop() })

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := pb_core.NewCartServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = client.ClearCart(ctx, &pb_core.ClearCartRequest{UserId: "user-1"})
	assert.Error(t, err)
	code := status.Code(err)
	assert.True(t, code == codes.DeadlineExceeded || code == codes.Canceled)
}

type blockingCartServer struct {
	pb_core.UnimplementedCartServiceServer
}

func (s *blockingCartServer) ClearCart(ctx context.Context, _ *pb_core.ClearCartRequest) (*pb_core.ClearCartResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
