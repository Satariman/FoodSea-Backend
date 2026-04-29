package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/catalog/handler"
	"github.com/foodsea/core/internal/platform/httputil"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func init() { gin.SetMode(gin.TestMode) }

// --- mocks ---

type mockProductGetter struct{ mock.Mock }

func (m *mockProductGetter) Execute(ctx context.Context, id uuid.UUID) (*domain.ProductDetail, error) {
	args := m.Called(ctx, id)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.ProductDetail)
	return v, nil
}

type mockProductLister struct{ mock.Mock }

func (m *mockProductLister) Execute(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int, error) {
	args := m.Called(ctx, filter)
	var err error
	if len(args) > 2 && args[2] != nil {
		err, _ = args[2].(error)
	}
	if err != nil {
		return nil, args.Int(1), err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, args.Int(1), nil
	}
	v, _ := args[0].([]domain.Product)
	return v, args.Int(1), nil
}

type mockBarcodeGetter struct{ mock.Mock }

func (m *mockBarcodeGetter) Execute(ctx context.Context, code string) (*domain.ProductDetail, error) {
	args := m.Called(ctx, code)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.ProductDetail)
	return v, nil
}

// --- helpers ---

func buildRouter(h *handler.ProductHandler) *gin.Engine {
	r := gin.New()
	r.GET("/products", h.ListProducts)
	r.GET("/products/barcode/:code", h.GetProductByBarcode)
	r.GET("/products/:id", h.GetProduct)
	return r
}

func doRequest(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func fakeDetail() *domain.ProductDetail {
	desc := "A description"
	comp := "Ingredients"
	weight := "500 мл"
	barcode := "4607025390015"
	imgURL := "https://img.example.com/product.jpg"

	return &domain.ProductDetail{
		Product: domain.Product{
			ID:          uuid.MustParse("5c8c2afb-b178-450b-9ba4-9394c81a7055"),
			Name:        "Энергетик Burn Classic, 500 мл",
			Description: &desc,
			Composition: &comp,
			Weight:      &weight,
			Barcode:     &barcode,
			ImageURL:    &imgURL,
			InStock:     true,
			CategoryID:  uuid.New(),
		},
		Category:    domain.Category{ID: uuid.New(), Name: "Энергетики"},
		Subcategory: &domain.Category{ID: uuid.New(), Name: "Энергетические напитки"},
		Brand:       &domain.Brand{ID: uuid.New(), Name: "Burn"},
		Nutrition:   &domain.Nutrition{Calories: 46, Protein: 0, Fat: 0, Carbohydrates: 11.5},
	}
}

// --- tests ---

func TestGetProduct_JSONFormat(t *testing.T) {
	getter := &mockProductGetter{}
	lister := &mockProductLister{}
	barcoder := &mockBarcodeGetter{}

	detail := fakeDetail()
	getter.On("Execute", mock.Anything, detail.ID).Return(detail, nil)

	h := handler.NewProductHandler(getter, lister, barcoder)
	r := buildRouter(h)

	w := doRequest(r, http.MethodGet, "/products/"+detail.ID.String())
	assert.Equal(t, http.StatusOK, w.Code)

	var resp httputil.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp.Meta)

	data, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var got handler.ProductDetailResponse
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, detail.ID.String(), got.ID)
	assert.Equal(t, "Энергетик Burn Classic, 500 мл", got.Name)
	assert.NotNil(t, got.Description)
	assert.NotNil(t, got.Subcategory)
	assert.NotNil(t, got.Brand)
	require.NotNil(t, got.Nutrition)
	assert.Equal(t, 46.0, got.Nutrition.Calories)
	assert.Equal(t, 11.5, got.Nutrition.Carbohydrates)
}

func TestGetProduct_InvalidUUID_Returns400(t *testing.T) {
	getter := &mockProductGetter{}
	h := handler.NewProductHandler(getter, &mockProductLister{}, &mockBarcodeGetter{})
	r := buildRouter(h)

	w := doRequest(r, http.MethodGet, "/products/not-a-uuid")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetProduct_NotFound_Returns404(t *testing.T) {
	getter := &mockProductGetter{}
	id := uuid.New()
	getter.On("Execute", mock.Anything, id).Return(nil, sherrors.ErrNotFound)

	h := handler.NewProductHandler(getter, &mockProductLister{}, &mockBarcodeGetter{})
	r := buildRouter(h)

	w := doRequest(r, http.MethodGet, "/products/"+id.String())
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListProducts_Pagination(t *testing.T) {
	getter := &mockProductGetter{}
	lister := &mockProductLister{}
	barcoder := &mockBarcodeGetter{}

	lister.On("Execute", mock.Anything, mock.Anything).Return([]domain.Product{}, 0, nil)

	h := handler.NewProductHandler(getter, lister, barcoder)
	r := buildRouter(h)

	w := doRequest(r, http.MethodGet, "/products?page=2&page_size=20")
	assert.Equal(t, http.StatusOK, w.Code)

	var resp httputil.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Meta)
	assert.Equal(t, 2, resp.Meta.Page)
	assert.Equal(t, 20, resp.Meta.PageSize)
	assert.Equal(t, 0, resp.Meta.TotalCount)
}

func TestGetProduct_InternalError_Returns500(t *testing.T) {
	getter := &mockProductGetter{}
	id := uuid.New()
	getter.On("Execute", mock.Anything, id).Return(nil, errors.New("db failure"))

	h := handler.NewProductHandler(getter, &mockProductLister{}, &mockBarcodeGetter{})
	r := buildRouter(h)

	w := doRequest(r, http.MethodGet, "/products/"+id.String())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListProducts_JSONExport(t *testing.T) {
	getter := &mockProductGetter{}
	lister := &mockProductLister{}

	products := []domain.Product{
		{ID: uuid.New(), Name: "Product A", InStock: true},
	}
	lister.On("Execute", mock.Anything, mock.Anything).Return(products, 1, nil)

	h := handler.NewProductHandler(getter, lister, &mockBarcodeGetter{})
	r := buildRouter(h)

	w := doRequest(r, http.MethodGet, "/products")
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	_, hasMeta := body["meta"]
	assert.True(t, hasMeta, "list response must include meta")
}
