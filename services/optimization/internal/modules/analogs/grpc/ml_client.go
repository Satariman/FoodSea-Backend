package grpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/optimization/internal/modules/analogs/domain"
	pbml "github.com/foodsea/proto/ml"
)

// MLClient adapts ml AnalogService gRPC to domain.AnalogProvider.
type MLClient struct {
	client pbml.AnalogServiceClient
	log    *slog.Logger
}

func NewMLClient(client pbml.AnalogServiceClient, log *slog.Logger) *MLClient {
	return &MLClient{client: client, log: log}
}

func (c *MLClient) GetAnalogs(ctx context.Context, productID uuid.UUID, topK int) ([]domain.Analog, error) {
	resp, err := c.client.GetAnalogs(ctx, &pbml.GetAnalogsRequest{
		ProductId:  productID.String(),
		TopK:       int32(topK),
		PriceAware: true,
	})
	if err != nil {
		return nil, fmt.Errorf("ml GetAnalogs: %w", err)
	}

	out := make([]domain.Analog, 0, len(resp.GetAnalogs()))
	for _, analog := range resp.GetAnalogs() {
		id, parseErr := uuid.Parse(analog.GetProductId())
		if parseErr != nil {
			c.log.WarnContext(ctx, "ml returned invalid analog product id", "product_id", analog.GetProductId(), "error", parseErr)
			continue
		}
		out = append(out, domain.Analog{
			ProductID:       id,
			ProductName:     analog.GetProductName(),
			Score:           analog.GetScore(),
			MinPriceKopecks: analog.GetMinPriceKopecks(),
		})
	}

	return out, nil
}

func (c *MLClient) GetBatchAnalogsForStores(
	ctx context.Context,
	productIDs []uuid.UUID,
	topK int,
	storeIDs []uuid.UUID,
) (map[uuid.UUID][]domain.Analog, error) {
	resp, err := c.client.GetBatchAnalogs(ctx, &pbml.GetBatchAnalogsRequest{
		ProductIds:     uuidsToStrings(productIDs),
		TopK:           int32(topK),
		FilterStoreIds: uuidsToStrings(storeIDs),
	})
	if err != nil {
		return nil, fmt.Errorf("ml GetBatchAnalogs: %w", err)
	}

	result := make(map[uuid.UUID][]domain.Analog, len(resp.GetAnalogsByProduct()))
	for productIDRaw, list := range resp.GetAnalogsByProduct() {
		productID, parseErr := uuid.Parse(productIDRaw)
		if parseErr != nil {
			c.log.WarnContext(ctx, "ml returned invalid source product id", "product_id", productIDRaw, "error", parseErr)
			continue
		}

		analogs := make([]domain.Analog, 0, len(list.GetAnalogs()))
		for _, analog := range list.GetAnalogs() {
			analogID, analogErr := uuid.Parse(analog.GetProductId())
			if analogErr != nil {
				c.log.WarnContext(ctx, "ml returned invalid analog product id", "product_id", analog.GetProductId(), "error", analogErr)
				continue
			}
			analogs = append(analogs, domain.Analog{
				ProductID:       analogID,
				ProductName:     analog.GetProductName(),
				Score:           analog.GetScore(),
				MinPriceKopecks: analog.GetMinPriceKopecks(),
			})
		}
		result[productID] = analogs
	}

	return result, nil
}

func uuidsToStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i := range ids {
		out[i] = ids[i].String()
	}
	return out
}
