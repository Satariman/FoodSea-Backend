package infra

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb_core "github.com/foodsea/proto/core"

	"github.com/foodsea/ordering/internal/modules/saga/domain"
)

// CoreClient implements domain.CartParticipant via gRPC to core-service.
type CoreClient struct {
	client pb_core.CartServiceClient
}

// NewCoreClient creates a CoreClient.
func NewCoreClient(client pb_core.CartServiceClient) *CoreClient {
	return &CoreClient{client: client}
}

// ClearCart calls core CartService.ClearCart.
func (c *CoreClient) ClearCart(ctx context.Context, userID uuid.UUID) error {
	_, err := c.client.ClearCart(ctx, &pb_core.ClearCartRequest{
		UserId: userID.String(),
	})
	return mapGRPCErr(err)
}

// RestoreCart calls core CartService.RestoreCart with item snapshots.
func (c *CoreClient) RestoreCart(ctx context.Context, userID uuid.UUID, items []domain.OrderItemSnapshot) error {
	protoItems := make([]*pb_core.CartItemProto, len(items))
	for i, it := range items {
		protoItems[i] = &pb_core.CartItemProto{
			ProductId:   it.ProductID.String(),
			ProductName: it.ProductName,
			Quantity:    int32(it.Quantity),
		}
	}
	_, err := c.client.RestoreCart(ctx, &pb_core.RestoreCartRequest{
		UserId: userID.String(),
		Items:  protoItems,
	})
	return mapGRPCErr(err)
}

// mapGRPCErr maps gRPC status codes to saga domain errors.
// codes.NotFound   → domain.ErrNotFound  (compensation treats this as already done)
// codes.Unavailable / DeadlineExceeded → domain.ErrTransient  (orchestrator retries)
func mapGRPCErr(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("%w: %s", domain.ErrNotFound, st.Message())
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf("%w: %s", domain.ErrTransient, st.Message())
	case codes.InvalidArgument:
		return fmt.Errorf("invalid argument: %s", st.Message())
	default:
		return fmt.Errorf("gRPC %v: %s", st.Code(), st.Message())
	}
}
