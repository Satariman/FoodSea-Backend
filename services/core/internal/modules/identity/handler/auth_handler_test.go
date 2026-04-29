package handler

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

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/usecase"
	"github.com/foodsea/core/internal/platform/httputil"
	"github.com/foodsea/core/internal/platform/middleware"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func init() {
	gin.SetMode(gin.TestMode)
	registerCustomValidators()
}

// mock use cases for handler tests

type mockRegister struct{ mock.Mock }

func (m *mockRegister) Execute(ctx context.Context, creds domain.Credentials) (usecase.RegisterResult, error) {
	args := m.Called(ctx, creds)
	return args.Get(0).(usecase.RegisterResult), args.Error(1)
}

type mockLogin struct{ mock.Mock }

func (m *mockLogin) Execute(ctx context.Context, creds domain.Credentials) (usecase.LoginResult, error) {
	args := m.Called(ctx, creds)
	return args.Get(0).(usecase.LoginResult), args.Error(1)
}

type mockRefresh struct{ mock.Mock }

func (m *mockRefresh) Execute(ctx context.Context, refreshToken string) (domain.TokenPair, error) {
	args := m.Called(ctx, refreshToken)
	return args.Get(0).(domain.TokenPair), args.Error(1)
}

type mockLogout struct{ mock.Mock }

func (m *mockLogout) Execute(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// helpers

func testPair() domain.TokenPair {
	return domain.TokenPair{
		Access:           "access-tok",
		Refresh:          "refresh-tok",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshExpiresAt: time.Now().Add(720 * time.Hour),
	}
}

func testUser() *domain.User {
	email := "test@example.com"
	return &domain.User{ID: uuid.New(), Email: &email, CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func setupAuthRouter(h *AuthHandler) *gin.Engine {
	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/refresh", h.Refresh)
	r.POST("/auth/logout", middleware.Auth("test-secret"), h.Logout)
	return r
}

func postJSON(t *testing.T, router *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// tests

func TestAuthHandler_Register(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		reg := &mockRegister{}
		u := testUser()
		pair := testPair()
		reg.On("Execute", mock.Anything, mock.Anything).Return(usecase.RegisterResult{User: u, TokenPair: pair}, nil)

		h := NewAuthHandler(reg, &mockLogin{}, &mockRefresh{}, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/register", map[string]any{
			"email": "test@example.com", "password": "password1",
		})

		assert.Equal(t, http.StatusOK, w.Code)
		var resp httputil.Response
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Empty(t, resp.Error)
	})

	t.Run("missing password → 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/register", map[string]any{"email": "x@x.com"})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid JSON → 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{})
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte("bad json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		setupAuthRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("duplicate email → 409", func(t *testing.T) {
		reg := &mockRegister{}
		reg.On("Execute", mock.Anything, mock.Anything).Return(usecase.RegisterResult{}, sherrors.ErrAlreadyExists)

		h := NewAuthHandler(reg, &mockLogin{}, &mockRefresh{}, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/register", map[string]any{
			"email": "dup@example.com", "password": "password1",
		})
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("validation error from use case → 400", func(t *testing.T) {
		reg := &mockRegister{}
		reg.On("Execute", mock.Anything, mock.Anything).Return(usecase.RegisterResult{}, sherrors.ErrInvalidInput)

		h := NewAuthHandler(reg, &mockLogin{}, &mockRefresh{}, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/register", map[string]any{
			"email": "ok@example.com", "password": "password1",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthHandler_Login(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		loginUC := &mockLogin{}
		u := testUser()
		pair := testPair()
		loginUC.On("Execute", mock.Anything, mock.Anything).Return(usecase.LoginResult{User: u, TokenPair: pair}, nil)

		h := NewAuthHandler(&mockRegister{}, loginUC, &mockRefresh{}, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/login", map[string]any{
			"email": "test@example.com", "password": "password1",
		})
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("wrong credentials → 401", func(t *testing.T) {
		loginUC := &mockLogin{}
		loginUC.On("Execute", mock.Anything, mock.Anything).Return(usecase.LoginResult{}, sherrors.ErrUnauthorized)

		h := NewAuthHandler(&mockRegister{}, loginUC, &mockRefresh{}, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/login", map[string]any{
			"email": "x@x.com", "password": "wrongpass",
		})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("missing password → 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/login", map[string]any{"email": "x@x.com"})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthHandler_Refresh(t *testing.T) {
	t.Run("valid refresh token → new pair", func(t *testing.T) {
		refUC := &mockRefresh{}
		refUC.On("Execute", mock.Anything, "my-refresh-token").Return(testPair(), nil)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, refUC, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/refresh", map[string]any{"refresh_token": "my-refresh-token"})

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid refresh token → 401", func(t *testing.T) {
		refUC := &mockRefresh{}
		refUC.On("Execute", mock.Anything, "bad-token").Return(domain.TokenPair{}, sherrors.ErrUnauthorized)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, refUC, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/refresh", map[string]any{"refresh_token": "bad-token"})

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("missing refresh_token → 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{})
		w := postJSON(t, setupAuthRouter(h), "/auth/refresh", map[string]any{})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthHandler_Logout(t *testing.T) {
	t.Run("no auth header → 401", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{})
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		w := httptest.NewRecorder()
		setupAuthRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
