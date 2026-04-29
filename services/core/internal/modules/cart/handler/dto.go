package handler

import (
	"time"

	"github.com/foodsea/core/internal/modules/cart/domain"
)

// AddItemRequest is the request body for POST /cart/items.
type AddItemRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int16  `json:"quantity"   binding:"required,min=1,max=99"`
}

// UpdateItemRequest is the request body for PUT /cart/items/:product_id.
type UpdateItemRequest struct {
	Quantity int16 `json:"quantity" binding:"required,min=1,max=99"`
}

// CartItemResponse is the per-item representation in GET /cart.
type CartItemResponse struct {
	ProductID   string    `json:"product_id"`
	ProductName string    `json:"product_name"`
	Quantity    int16     `json:"quantity"`
	AddedAt     time.Time `json:"added_at"`
}

// CartResponse is the response body for all /cart endpoints that return cart state.
type CartResponse struct {
	Items []CartItemResponse `json:"items"`
}

func toCartResponse(c *domain.Cart) CartResponse {
	items := make([]CartItemResponse, len(c.Items))
	for i, item := range c.Items {
		items[i] = CartItemResponse{
			ProductID:   item.ProductID.String(),
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			AddedAt:     item.AddedAt,
		}
	}
	return CartResponse{Items: items}
}
