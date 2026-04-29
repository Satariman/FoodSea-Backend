package grpc

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
	pb "github.com/foodsea/proto/optimization"
)

type stubGetResult struct {
	result *domain.OptimizationResult
	err    error
}

func (s *stubGetResult) Execute(context.Context, uuid.UUID) (*domain.OptimizationResult, error) {
	return s.result, s.err
}

type stubLock struct{ err error }

func (s *stubLock) Execute(context.Context, uuid.UUID) error { return s.err }

type stubUnlock struct{ err error }

func (s *stubUnlock) Execute(context.Context, uuid.UUID) error { return s.err }

func TestOptimizationServer_LockAndGetResult(t *testing.T) {
	resultID := uuid.New()
	userID := uuid.New()
	productID := uuid.New()
	storeID := uuid.New()

	srv := NewOptimizationServer(
		&stubGetResult{result: &domain.OptimizationResult{
			ID:              resultID,
			UserID:          userID,
			TotalKopecks:    900,
			DeliveryKopecks: 100,
			SavingsKopecks:  50,
			Status:          "active",
			Items: []domain.Assignment{{
				ProductID:   productID,
				ProductName: "Milk",
				StoreID:     storeID,
				StoreName:   "Store",
				Quantity:    1,
				Price:       900,
			}},
		}},
		&stubLock{},
		&stubUnlock{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	pb.RegisterOptimizationServiceServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
	})

	dialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	client := pb.NewOptimizationServiceClient(conn)

	lockResp, err := client.LockResult(context.Background(), &pb.LockResultRequest{ResultId: resultID.String()})
	require.NoError(t, err)
	require.True(t, lockResp.GetSuccess())

	getResp, err := client.GetResult(context.Background(), &pb.GetResultRequest{ResultId: resultID.String()})
	require.NoError(t, err)
	require.Equal(t, resultID.String(), getResp.GetResult().GetId())
	require.Len(t, getResp.GetResult().GetItems(), 1)
	require.Equal(t, productID.String(), getResp.GetResult().GetItems()[0].GetProductId())
}
