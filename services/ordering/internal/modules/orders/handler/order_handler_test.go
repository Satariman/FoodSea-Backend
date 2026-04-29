package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	"github.com/foodsea/ordering/internal/modules/orders/handler"
	"github.com/foodsea/ordering/internal/platform/middleware"
	shared "github.com/foodsea/ordering/internal/shared/domain"
	sherrors "github.com/foodsea/ordering/internal/shared/errors"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ─── mocks ───────────────────────────────────────────────────────────────────

type mockGetOrder struct{ mock.Mock }

func (m *mockGetOrder) Execute(ctx context.Context, orderID, userID uuid.UUID) (*domain.Order, error) {
	args := m.Called(ctx, orderID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Order), args.Error(1)
}

type mockListOrders struct{ mock.Mock }

func (m *mockListOrders) Execute(ctx context.Context, userID uuid.UUID, p shared.Pagination) ([]domain.Order, int, error) {
	args := m.Called(ctx, userID, p)
	return args.Get(0).([]domain.Order), args.Int(1), args.Error(2)
}

type mockUpdateStatus struct{ mock.Mock }

func (m *mockUpdateStatus) Execute(ctx context.Context, orderID uuid.UUID, to shared.OrderStatus, comment *string) error {
	args := m.Called(ctx, orderID, to, comment)
	return args.Error(0)
}

// ─── helper ──────────────────────────────────────────────────────────────────

func newTestRouter(h *handler.OrderHandler, userID *uuid.UUID) *gin.Engine {
	r := gin.New()
	if userID != nil {
		id := *userID
		r.Use(func(c *gin.Context) {
			ctx := middleware.WithUserIDContext(c.Request.Context(), id)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		})
	}
	r.GET("/orders", h.ListOrders)
	r.GET("/orders/:id", h.GetOrder)
	r.PATCH("/orders/:id/status", h.UpdateStatus)
	return r
}

func sampleOrder(userID uuid.UUID) *domain.Order {
	return &domain.Order{
		ID:              uuid.New(),
		UserID:          userID,
		TotalKopecks:    3000,
		DeliveryKopecks: 150,
		Status:          shared.StatusCreated,
		Items: []domain.OrderItem{
			{
				ID:           uuid.New(),
				ProductID:    uuid.New(),
				ProductName:  "Milk",
				StoreID:      uuid.New(),
				StoreName:    "Shop",
				Quantity:     2,
				PriceKopecks: 1500,
			},
		},
		History:   []domain.StatusChange{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestListOrders_NoAuth_Returns401(t *testing.T) {
	h := handler.NewOrderHandler(
		&mockGetOrder{},
		&mockListOrders{},
		&mockUpdateStatus{},
	)
	r := newTestRouter(h, nil) // no userID injected

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListOrders_ReturnsOrderList(t *testing.T) {
	userID := uuid.New()
	orders := []domain.Order{
		{
			ID: uuid.New(), UserID: userID,
			TotalKopecks: 1000, DeliveryKopecks: 100,
			Status: shared.StatusCreated, CreatedAt: time.Now(),
		},
	}

	listUC := &mockListOrders{}
	listUC.On("Execute", mock.Anything, userID, shared.NewPagination(1, 20)).
		Return(orders, 1, nil)

	h := handler.NewOrderHandler(&mockGetOrder{}, listUC, &mockUpdateStatus{})
	r := newTestRouter(h, &userID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders?page=1&page_size=20", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
	listUC.AssertExpectations(t)
}

func TestGetOrder_NoAuth_Returns401(t *testing.T) {
	h := handler.NewOrderHandler(&mockGetOrder{}, &mockListOrders{}, &mockUpdateStatus{})
	r := newTestRouter(h, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetOrder_OtherUsersOrder_Returns404(t *testing.T) {
	userID := uuid.New()
	orderID := uuid.New()

	getUC := &mockGetOrder{}
	getUC.On("Execute", mock.Anything, orderID, userID).Return(nil, sherrors.ErrNotFound)

	h := handler.NewOrderHandler(getUC, &mockListOrders{}, &mockUpdateStatus{})
	r := newTestRouter(h, &userID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	getUC.AssertExpectations(t)
}

func TestGetOrder_Success_ReturnsDetail(t *testing.T) {
	userID := uuid.New()
	order := sampleOrder(userID)

	getUC := &mockGetOrder{}
	getUC.On("Execute", mock.Anything, order.ID, userID).Return(order, nil)

	h := handler.NewOrderHandler(getUC, &mockListOrders{}, &mockUpdateStatus{})
	r := newTestRouter(h, &userID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+order.ID.String(), nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, order.ID.String(), data["id"])
	assert.Equal(t, float64(3000), data["total_kopecks"])
	getUC.AssertExpectations(t)
}

func TestUpdateStatus_InvalidBody_Returns400(t *testing.T) {
	userID := uuid.New()
	h := handler.NewOrderHandler(&mockGetOrder{}, &mockListOrders{}, &mockUpdateStatus{})
	r := newTestRouter(h, &userID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/orders/"+uuid.New().String()+"/status",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateStatus_ConflictTransition_Returns409(t *testing.T) {
	userID := uuid.New()
	orderID := uuid.New()

	updateUC := &mockUpdateStatus{}
	updateUC.On("Execute", mock.Anything, orderID, shared.StatusDelivered, (*string)(nil)).
		Return(sherrors.ErrConflict)

	h := handler.NewOrderHandler(&mockGetOrder{}, &mockListOrders{}, updateUC)
	r := newTestRouter(h, &userID)

	body, _ := json.Marshal(map[string]any{"status": "delivered"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/orders/"+orderID.String()+"/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	updateUC.AssertExpectations(t)
}

func TestUpdateStatus_Success_Returns200(t *testing.T) {
	userID := uuid.New()
	orderID := uuid.New()

	updateUC := &mockUpdateStatus{}
	updateUC.On("Execute", mock.Anything, orderID, shared.StatusConfirmed, (*string)(nil)).
		Return(nil)

	h := handler.NewOrderHandler(&mockGetOrder{}, &mockListOrders{}, updateUC)
	r := newTestRouter(h, &userID)

	body, _ := json.Marshal(map[string]any{"status": "confirmed"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/orders/"+orderID.String()+"/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	updateUC.AssertExpectations(t)
}
