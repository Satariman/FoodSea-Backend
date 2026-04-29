package grpcclient

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb_core "github.com/foodsea/proto/core"
	pb_ml "github.com/foodsea/proto/ml"

	"github.com/foodsea/optimization/internal/platform/config"
)

const bufSize = 1024 * 1024

type fakeCoreServer struct {
	pb_core.UnimplementedCartServiceServer
	pb_core.UnimplementedOfferServiceServer
	pb_core.UnimplementedCatalogServiceServer

	offerFailuresLeft int32
	receivedMD        metadata.MD
}

func (s *fakeCoreServer) GetCartItems(ctx context.Context, _ *pb_core.GetCartItemsRequest) (*pb_core.GetCartItemsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.receivedMD = md
	return &pb_core.GetCartItemsResponse{
		Items: []*pb_core.CartItemProto{{ProductId: "p1", ProductName: "Milk", Quantity: 2}},
	}, nil
}

func (s *fakeCoreServer) ClearCart(context.Context, *pb_core.ClearCartRequest) (*pb_core.ClearCartResponse, error) {
	return &pb_core.ClearCartResponse{Success: true}, nil
}

func (s *fakeCoreServer) RestoreCart(context.Context, *pb_core.RestoreCartRequest) (*pb_core.RestoreCartResponse, error) {
	return &pb_core.RestoreCartResponse{Success: true}, nil
}

func (s *fakeCoreServer) GetOffers(ctx context.Context, _ *pb_core.GetOffersRequest) (*pb_core.GetOffersResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.receivedMD = md
	if atomic.LoadInt32(&s.offerFailuresLeft) > 0 {
		atomic.AddInt32(&s.offerFailuresLeft, -1)
		return nil, status.Error(codes.Unavailable, "try again")
	}
	return &pb_core.GetOffersResponse{
		Offers: []*pb_core.OfferProto{{ProductId: "p1", StoreId: "s1", StoreName: "Store", PriceKopecks: 12345, InStock: true}},
	}, nil
}

func (s *fakeCoreServer) GetDeliveryConditions(context.Context, *pb_core.GetDeliveryConditionsRequest) (*pb_core.GetDeliveryConditionsResponse, error) {
	return &pb_core.GetDeliveryConditionsResponse{}, nil
}

func (s *fakeCoreServer) ListProductsForML(context.Context, *pb_core.ListProductsForMLRequest) (*pb_core.ListProductsForMLResponse, error) {
	return &pb_core.ListProductsForMLResponse{}, nil
}

type fakeMLServer struct {
	pb_ml.UnimplementedAnalogServiceServer

	receivedMD metadata.MD
}

func (s *fakeMLServer) GetAnalogs(ctx context.Context, _ *pb_ml.GetAnalogsRequest) (*pb_ml.GetAnalogsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.receivedMD = md
	return &pb_ml.GetAnalogsResponse{
		Analogs: []*pb_ml.AnalogProto{{ProductId: "p2", ProductName: "Milk Analog", Score: 0.91, MinPriceKopecks: 12000}},
	}, nil
}

func (s *fakeMLServer) GetBatchAnalogs(context.Context, *pb_ml.GetBatchAnalogsRequest) (*pb_ml.GetBatchAnalogsResponse, error) {
	return &pb_ml.GetBatchAnalogsResponse{AnalogsByProduct: map[string]*pb_ml.AnalogList{}}, nil
}

func setupBufConnServer(t *testing.T, register func(s *grpc.Server), serviceImpl any) (*bufconn.Listener, any) {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	register(s)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() { s.Stop() })
	return lis, serviceImpl
}

func dialerFor(coreLis, mlLis *bufconn.Listener) func(ctx context.Context, addr string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		switch {
		case strings.Contains(addr, "core-bufnet"):
			return coreLis.DialContext(ctx)
		case strings.Contains(addr, "ml-bufnet"):
			return mlLis.DialContext(ctx)
		default:
			return nil, status.Errorf(codes.Unavailable, "unknown target: %s", addr)
		}
	}
}

