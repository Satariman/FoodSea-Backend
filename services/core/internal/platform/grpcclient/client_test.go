package grpcclient

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	pbml "github.com/foodsea/proto/ml"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeAnalogServer struct {
	pbml.UnimplementedAnalogServiceServer
}

func (s *fakeAnalogServer) SearchByPhoto(ctx context.Context, req *pbml.SearchByPhotoRequest) (*pbml.SearchByPhotoResponse, error) {
	return &pbml.SearchByPhotoResponse{MatchedName: "milk"}, nil
}

func TestDialML(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pbml.RegisterAnalogServiceServer(srv, &fakeAnalogServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	clients, err := DialML(
		context.Background(),
		"passthrough:///bufnet",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clients.Close()) })

	resp, err := clients.Analog.SearchByPhoto(context.Background(), &pbml.SearchByPhotoRequest{
		Image:         []byte("img"),
		ImageMimeType: "image/jpeg",
		OcrText:       "milk",
		TopK:          1,
	})
	require.NoError(t, err)
	require.Equal(t, "milk", resp.GetMatchedName())
}

func TestRetryInterceptor_RetriesOnUnavailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := retryInterceptor(3, logger)

	attempts := 0
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		attempts++
		if attempts == 1 {
			return status.Error(codes.Unavailable, "temporary unavailable")
		}
		return nil
	}

	err := interceptor(context.Background(), "/ml.AnalogService/SearchByPhoto", nil, nil, nil, invoker)
	require.NoError(t, err)
	require.Equal(t, 2, attempts)
}
