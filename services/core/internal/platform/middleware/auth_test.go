package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/foodsea/core/internal/platform/middleware"
)

const testSecret = "test-secret"

func makeToken(t *testing.T, userID uuid.UUID, ttl time.Duration) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func authRouter(t *testing.T) *gin.Engine {
	t.Helper()
	r := gin.New()
	r.GET("/protected", middleware.Auth(testSecret), func(c *gin.Context) {
		uid, err := middleware.UserIDFromContext(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user_id": uid.String()})
	})
	return r
}

func TestAuth_MissingHeader(t *testing.T) {
	r := authRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protected", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "missing authorization header")
}

func TestAuth_InvalidFormat(t *testing.T) {
	r := authRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid authorization format")
}

func TestAuth_ExpiredToken(t *testing.T) {
	r := authRouter(t)
	token := makeToken(t, uuid.New(), -time.Hour) // already expired
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ValidToken(t *testing.T) {
	r := authRouter(t)
	userID := uuid.New()
	token := makeToken(t, userID, time.Hour)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), userID.String())
}

func TestAuth_WrongSigningKey(t *testing.T) {
	r := authRouter(t)
	// Token signed with a different secret
	claims := jwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte("wrong-secret"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
