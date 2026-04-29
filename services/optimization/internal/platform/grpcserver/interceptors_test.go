package grpcserver_test

import (
	"context"
	"log/slog"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/foodsea/optimization/internal/platform/grpcserver"
)

const bufSize = 1 << 20

func newBufconnServer(t *testing.T, extraInterceptors ...grpc.UnaryServerInterceptor) (*grpc.Server, *bufconn.Listener) {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpcserver.New(slog.Default(), extraInterceptors...)
	grpc_testing.RegisterTestServiceServer(srv, &passthroughService{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return srv, lis
}

func newConn(t *testing.T, lis *bufconn.Listener) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// passthroughService implements grpc_testing.TestServiceServer minimally.
type passthroughService struct {
	grpc_testing.UnimplementedTestServiceServer
}

func (s *passthroughService) EmptyCall(ctx context.Context, req *grpc_testing.Empty) (*grpc_testing.Empty, error) {
	return &grpc_testing.Empty{}, nil
}

func TestRecoveryInterceptor_PanicReturnsInternal(t *testing.T) {
	// Inject a panic-inducing interceptor before the handler.
	panicInterceptor := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		panic("test panic from handler")
	}

	_, lis := newBufconnServer(t, panicInterceptor)
	conn := newConn(t, lis)

	client := grpc_testing.NewTestServiceClient(conn)
	_, err := client.EmptyCall(context.Background(), &grpc_testing.Empty{})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestNormalCall_ReturnsOK(t *testing.T) {
	_, lis := newBufconnServer(t)
	conn := newConn(t, lis)

	client := grpc_testing.NewTestServiceClient(conn)
	_, err := client.EmptyCall(context.Background(), &grpc_testing.Empty{})
	require.NoError(t, err)
}
