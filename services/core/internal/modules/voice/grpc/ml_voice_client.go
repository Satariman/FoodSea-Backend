package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/foodsea/core/internal/modules/voice/domain"
	pbml "github.com/foodsea/proto/ml"
)

type voiceServiceStub interface {
	ParseShoppingList(ctx context.Context, in *pbml.ParseShoppingListRequest, opts ...grpc.CallOption) (*pbml.ParseShoppingListResponse, error)
}

type MLVoiceClient struct {
	client  voiceServiceStub
	timeout time.Duration
}

func NewMLVoiceClient(conn *grpc.ClientConn, timeout time.Duration) *MLVoiceClient {
	return &MLVoiceClient{client: pbml.NewVoiceServiceClient(conn), timeout: timeout}
}

func (c *MLVoiceClient) ParseShoppingList(ctx context.Context, text, locale string) ([]domain.VoiceItem, []string, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	resp, err := c.client.ParseShoppingList(ctx, &pbml.ParseShoppingListRequest{
		Text:        text,
		Locale:      locale,
		TopKPerItem: 1,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("ml voice ParseShoppingList: %w", err)
	}
	items := make([]domain.VoiceItem, len(resp.GetItems()))
	for i, it := range resp.GetItems() {
		items[i] = domain.VoiceItem{
			ProductID:   it.GetProductId(),
			ProductName: it.GetProductName(),
			Quantity:    it.GetQuantity(),
			Unit:        it.GetUnit(),
			Confidence:  it.GetConfidence(),
			RawQuery:    it.GetRawQuery(),
		}
	}
	return items, resp.GetUnmatchedQueries(), nil
}
