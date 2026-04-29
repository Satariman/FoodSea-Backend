package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	"github.com/foodsea/ordering/internal/platform/httputil"
	"github.com/foodsea/ordering/internal/platform/middleware"
	shared "github.com/foodsea/ordering/internal/shared/domain"
	sherrors "github.com/foodsea/ordering/internal/shared/errors"
)

type getOrderExecutor interface {
	Execute(ctx context.Context, orderID, userID uuid.UUID) (*domain.Order, error)
}

type listOrdersExecutor interface {
	Execute(ctx context.Context, userID uuid.UUID, p shared.Pagination) ([]domain.Order, int, error)
}

type updateStatusExecutor interface {
	Execute(ctx context.Context, orderID uuid.UUID, to shared.OrderStatus, comment *string) error
}

// OrderHandler wires use cases to HTTP routes.
type OrderHandler struct {
	getOrder     getOrderExecutor
	listOrders   listOrdersExecutor
	updateStatus updateStatusExecutor
}

// NewOrderHandler constructs the handler.
func NewOrderHandler(
	getOrder getOrderExecutor,
	listOrders listOrdersExecutor,
	updateStatus updateStatusExecutor,
) *OrderHandler {
	return &OrderHandler{
		getOrder:     getOrder,
		listOrders:   listOrders,
		updateStatus: updateStatus,
	}
}

// ─── DTOs ────────────────────────────────────────────────────────────────────

// OrderBriefResponse is a compact representation returned in list queries.
type OrderBriefResponse struct {
	ID              uuid.UUID `json:"id"`
	Status          string    `json:"status"`
	TotalKopecks    int64     `json:"total_kopecks"`
	DeliveryKopecks int64     `json:"delivery_kopecks"`
	CreatedAt       time.Time `json:"created_at"`
}

// OrderDetailResponse is the full order representation including items and history.
type OrderDetailResponse struct {
	ID                   uuid.UUID              `json:"id"`
	UserID               uuid.UUID              `json:"user_id"`
	OptimizationResultID *uuid.UUID             `json:"optimization_result_id,omitempty"`
	Status               string                 `json:"status"`
	TotalKopecks         int64                  `json:"total_kopecks"`
	DeliveryKopecks      int64                  `json:"delivery_kopecks"`
	Items                []OrderItemResponse    `json:"items"`
	History              []StatusChangeResponse `json:"history"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

// OrderItemResponse is the snapshot of a single item within an order.
type OrderItemResponse struct {
	ID           uuid.UUID `json:"id"`
	ProductID    uuid.UUID `json:"product_id"`
	ProductName  string    `json:"product_name"`
	StoreID      uuid.UUID `json:"store_id"`
	StoreName    string    `json:"store_name"`
	Quantity     int16     `json:"quantity"`
	PriceKopecks int64     `json:"price_kopecks"`
}

// StatusChangeResponse represents a single entry in the order status history.
type StatusChangeResponse struct {
	Status    string    `json:"status"`
	Comment   *string   `json:"comment,omitempty"`
	ChangedAt time.Time `json:"changed_at"`
}

// UpdateStatusRequest is the body for PATCH /orders/:id/status.
type UpdateStatusRequest struct {
	Status  string  `json:"status" binding:"required"`
	Comment *string `json:"comment,omitempty"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// ListOrders godoc
// @Summary      List user orders
// @Tags         orders
// @Security     BearerAuth
// @Produce      json
// @Param        page       query  int  false  "Page number"
// @Param        page_size  query  int  false  "Items per page"
// @Success      200  {object}  httputil.Response{data=[]OrderBriefResponse}
// @Failure      401  {object}  httputil.Response
// @Router       /orders [get]
func (h *OrderHandler) ListOrders(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, sherrors.ErrUnauthorized)
		return
	}

	p := httputil.ParsePagination(c)
	orders, total, err := h.listOrders.Execute(c.Request.Context(), userID, p)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	brief := make([]OrderBriefResponse, len(orders))
	for i, o := range orders {
		brief[i] = toOrderBrief(o)
	}

	totalPages := (total + p.PageSize - 1) / p.PageSize
	if total == 0 {
		totalPages = 0
	}
	httputil.OKWithMeta(c, brief, &httputil.Meta{
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalCount: total,
		TotalPages: totalPages,
	})
}

// GetOrder godoc
// @Summary      Get order by ID
// @Tags         orders
// @Security     BearerAuth
// @Produce      json
// @Param        id   path  string  true  "Order UUID"
// @Success      200  {object}  httputil.Response{data=OrderDetailResponse}
// @Failure      401  {object}  httputil.Response
// @Failure      404  {object}  httputil.Response
// @Router       /orders/{id} [get]
func (h *OrderHandler) GetOrder(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, sherrors.ErrUnauthorized)
		return
	}

	orderID, ok := httputil.ParseUUID(c, "id")
	if !ok {
		return
	}

	order, err := h.getOrder.Execute(c.Request.Context(), orderID, userID)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toOrderDetail(order))
}

// UpdateStatus godoc
// @Summary      Update order status (admin / demo)
// @Tags         orders
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string               true  "Order UUID"
// @Param        body  body  UpdateStatusRequest  true  "Status transition"
// @Success      200   {object}  httputil.Response
// @Failure      400   {object}  httputil.Response
// @Failure      409   {object}  httputil.Response
// @Router       /orders/{id}/status [patch]
func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	orderID, ok := httputil.ParseUUID(c, "id")
	if !ok {
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, "invalid request body")
		return
	}

	to := shared.OrderStatus(req.Status)
	if err := h.updateStatus.Execute(c.Request.Context(), orderID, to, req.Comment); err != nil {
		httputil.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, httputil.Response{Data: gin.H{"message": "status updated"}})
}

// ─── Converters ──────────────────────────────────────────────────────────────

func toOrderBrief(o domain.Order) OrderBriefResponse {
	return OrderBriefResponse{
		ID:              o.ID,
		Status:          o.Status.String(),
		TotalKopecks:    o.TotalKopecks,
		DeliveryKopecks: o.DeliveryKopecks,
		CreatedAt:       o.CreatedAt,
	}
}

func toOrderDetail(o *domain.Order) OrderDetailResponse {
	items := make([]OrderItemResponse, len(o.Items))
	for i, it := range o.Items {
		items[i] = OrderItemResponse{
			ID:           it.ID,
			ProductID:    it.ProductID,
			ProductName:  it.ProductName,
			StoreID:      it.StoreID,
			StoreName:    it.StoreName,
			Quantity:     it.Quantity,
			PriceKopecks: it.PriceKopecks,
		}
	}

	history := make([]StatusChangeResponse, len(o.History))
	for i, h := range o.History {
		history[i] = StatusChangeResponse{
			Status:    h.Status.String(),
			Comment:   h.Comment,
			ChangedAt: h.ChangedAt,
		}
	}

	return OrderDetailResponse{
		ID:                   o.ID,
		UserID:               o.UserID,
		OptimizationResultID: o.OptimizationResultID,
		Status:               o.Status.String(),
		TotalKopecks:         o.TotalKopecks,
		DeliveryKopecks:      o.DeliveryKopecks,
		Items:                items,
		History:              history,
		CreatedAt:            o.CreatedAt,
		UpdatedAt:            o.UpdatedAt,
	}
}
