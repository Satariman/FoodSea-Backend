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

type mockOAuthStart struct{ mock.Mock }

func (m *mockOAuthStart) Execute(ctx context.Context, req domain.OAuthStartRequest) (domain.OAuthStartResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return domain.OAuthStartResult{}, args.Error(1)
	}
	return args.Get(0).(domain.OAuthStartResult), args.Error(1)
}

type mockOAuthCallback struct{ mock.Mock }

func (m *mockOAuthCallback) Execute(ctx context.Context, req domain.OAuthCallbackRequest) (domain.OAuthCallbackResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return domain.OAuthCallbackResult{}, args.Error(1)
	}
	return args.Get(0).(domain.OAuthCallbackResult), args.Error(1)
}

func (m *mockOAuthCallback) ExecuteToken(ctx context.Context, req domain.OAuthTokenCallbackRequest) (domain.OAuthTokenCallbackResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return domain.OAuthTokenCallbackResult{}, args.Error(1)
	}
	return args.Get(0).(domain.OAuthTokenCallbackResult), args.Error(1)
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

func ptr(s string) *string {
	return &s
}

func setupAuthRouter(h *AuthHandler) *gin.Engine {
	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/refresh", h.Refresh)
	r.GET("/auth/oauth/:provider/start", h.OAuthStart)
	r.POST("/auth/oauth/:provider/callback", h.OAuthCallback)
	r.GET("/auth/oauth/native/:provider/start", h.OAuthNativeStart)
	r.POST("/auth/oauth/native/:provider/callback", h.OAuthNativeCallback)
	r.POST("/auth/oauth/native/:provider/sdk/callback", h.OAuthNativeSDKCallback)
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

		h := NewAuthHandler(reg, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/register", map[string]any{
			"email": "test@example.com", "password": "password1",
		})

		assert.Equal(t, http.StatusOK, w.Code)
		var resp httputil.Response
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Empty(t, resp.Error)
	})

	t.Run("missing password → 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/register", map[string]any{"email": "x@x.com"})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid JSON → 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte("bad json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		setupAuthRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("duplicate email → 409", func(t *testing.T) {
		reg := &mockRegister{}
		reg.On("Execute", mock.Anything, mock.Anything).Return(usecase.RegisterResult{}, sherrors.ErrAlreadyExists)

		h := NewAuthHandler(reg, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/register", map[string]any{
			"email": "dup@example.com", "password": "password1",
		})
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("validation error from use case → 400", func(t *testing.T) {
		reg := &mockRegister{}
		reg.On("Execute", mock.Anything, mock.Anything).Return(usecase.RegisterResult{}, sherrors.ErrInvalidInput)

		h := NewAuthHandler(reg, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
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

		h := NewAuthHandler(&mockRegister{}, loginUC, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/login", map[string]any{
			"email": "test@example.com", "password": "password1",
		})
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("wrong credentials → 401", func(t *testing.T) {
		loginUC := &mockLogin{}
		loginUC.On("Execute", mock.Anything, mock.Anything).Return(usecase.LoginResult{}, sherrors.ErrUnauthorized)

		h := NewAuthHandler(&mockRegister{}, loginUC, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/login", map[string]any{
			"email": "x@x.com", "password": "wrongpass",
		})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("missing password → 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/login", map[string]any{"email": "x@x.com"})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthHandler_Refresh(t *testing.T) {
	t.Run("valid refresh token → new pair", func(t *testing.T) {
		refUC := &mockRefresh{}
		refUC.On("Execute", mock.Anything, "my-refresh-token").Return(testPair(), nil)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, refUC, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/refresh", map[string]any{"refresh_token": "my-refresh-token"})

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid refresh token → 401", func(t *testing.T) {
		refUC := &mockRefresh{}
		refUC.On("Execute", mock.Anything, "bad-token").Return(domain.TokenPair{}, sherrors.ErrUnauthorized)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, refUC, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/refresh", map[string]any{"refresh_token": "bad-token"})

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("missing refresh_token → 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/refresh", map[string]any{})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthHandler_Logout(t *testing.T) {
	t.Run("no auth header → 401", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		w := httptest.NewRecorder()
		setupAuthRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestAuthHandler_OAuthStart(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		oauthStart := &mockOAuthStart{}
		oauthStart.On("Execute", mock.Anything, domain.OAuthStartRequest{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "https://app/cb",
			Mode:       domain.OAuthFlowModeLegacy,
		}).Return(domain.OAuthStartResult{
			AuthURL: "https://accounts.google.com/auth?state=s1",
			State:   "s1",
		}, nil)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, oauthStart, &mockOAuthCallback{})
		req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/start?redirect_uri=https://app/cb", nil)
		w := httptest.NewRecorder()
		setupAuthRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("bad provider => 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		req := httptest.NewRequest(http.MethodGet, "/auth/oauth/invalid/start?redirect_uri=https://app/cb", nil)
		w := httptest.NewRecorder()
		setupAuthRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("start usecase invalid input => 400", func(t *testing.T) {
		oauthStart := &mockOAuthStart{}
		oauthStart.On("Execute", mock.Anything, mock.Anything).Return(domain.OAuthStartResult{}, sherrors.ErrInvalidInput)
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, oauthStart, &mockOAuthCallback{})
		req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/start?redirect_uri=bad", nil)
		w := httptest.NewRecorder()
		setupAuthRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthHandler_OAuthNativeStart(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		oauthStart := &mockOAuthStart{}
		oauthStart.On("Execute", mock.Anything, domain.OAuthStartRequest{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "foodsea://oauth/callback",
			Mode:       domain.OAuthFlowModeNative,
		}).Return(domain.OAuthStartResult{
			AuthURL: "https://accounts.google.com/auth?state=s1",
			State:   "s1",
		}, nil)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, oauthStart, &mockOAuthCallback{})
		req := httptest.NewRequest(http.MethodGet, "/auth/oauth/native/google/start?redirect_uri=foodsea://oauth/callback", nil)
		w := httptest.NewRecorder()
		setupAuthRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAuthHandler_OAuthCallback(t *testing.T) {
	t.Run("callback success => 200", func(t *testing.T) {
		oauthCallback := &mockOAuthCallback{}
		u := testUser()
		pair := testPair()
		oauthCallback.On("Execute", mock.Anything, domain.OAuthCallbackRequest{
			Provider:    domain.OAuthProviderGoogle,
			Code:        "code-1",
			State:       "state-1",
			RedirectURI: "https://app/cb",
			Mode:        domain.OAuthFlowModeLegacy,
		}).Return(domain.OAuthCallbackResult{
			User:      u,
			TokenPair: pair,
		}, nil)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, oauthCallback)
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/google/callback", map[string]any{
			"code":         "code-1",
			"state":        "state-1",
			"redirect_uri": "https://app/cb",
		})
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("callback bad provider => 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/notreal/callback", map[string]any{
			"code":         "code-1",
			"state":        "state-1",
			"redirect_uri": "https://app/cb",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("callback missing fields => 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/google/callback", map[string]any{
			"code": "code-1",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("callback conflict => 409", func(t *testing.T) {
		oauthCallback := &mockOAuthCallback{}
		oauthCallback.On("Execute", mock.Anything, mock.Anything).Return(domain.OAuthCallbackResult{}, sherrors.ErrConflict)
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, oauthCallback)
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/google/callback", map[string]any{
			"code":         "code-1",
			"state":        "state-1",
			"redirect_uri": "https://app/cb",
		})
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("callback unauthorized => 401", func(t *testing.T) {
		oauthCallback := &mockOAuthCallback{}
		oauthCallback.On("Execute", mock.Anything, mock.Anything).Return(domain.OAuthCallbackResult{}, sherrors.ErrUnauthorized)
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, oauthCallback)
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/google/callback", map[string]any{
			"code":         "code-1",
			"state":        "state-1",
			"redirect_uri": "https://app/cb",
		})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestAuthHandler_OAuthNativeCallback(t *testing.T) {
	t.Run("callback success => 200", func(t *testing.T) {
		oauthCallback := &mockOAuthCallback{}
		u := testUser()
		pair := testPair()
		oauthCallback.On("Execute", mock.Anything, domain.OAuthCallbackRequest{
			Provider:    domain.OAuthProviderGoogle,
			Code:        "code-1",
			State:       "state-1",
			RedirectURI: "foodsea://oauth/callback",
			Mode:        domain.OAuthFlowModeNative,
		}).Return(domain.OAuthCallbackResult{
			User:      u,
			TokenPair: pair,
		}, nil)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, oauthCallback)
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/native/google/callback", map[string]any{
			"code":         "code-1",
			"state":        "state-1",
			"redirect_uri": "foodsea://oauth/callback",
		})
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("apple callback uses token flow => 200", func(t *testing.T) {
		oauthCallback := &mockOAuthCallback{}
		u := testUser()
		pair := testPair()
		oauthCallback.On("ExecuteToken", mock.Anything, domain.OAuthTokenCallbackRequest{
			Provider:    domain.OAuthProviderApple,
			AccessToken: "apple-identity-token",
			FullName:    ptr("Ivan Ivanov"),
			Email:       ptr("apple-user@example.com"),
		}).Return(domain.OAuthTokenCallbackResult{
			User:      u,
			TokenPair: pair,
		}, nil)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, oauthCallback)
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/native/apple/callback", map[string]any{
			"identity_token": "apple-identity-token",
			"full_name":      "Ivan Ivanov",
			"email":          "apple-user@example.com",
		})
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("apple callback missing identity_token => 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/native/apple/callback", map[string]any{
			"full_name": "Ivan Ivanov",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAuthHandler_OAuthNativeSDKCallback(t *testing.T) {
	t.Run("sdk callback success => 200", func(t *testing.T) {
		oauthCallback := &mockOAuthCallback{}
		u := testUser()
		pair := testPair()
		oauthCallback.On("ExecuteToken", mock.Anything, domain.OAuthTokenCallbackRequest{
			Provider:    domain.OAuthProviderYandex,
			AccessToken: "sdk-token",
		}).Return(domain.OAuthTokenCallbackResult{
			User:      u,
			TokenPair: pair,
		}, nil)

		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, oauthCallback)
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/native/yandex/sdk/callback", map[string]any{
			"access_token": "sdk-token",
		})
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("sdk callback missing token => 400", func(t *testing.T) {
		h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, &mockOAuthStart{}, &mockOAuthCallback{})
		w := postJSON(t, setupAuthRouter(h), "/auth/oauth/native/yandex/sdk/callback", map[string]any{})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
