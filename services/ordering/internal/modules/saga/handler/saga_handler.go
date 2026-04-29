package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/saga/domain"
	"github.com/foodsea/ordering/internal/platform/httputil"
	"github.com/foodsea/ordering/internal/platform/middleware"
)

type startSagaExecutor interface {
	Start(ctx context.Context, input domain.StartInput) (uuid.UUID, error)
}

type sagaStateReader interface {
	GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.SagaState, error)
}

// SagaHandler wires saga use cases to HTTP routes.
type SagaHandler struct {
	orchestrator startSagaExecutor
	repo         sagaStateReader
}

// NewSagaHandler constructs the handler.
func NewSagaHandler(orchestrator startSagaExecutor, repo sagaStateReader) *SagaHandler {
	return &SagaHandler{orchestrator: orchestrator, repo: repo}
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type placeOrderRequest struct {
	OptimizationResultID uuid.UUID `json:"optimization_result_id" binding:"required"`
}

type placeOrderResponse struct {
	OrderID uuid.UUID `json:"order_id"`
	Status  string    `json:"status"`
}

type sagaStateResponse struct {
	SagaID      uuid.UUID `json:"saga_id"`
	OrderID     uuid.UUID `json:"order_id"`
	UserID      uuid.UUID `json:"user_id"`
	CurrentStep int8      `json:"current_step"`
	Status      string    `json:"status"`
	Failure     string    `json:"failure_reason,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// PlaceOrder godoc
// @Summary      Place a new order via saga
// @Tags         orders
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  placeOrderRequest  true  "Optimization result ID"
// @Success      201   {object}  httputil.Response{data=placeOrderResponse}
// @Failure      409   {object}  httputil.Response
// @Failure      500   {object}  httputil.Response
// @Router       /orders [post]
func (h *SagaHandler) PlaceOrder(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	var req placeOrderRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	orderID, err := h.orchestrator.Start(c.Request.Context(), domain.StartInput{
		UserID:               userID,
		OptimizationResultID: req.OptimizationResultID,
	})
	if err != nil {
		if errors.Is(err, domain.ErrManualIntervention) {
			c.JSON(http.StatusInternalServerError, httputil.Response{Error: "saga failed and requires manual intervention"})
			return
		}
		httputil.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, httputil.Response{Data: placeOrderResponse{
		OrderID: orderID,
		Status:  "confirmed",
	}})
}

// GetSagaState godoc
// @Summary      Get saga state for an order (admin/debug)
// @Tags         orders
// @Security     BearerAuth
// @Produce      json
// @Param        id   path  string  true  "Order UUID"
// @Success      200  {object}  httputil.Response{data=sagaStateResponse}
// @Failure      404  {object}  httputil.Response
// @Router       /orders/{id}/saga [get]
func (h *SagaHandler) GetSagaState(c *gin.Context) {
	orderID, ok := httputil.ParseUUID(c, "id")
	if !ok {
		return // ParseUUID already wrote the error response
	}

	s, err := h.repo.GetByOrderID(c.Request.Context(), orderID)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, sagaStateResponse{
		SagaID:      s.ID,
		OrderID:     s.OrderID,
		UserID:      s.UserID,
		CurrentStep: s.CurrentStep,
		Status:      string(s.Status),
		Failure:     s.Payload.FailureReason,
		UpdatedAt:   s.UpdatedAt,
	})
}
