package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/search/domain"
	"github.com/foodsea/core/internal/modules/search/handler"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- mock ---

type mockUseCase struct{ mock.Mock }

func (m *mockUseCase) Execute(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error) {
	args := m.Called(ctx, q)
	if v, ok := args.Get(0).(domain.SearchResult); ok {
		return v, args.Error(1)
	}
	return domain.SearchResult{}, args.Error(1)
}

// --- helpers ---

func newRouter(h *handler.SearchHandler) *gin.Engine {
	r := gin.New()
	r.GET("/search", h.Search)
	return r
}

func doGet(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func emptyResult() domain.SearchResult {
	return domain.SearchResult{Items: []domain.SearchResultItem{}, Total: 0}
}

// --- tests ---

func TestSearchHandler_MissingQuery_400(t *testing.T) {
	uc := new(mockUseCase)
	uc.On("Execute", mock.Anything, mock.MatchedBy(func(q domain.SearchQuery) bool {
		return q.Text == ""
	})).Return(emptyResult(), errors.New("ErrInvalidInput: query must be at least 2 characters: "+sherrors.ErrInvalidInput.Error()))

	// Use real usecase behavior: empty text → 400 via ErrInvalidInput from usecase.
	// We simulate the usecase returning ErrInvalidInput.
	uc2 := new(mockUseCase)
	uc2.On("Execute", mock.Anything, mock.Anything).
		Return(emptyResult(), errors.Join(sherrors.ErrInvalidInput, errors.New("query must be at least 2 characters")))

	h := handler.NewSearchHandler(uc2)
	r := newRouter(h)

	w := doGet(r, "/search")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_QueryTooShort_400(t *testing.T) {
	uc := new(mockUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).
		Return(emptyResult(), errors.Join(sherrors.ErrInvalidInput, errors.New("query must be at least 2 characters")))

	h := handler.NewSearchHandler(uc)
	r := newRouter(h)

	// Single ASCII character — 1 rune — too short.
	w := doGet(r, "/search?q=a")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_InvalidSort_400(t *testing.T) {
	uc := new(mockUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).
		Return(emptyResult(), errors.Join(sherrors.ErrInvalidInput, errors.New("invalid sort option")))

	h := handler.NewSearchHandler(uc)
	r := newRouter(h)

	w := doGet(r, "/search?q=ab&sort=invalid")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_MinPriceExceedsMax_400(t *testing.T) {
	uc := new(mockUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).
		Return(emptyResult(), errors.Join(sherrors.ErrInvalidInput, errors.New("min_price cannot exceed max_price")))

	h := handler.NewSearchHandler(uc)
	r := newRouter(h)

	w := doGet(r, "/search?q=ab&min_price=100&max_price=50")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_PageSizeClamped_To100(t *testing.T) {
	uc := new(mockUseCase)
	uc.On("Execute", mock.Anything, mock.MatchedBy(func(q domain.SearchQuery) bool {
		return q.Pagination.PageSize == 100
	})).Return(emptyResult(), nil)

	h := handler.NewSearchHandler(uc)
	r := newRouter(h)

	w := doGet(r, "/search?q=ab&page_size=500")
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	meta, ok := resp["meta"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(100), meta["page_size"])
}

func TestSearchHandler_ValidRequest_200(t *testing.T) {
	uc := new(mockUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).Return(emptyResult(), nil)

	h := handler.NewSearchHandler(uc)
	r := newRouter(h)

	w := doGet(r, "/search?q=молоко")
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	_, ok := resp["data"]
	assert.True(t, ok, "response should have data field")
	_, ok = resp["meta"]
	assert.True(t, ok, "response should have meta field")
}

func TestSearchHandler_InvalidCategoryUUID_400(t *testing.T) {
	uc := new(mockUseCase)
	h := handler.NewSearchHandler(uc)
	r := newRouter(h)

	w := doGet(r, "/search?q=ab&category_id=not-a-uuid")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	uc.AssertNotCalled(t, "Execute")
}

func TestSearchHandler_InvalidMinPrice_400(t *testing.T) {
	uc := new(mockUseCase)
	h := handler.NewSearchHandler(uc)
	r := newRouter(h)

	w := doGet(r, "/search?q=ab&min_price=abc")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	uc.AssertNotCalled(t, "Execute")
}
