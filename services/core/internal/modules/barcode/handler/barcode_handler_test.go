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

	"github.com/foodsea/core/internal/modules/barcode/handler"
	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	catalogdto "github.com/foodsea/core/internal/modules/catalog/handler"
	"github.com/foodsea/core/internal/platform/httputil"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func init() { gin.SetMode(gin.TestMode) }

// --- mock ---

type mockGetter struct{ mock.Mock }

func (m *mockGetter) ByBarcode(ctx context.Context, code string) (*catalogdomain.ProductDetail, error) {
	args := m.Called(ctx, code)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if args.Get(0) == nil {
		return nil, nil
	}
	v, _ := args.Get(0).(*catalogdomain.ProductDetail)
	return v, nil
}

// --- helpers ---

func render(d *catalogdomain.ProductDetail) catalogdto.ProductDetailResponse {
	return catalogdto.ToProductDetailResponse(d)
}

func newRouter(g *mockGetter) *gin.Engine {
	r := gin.New()
	h := handler.New(g, render)
	r.GET("/barcode/:code", h.GetByBarcode)
	return r
}

func doGet(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func fakeDetail() *catalogdomain.ProductDetail {
	name := "Молоко Простоквашино 3.2%, 1 л"
	barcode := "4600494006635"
	return &catalogdomain.ProductDetail{
		Product: catalogdomain.Product{
			ID:      uuid.MustParse("1a2b3c4d-0000-0000-0000-000000000001"),
			Name:    name,
			Barcode: &barcode,
			InStock: true,
		},
		Category: catalogdomain.Category{ID: uuid.New(), Name: "Молочные продукты"},
	}
}

// --- tests ---

func TestGetByBarcode_ValidCode_Returns200(t *testing.T) {
	g := new(mockGetter)
	detail := fakeDetail()
	g.On("ByBarcode", mock.Anything, "4600494006635").Return(detail, nil)

	r := newRouter(g)
	w := doGet(r, "/barcode/4600494006635")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httputil.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var got catalogdto.ProductDetailResponse
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, detail.ID.String(), got.ID)
	assert.Equal(t, detail.Name, got.Name)
	assert.NotNil(t, got.Barcode)
	g.AssertExpectations(t)
}

func TestGetByBarcode_ErrInvalidInput_Returns400(t *testing.T) {
	g := new(mockGetter)
	g.On("ByBarcode", mock.Anything, mock.Anything).Return(nil, sherrors.ErrInvalidInput)

	r := newRouter(g)
	w := doGet(r, "/barcode/000")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	g.AssertExpectations(t)
}

func TestGetByBarcode_ValidationError_Returns422(t *testing.T) {
	g := new(mockGetter)
	ve := &sherrors.ValidationError{Field: "barcode", Message: "must be a valid EAN-8 or EAN-13"}
	g.On("ByBarcode", mock.Anything, mock.Anything).Return(nil, ve)

	r := newRouter(g)
	w := doGet(r, "/barcode/00000000000")

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	g.AssertExpectations(t)
}

func TestGetByBarcode_ErrNotFound_Returns404(t *testing.T) {
	g := new(mockGetter)
	g.On("ByBarcode", mock.Anything, mock.Anything).Return(nil, sherrors.ErrNotFound)

	r := newRouter(g)
	w := doGet(r, "/barcode/4600494006635")

	assert.Equal(t, http.StatusNotFound, w.Code)
	g.AssertExpectations(t)
}

func TestGetByBarcode_GenericError_Returns500(t *testing.T) {
	g := new(mockGetter)
	g.On("ByBarcode", mock.Anything, mock.Anything).Return(nil, errors.New("db connection lost"))

	r := newRouter(g)
	w := doGet(r, "/barcode/4600494006635")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	g.AssertExpectations(t)
}
