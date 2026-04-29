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

	"github.com/foodsea/core/internal/modules/cart/domain"
	"github.com/foodsea/core/internal/modules/cart/handler"
	"github.com/foodsea/core/internal/platform/middleware"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- mocks ---

type mockGetCart struct{ mock.Mock }

func (m *mockGetCart) Execute(ctx context.Context, userID uuid.UUID) (*domain.Cart, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).(*domain.Cart)
	return v, args.Error(1)
}

type mockAddItem struct{ mock.Mock }

func (m *mockAddItem) Execute(ctx context.Context, userID, productID uuid.UUID, qty int16) error {
	args := m.Called(ctx, userID, productID, qty)
	return args.Error(0)
}

type mockUpdateItem struct{ mock.Mock }

func (m *mockUpdateItem) Execute(ctx context.Context, userID, productID uuid.UUID, qty int16) error {
	args := m.Called(ctx, userID, productID, qty)
	return args.Error(0)
}

type mockRemoveItem struct{ mock.Mock }

func (m *mockRemoveItem) Execute(ctx context.Context, userID, productID uuid.UUID) error {
	args := m.Called(ctx, userID, productID)
	return args.Error(0)
}

type mockClearCart struct{ mock.Mock }

func (m *mockClearCart) Execute(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// --- helpers ---

func newRouter(h *handler.CartHandler, userID uuid.UUID) *gin.Engine {
	r := gin.New()
	// inject user ID without real JWT middleware
	r.Use(func(c *gin.Context) {
		ctx := middleware.WithUserIDContext(c.Request.Context(), userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/cart", h.GetCart)
	r.POST("/cart/items", h.AddItem)
	r.PUT("/cart/items/:product_id", h.UpdateItem)
	r.DELETE("/cart/items/:product_id", h.RemoveItem)
	r.DELETE("/cart", h.ClearCart)
	return r
}

func emptyCart(userID uuid.UUID) *domain.Cart {
	return &domain.Cart{ID: uuid.New(), UserID: userID, Items: []domain.CartItem{}}
}

func cartWith(userID, productID uuid.UUID, qty int16) *domain.Cart {
	return &domain.Cart{
		ID:     uuid.New(),
		UserID: userID,
		Items: []domain.CartItem{
			{ID: uuid.New(), ProductID: productID, ProductName: "Test", Quantity: qty, AddedAt: time.Now()},
		},
	}
}

// --- tests ---

func TestGetCart_OK(t *testing.T) {
	getC := &mockGetCart{}
	addI := &mockAddItem{}
	updI := &mockUpdateItem{}
	remI := &mockRemoveItem{}
	clrC := &mockClearCart{}

	userID := uuid.New()
	getC.On("Execute", mock.Anything, userID).Return(emptyCart(userID), nil)

	h := handler.NewCartHandler(getC, addI, updI, remI, clrC)
	r := newRouter(h, userID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/cart", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	getC.AssertExpectations(t)
}

func TestAddItem_BadProductID_400(t *testing.T) {
	getC := &mockGetCart{}
	addI := &mockAddItem{}
	updI := &mockUpdateItem{}
	remI := &mockRemoveItem{}
	clrC := &mockClearCart{}

	userID := uuid.New()
	h := handler.NewCartHandler(getC, addI, updI, remI, clrC)
	r := newRouter(h, userID)

	body := `{"product_id":"not-a-uuid","quantity":1}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/cart/items", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddItem_QtyTooHigh_400(t *testing.T) {
	getC := &mockGetCart{}
	addI := &mockAddItem{}
	updI := &mockUpdateItem{}
	remI := &mockRemoveItem{}
	clrC := &mockClearCart{}

	userID := uuid.New()
	h := handler.NewCartHandler(getC, addI, updI, remI, clrC)
	r := newRouter(h, userID)

	body, _ := json.Marshal(map[string]any{"product_id": uuid.New().String(), "quantity": 100})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/cart/items", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddItem_Success_201(t *testing.T) {
	getC := &mockGetCart{}
	addI := &mockAddItem{}
	updI := &mockUpdateItem{}
	remI := &mockRemoveItem{}
	clrC := &mockClearCart{}

	userID := uuid.New()
	productID := uuid.New()

	addI.On("Execute", mock.Anything, userID, productID, int16(2)).Return(nil)
	getC.On("Execute", mock.Anything, userID).Return(cartWith(userID, productID, 2), nil)

	h := handler.NewCartHandler(getC, addI, updI, remI, clrC)
	r := newRouter(h, userID)

	body, _ := json.Marshal(map[string]any{"product_id": productID.String(), "quantity": 2})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/cart/items", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestRemoveItem_Idempotent_204(t *testing.T) {
	getC := &mockGetCart{}
	addI := &mockAddItem{}
	updI := &mockUpdateItem{}
	remI := &mockRemoveItem{}
	clrC := &mockClearCart{}

	userID := uuid.New()
	productID := uuid.New()
	remI.On("Execute", mock.Anything, userID, productID).Return(nil)

	h := handler.NewCartHandler(getC, addI, updI, remI, clrC)
	r := newRouter(h, userID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/cart/items/"+productID.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodDelete, "/cart/items/"+productID.String(), nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNoContent, w2.Code)
}

func TestAddItem_NoAuth_401(t *testing.T) {
	getC := &mockGetCart{}
	addI := &mockAddItem{}
	updI := &mockUpdateItem{}
	remI := &mockRemoveItem{}
	clrC := &mockClearCart{}

	h := handler.NewCartHandler(getC, addI, updI, remI, clrC)

	// router WITHOUT user injection — simulates missing auth
	r := gin.New()
	r.POST("/cart/items", h.AddItem)

	body, _ := json.Marshal(map[string]any{"product_id": uuid.New().String(), "quantity": 1})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/cart/items", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAddItem_ProductNotFound_404(t *testing.T) {
	getC := &mockGetCart{}
	addI := &mockAddItem{}
	updI := &mockUpdateItem{}
	remI := &mockRemoveItem{}
	clrC := &mockClearCart{}

	userID := uuid.New()
	productID := uuid.New()
	addI.On("Execute", mock.Anything, userID, productID, int16(1)).Return(sherrors.ErrNotFound)

	h := handler.NewCartHandler(getC, addI, updI, remI, clrC)
	r := newRouter(h, userID)

	body, _ := json.Marshal(map[string]any{"product_id": productID.String(), "quantity": 1})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/cart/items", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
