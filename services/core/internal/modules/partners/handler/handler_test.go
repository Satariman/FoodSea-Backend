package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/partners/domain"
	"github.com/foodsea/core/internal/modules/partners/handler"
	"github.com/foodsea/core/internal/platform/httputil"
)

func init() { gin.SetMode(gin.TestMode) }

// --- mocks ---

type mockListStores struct{ mock.Mock }

func (m *mockListStores) Execute(ctx context.Context) ([]domain.Store, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).([]domain.Store)
	return v, args.Error(1)
}

type mockListOffers struct{ mock.Mock }

func (m *mockListOffers) Execute(ctx context.Context, productID uuid.UUID, hasDiscountOnly bool) ([]domain.OfferWithStore, error) {
	args := m.Called(ctx, productID, hasDiscountOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	v, _ := args.Get(0).([]domain.OfferWithStore)
	return v, args.Error(1)
}

// --- helpers ---

func newStoreRouter(h *handler.StoreHandler) *gin.Engine {
	r := gin.New()
	r.GET("/stores", h.ListStores)
	return r
}

func newOfferRouter(h *handler.OfferHandler) *gin.Engine {
	r := gin.New()
	r.GET("/products/:id/offers", h.ListOffersByProduct)
	return r
}

func fakeStore() domain.Store {
	return domain.Store{ID: uuid.New(), Name: "Магнит", Slug: "magnit", IsActive: true}
}

// --- store handler tests ---

func TestListStores_OK(t *testing.T) {
	m := &mockListStores{}
	stores := []domain.Store{fakeStore()}
	m.On("Execute", mock.Anything).Return(stores, nil)

	r := newStoreRouter(handler.NewStoreHandler(m))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/stores", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httputil.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := json.Marshal(resp.Data)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(data, &list))
	assert.Len(t, list, 1)
	assert.Equal(t, "Магнит", list[0]["name"])
}

func TestListStores_InternalError(t *testing.T) {
	m := &mockListStores{}
	m.On("Execute", mock.Anything).Return(nil, assert.AnError)

	r := newStoreRouter(handler.NewStoreHandler(m))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/stores", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- offer handler tests ---

func TestListOffersByProduct_OK(t *testing.T) {
	m := &mockListOffers{}
	productID := uuid.New()
	store := fakeStore()
	offers := []domain.OfferWithStore{
		{
			Offer: domain.Offer{
				ID:           uuid.New(),
				ProductID:    productID,
				StoreID:      store.ID,
				PriceKopecks: 9900,
				InStock:      true,
			},
			Store: store,
		},
	}
	m.On("Execute", mock.Anything, productID, false).Return(offers, nil)

	r := newOfferRouter(handler.NewOfferHandler(m))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/"+productID.String()+"/offers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httputil.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := json.Marshal(resp.Data)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(data, &list))
	assert.Len(t, list, 1)
	assert.Equal(t, float64(9900), list[0]["price_kopecks"])
}

func TestListOffersByProduct_InvalidUUID(t *testing.T) {
	m := &mockListOffers{}

	r := newOfferRouter(handler.NewOfferHandler(m))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/not-a-uuid/offers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListOffersByProduct_NoDiscount_OmitsDiscountFields(t *testing.T) {
	m := &mockListOffers{}
	productID := uuid.New()
	store := fakeStore()
	offers := []domain.OfferWithStore{
		{
			Offer: domain.Offer{
				ID:              uuid.New(),
				ProductID:       productID,
				StoreID:         store.ID,
				PriceKopecks:    9900,
				DiscountPercent: 0,
				InStock:         true,
			},
			Store: store,
		},
	}
	m.On("Execute", mock.Anything, productID, false).Return(offers, nil)

	r := newOfferRouter(handler.NewOfferHandler(m))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/"+productID.String()+"/offers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httputil.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := json.Marshal(resp.Data)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(data, &list))
	require.Len(t, list, 1)
	_, hasDiscount := list[0]["discount_percent"]
	_, hasOriginal := list[0]["original_price_kopecks"]
	assert.False(t, hasDiscount, "discount_percent must be absent when no discount")
	assert.False(t, hasOriginal, "original_price_kopecks must be absent when no discount")
}

func TestListOffersByProduct_WithDiscount_IncludesDiscountFields(t *testing.T) {
	m := &mockListOffers{}
	productID := uuid.New()
	store := fakeStore()
	originalPrice := int64(12000)
	offers := []domain.OfferWithStore{
		{
			Offer: domain.Offer{
				ID:                   uuid.New(),
				ProductID:            productID,
				StoreID:              store.ID,
				PriceKopecks:         9000,
				OriginalPriceKopecks: &originalPrice,
				DiscountPercent:      25,
				InStock:              true,
			},
			Store: store,
		},
	}
	m.On("Execute", mock.Anything, productID, false).Return(offers, nil)

	r := newOfferRouter(handler.NewOfferHandler(m))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/"+productID.String()+"/offers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httputil.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := json.Marshal(resp.Data)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(data, &list))
	require.Len(t, list, 1)
	assert.Equal(t, float64(25), list[0]["discount_percent"])
	assert.Equal(t, float64(12000), list[0]["original_price_kopecks"])
}

func TestListOffersByProduct_HasDiscountFilter_Forwarded(t *testing.T) {
	m := &mockListOffers{}
	productID := uuid.New()
	m.On("Execute", mock.Anything, productID, true).Return([]domain.OfferWithStore{}, nil)

	r := newOfferRouter(handler.NewOfferHandler(m))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/"+productID.String()+"/offers?has_discount=true", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	m.AssertCalled(t, "Execute", mock.Anything, productID, true)
}
