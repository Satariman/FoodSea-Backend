package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	analogsdomain "github.com/foodsea/optimization/internal/modules/analogs/domain"
	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
	"github.com/foodsea/optimization/internal/platform/middleware"
)

type mockRunOptimization struct{ mock.Mock }

func (m *mockRunOptimization) Execute(ctx context.Context, userID uuid.UUID) (*domain.OptimizationResult, error) {
	args := m.Called(ctx, userID)
	res, _ := args.Get(0).(*domain.OptimizationResult)
	return res, args.Error(1)
}

type mockGetResult struct{ mock.Mock }

func (m *mockGetResult) Execute(ctx context.Context, resultID uuid.UUID) (*domain.OptimizationResult, error) {
	args := m.Called(ctx, resultID)
	res, _ := args.Get(0).(*domain.OptimizationResult)
	return res, args.Error(1)
}

type mockGetAnalogs struct{ mock.Mock }

func (m *mockGetAnalogs) Execute(ctx context.Context, productID uuid.UUID, topK int) ([]analogsdomain.Analog, error) {
	args := m.Called(ctx, productID, topK)
	res, _ := args.Get(0).([]analogsdomain.Analog)
	return res, args.Error(1)
}

func TestHandler_RunOptimization_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := &mockRunOptimization{}
	get := &mockGetResult{}
	analogs := &mockGetAnalogs{}
	h := &Handler{runOptimization: run, getResult: get, getAnalogs: analogs, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	r := gin.New()
	r.POST("/optimize", h.RunOptimization)

	req := httptest.NewRequest(http.MethodPost, "/optimize", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_GetResult_OtherUserGets404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := &mockRunOptimization{}
	get := &mockGetResult{}
	analogs := &mockGetAnalogs{}
	h := &Handler{runOptimization: run, getResult: get, getAnalogs: analogs, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	requestUser := uuid.New()
	ownerUser := uuid.New()
	resultID := uuid.New()
	get.On("Execute", mock.Anything, resultID).Return(&domain.OptimizationResult{ID: resultID, UserID: ownerUser}, nil).Once()

	r := gin.New()
	r.GET("/optimize/:id", func(c *gin.Context) {
		c.Request = c.Request.WithContext(middleware.WithUserIDContext(c.Request.Context(), requestUser))
		h.GetResult(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/optimize/"+resultID.String(), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	get.AssertExpectations(t)
}

func TestHandler_RunAndAnalogs_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := &mockRunOptimization{}
	get := &mockGetResult{}
	analogs := &mockGetAnalogs{}
	h := &Handler{runOptimization: run, getResult: get, getAnalogs: analogs, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	userID := uuid.New()
	resultID := uuid.New()
	productID := uuid.New()

	run.On("Execute", mock.Anything, userID).Return(&domain.OptimizationResult{
		ID:              resultID,
		UserID:          userID,
		TotalKopecks:    1000,
		DeliveryKopecks: 100,
		SavingsKopecks:  200,
		Status:          "active",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}, nil).Once()

	analogs.On("Execute", mock.Anything, productID, 5).Return([]analogsdomain.Analog{{
		ProductID:       uuid.New(),
		ProductName:     "Alt",
		Score:           0.9,
		MinPriceKopecks: 350,
	}}, nil).Once()

	r := gin.New()
	r.POST("/optimize", func(c *gin.Context) {
		c.Request = c.Request.WithContext(middleware.WithUserIDContext(c.Request.Context(), userID))
		h.RunOptimization(c)
	})
	r.GET("/analogs/:product_id", func(c *gin.Context) {
		c.Request = c.Request.WithContext(middleware.WithUserIDContext(c.Request.Context(), userID))
		h.GetAnalogs(c)
	})

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodPost, "/optimize", http.NoBody))
	require.Equal(t, http.StatusOK, w1.Code)
	var body1 map[string]any
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &body1))
	require.Contains(t, body1, "data")

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/analogs/"+productID.String(), http.NoBody))
	require.Equal(t, http.StatusOK, w2.Code)

	run.AssertExpectations(t)
	analogs.AssertExpectations(t)
}
