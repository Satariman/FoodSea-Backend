package infra

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	pb_opt "github.com/foodsea/proto/optimization"

	"github.com/foodsea/ordering/internal/modules/saga/domain"
)

// OptimizationClient implements domain.OptimizationParticipant via gRPC.
type OptimizationClient struct {
	client pb_opt.OptimizationServiceClient
}

// NewOptimizationClient creates an OptimizationClient.
func NewOptimizationClient(client pb_opt.OptimizationServiceClient) *OptimizationClient {
	return &OptimizationClient{client: client}
}

// LockResult calls optimization LockResult.
func (c *OptimizationClient) LockResult(ctx context.Context, resultID uuid.UUID) error {
	_, err := c.client.LockResult(ctx, &pb_opt.LockResultRequest{
		ResultId: resultID.String(),
	})
	return mapGRPCErr(err)
}

// UnlockResult calls optimization UnlockResult. NotFound is treated as success (idempotent).
func (c *OptimizationClient) UnlockResult(ctx context.Context, resultID uuid.UUID) error {
	_, err := c.client.UnlockResult(ctx, &pb_opt.UnlockResultRequest{
		ResultId: resultID.String(),
	})
	if err != nil {
		mapped := mapGRPCErr(err)
		if errors.Is(mapped, domain.ErrNotFound) {
			return nil // already unlocked/expired — treat as success
		}
		return mapped
	}
	return nil
}

// GetResult fetches and maps an optimization result.
func (c *OptimizationClient) GetResult(ctx context.Context, resultID uuid.UUID) (*domain.OptimizationResult, error) {
	resp, err := c.client.GetResult(ctx, &pb_opt.GetResultRequest{
		ResultId: resultID.String(),
	})
	if err != nil {
		return nil, mapGRPCErr(err)
	}
	if resp.Result == nil {
		return nil, domain.ErrNotFound
	}
	return protoToOptResult(resp.Result)
}

func protoToOptResult(p *pb_opt.OptimizationResultProto) (*domain.OptimizationResult, error) {
	id, err := uuid.Parse(p.Id)
	if err != nil {
		return nil, fmt.Errorf("parse result id: %w", err)
	}
	userID, err := uuid.Parse(p.UserId)
	if err != nil {
		return nil, fmt.Errorf("parse user id: %w", err)
	}

	items := make([]domain.OrderItemSnapshot, len(p.Items))
	for i, it := range p.Items {
		productID, err := uuid.Parse(it.ProductId)
		if err != nil {
			return nil, fmt.Errorf("parse product_id[%d]: %w", i, err)
		}
		storeID, err := uuid.Parse(it.StoreId)
		if err != nil {
			return nil, fmt.Errorf("parse store_id[%d]: %w", i, err)
		}
		items[i] = domain.OrderItemSnapshot{
			ProductID:    productID,
			ProductName:  "", // not included in optimization proto
			StoreID:      storeID,
			StoreName:    it.StoreName,
			Quantity:     int16(it.Quantity),
			PriceKopecks: it.PriceKopecks,
		}
	}

	return &domain.OptimizationResult{
		ID:              id,
		UserID:          userID,
		TotalKopecks:    p.TotalKopecks,
		DeliveryKopecks: p.DeliveryKopecks,
		Status:          p.Status,
		Items:           items,
	}, nil
}