func TestClientSet_CallsCoreAndML(t *testing.T) {
	coreSrv := &fakeCoreServer{}
	mlSrv := &fakeMLServer{}

	coreLis := bufconn.Listen(bufSize)
	coreGRPC := grpc.NewServer()
	pb_core.RegisterCartServiceServer(coreGRPC, coreSrv)
	pb_core.RegisterOfferServiceServer(coreGRPC, coreSrv)
	pb_core.RegisterCatalogServiceServer(coreGRPC, coreSrv)
	go func() { _ = coreGRPC.Serve(coreLis) }()
	t.Cleanup(coreGRPC.Stop)

	mlLis := bufconn.Listen(bufSize)
	mlGRPC := grpc.NewServer()
	pb_ml.RegisterAnalogServiceServer(mlGRPC, mlSrv)
	go func() { _ = mlGRPC.Serve(mlLis) }()
	t.Cleanup(mlGRPC.Stop)

	cfg := config.Config{
		GRPCClients: config.GRPCClientConfig{
			CoreAddr: "passthrough:///core-bufnet",
			MLAddr:   "passthrough:///ml-bufnet",
		},
	}

	clients, err := Dial(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), grpc.WithContextDialer(dialerFor(coreLis, mlLis)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = clients.Close() })

	cartResp, err := clients.Cart.GetCartItems(context.Background(), &pb_core.GetCartItemsRequest{UserId: "u1"})
	require.NoError(t, err)
	require.Len(t, cartResp.Items, 1)
	assert.Equal(t, "Milk", cartResp.Items[0].ProductName)

	offersResp, err := clients.Offer.GetOffers(context.Background(), &pb_core.GetOffersRequest{ProductIds: []string{"p1"}})
	require.NoError(t, err)
	require.Len(t, offersResp.Offers, 1)
	assert.Equal(t, int64(12345), offersResp.Offers[0].PriceKopecks)

	analogsResp, err := clients.Analog.GetAnalogs(context.Background(), &pb_ml.GetAnalogsRequest{ProductId: "p1", TopK: 1})
	require.NoError(t, err)
	require.Len(t, analogsResp.Analogs, 1)
	assert.Equal(t, "p2", analogsResp.Analogs[0].ProductId)
}

func TestClientSet_RetryOnUnavailable(t *testing.T) {
	coreSrv := &fakeCoreServer{offerFailuresLeft: 2}
	mlSrv := &fakeMLServer{}

	coreLis := bufconn.Listen(bufSize)
	coreGRPC := grpc.NewServer()
	pb_core.RegisterCartServiceServer(coreGRPC, coreSrv)
	pb_core.RegisterOfferServiceServer(coreGRPC, coreSrv)
	pb_core.RegisterCatalogServiceServer(coreGRPC, coreSrv)
	go func() { _ = coreGRPC.Serve(coreLis) }()
	t.Cleanup(coreGRPC.Stop)

	mlLis := bufconn.Listen(bufSize)
	mlGRPC := grpc.NewServer()
	pb_ml.RegisterAnalogServiceServer(mlGRPC, mlSrv)
	go func() { _ = mlGRPC.Serve(mlLis) }()
	t.Cleanup(mlGRPC.Stop)

	cfg := config.Config{GRPCClients: config.GRPCClientConfig{CoreAddr: "passthrough:///core-bufnet", MLAddr: "passthrough:///ml-bufnet"}}
	clients, err := Dial(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), grpc.WithContextDialer(dialerFor(coreLis, mlLis)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = clients.Close() })

	_, err = clients.Offer.GetOffers(context.Background(), &pb_core.GetOffersRequest{ProductIds: []string{"p1"}})
	require.NoError(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&coreSrv.offerFailuresLeft))
}

func TestClientSet_RequestIDPropagation(t *testing.T) {
	coreSrv := &fakeCoreServer{}
	mlSrv := &fakeMLServer{}

	coreLis := bufconn.Listen(bufSize)
	coreGRPC := grpc.NewServer()
	pb_core.RegisterCartServiceServer(coreGRPC, coreSrv)
	pb_core.RegisterOfferServiceServer(coreGRPC, coreSrv)
	pb_core.RegisterCatalogServiceServer(coreGRPC, coreSrv)
	go func() { _ = coreGRPC.Serve(coreLis) }()
	t.Cleanup(coreGRPC.Stop)

	mlLis := bufconn.Listen(bufSize)
	mlGRPC := grpc.NewServer()
	pb_ml.RegisterAnalogServiceServer(mlGRPC, mlSrv)
	go func() { _ = mlGRPC.Serve(mlLis) }()
	t.Cleanup(mlGRPC.Stop)

	cfg := config.Config{GRPCClients: config.GRPCClientConfig{CoreAddr: "passthrough:///core-bufnet", MLAddr: "passthrough:///ml-bufnet"}}
	clients, err := Dial(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), grpc.WithContextDialer(dialerFor(coreLis, mlLis)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = clients.Close() })

	ctx := WithRequestID(context.Background(), "req-123")

	_, err = clients.Cart.GetCartItems(ctx, &pb_core.GetCartItemsRequest{UserId: "u1"})
	require.NoError(t, err)
	_, err = clients.Analog.GetAnalogs(ctx, &pb_ml.GetAnalogsRequest{ProductId: "p1", TopK: 1})
	require.NoError(t, err)

	require.NotNil(t, coreSrv.receivedMD)
	require.NotNil(t, mlSrv.receivedMD)
	assert.Equal(t, "req-123", coreSrv.receivedMD.Get("x-request-id")[0])
	assert.Equal(t, "req-123", mlSrv.receivedMD.Get("x-request-id")[0])
}
