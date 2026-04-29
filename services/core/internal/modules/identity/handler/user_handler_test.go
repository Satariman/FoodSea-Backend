package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/platform/middleware"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

const testSecret = "test-secret"

type mockGetProfile struct{ mock.Mock }

func (m *mockGetProfile) Execute(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

type mockCompleteOnboarding struct{ mock.Mock }

func (m *mockCompleteOnboarding) Execute(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func setupUserRouter(h *UserHandler) *gin.Engine {
	r := gin.New()
	protected := r.Group("/", middleware.Auth(testSecret))
	protected.GET("/users/me", h.Me)
	protected.POST("/users/me/onboarding", h.CompleteOnboarding)
	return r
}

func makeToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ID:        uuid.NewString(),
	})
	tok, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return tok
}

func TestUserHandler_Me(t *testing.T) {
	t.Run("returns profile with valid token", func(t *testing.T) {
		prof := &mockGetProfile{}
		userID := uuid.New()
		email := "me@example.com"
		u := &domain.User{ID: userID, Email: &email, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		prof.On("Execute", mock.Anything, userID).Return(u, nil)

		h := NewUserHandler(prof, &mockCompleteOnboarding{})
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", makeToken(t, userID)))
		w := httptest.NewRecorder()
		setupUserRouter(h).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		data := resp["data"].(map[string]any)
		assert.Equal(t, userID.String(), data["id"])
		assert.Nil(t, data["password_hash"], "password_hash must not appear in response")
	})

	t.Run("no token → 401", func(t *testing.T) {
		h := NewUserHandler(&mockGetProfile{}, &mockCompleteOnboarding{})
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		w := httptest.NewRecorder()
		setupUserRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("user not found in db → 404", func(t *testing.T) {
		prof := &mockGetProfile{}
		userID := uuid.New()
		prof.On("Execute", mock.Anything, userID).Return(nil, sherrors.ErrNotFound)

		h := NewUserHandler(prof, &mockCompleteOnboarding{})
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", makeToken(t, userID)))
		w := httptest.NewRecorder()
		setupUserRouter(h).ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestUserHandler_CompleteOnboarding(t *testing.T) {
	t.Run("success → 204", func(t *testing.T) {
		onb := &mockCompleteOnboarding{}
		userID := uuid.New()
		onb.On("Execute", mock.Anything, userID).Return(nil)

		h := NewUserHandler(&mockGetProfile{}, onb)
		req := httptest.NewRequest(http.MethodPost, "/users/me/onboarding", nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", makeToken(t, userID)))
		w := httptest.NewRecorder()
		setupUserRouter(h).ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("idempotent — second call also 204", func(t *testing.T) {
		onb := &mockCompleteOnboarding{}
		userID := uuid.New()
		onb.On("Execute", mock.Anything, userID).Return(nil).Times(2)

		h := NewUserHandler(&mockGetProfile{}, onb)
		router := setupUserRouter(h)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodPost, "/users/me/onboarding", nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", makeToken(t, userID)))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNoContent, w.Code)
		}
		onb.AssertExpectations(t)
	})

	t.Run("no token → 401", func(t *testing.T) {
		h := NewUserHandler(&mockGetProfile{}, &mockCompleteOnboarding{})
		req := httptest.NewRequest(http.MethodPost, "/users/me/onboarding", nil)
		w := httptest.NewRecorder()
		setupUserRouter(h).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
