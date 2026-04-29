package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

const testSecret = "test-jwt-secret"

func makeToken(t *testing.T, secret string, userID uuid.UUID, exp time.Time) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return tok
}

func TestAuth_ValidToken(t *testing.T) {
	userID := uuid.New()
	token := makeToken(t, testSecret, userID, time.Now().Add(time.Hour))

	r := gin.New()
	r.Use(Auth(testSecret))
	r.GET("/test", func(c *gin.Context) {
		id, err := UserIDFromContext(c.Request.Context())
		require.NoError(t, err)
		assert.Equal(t, userID, id)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_MissingHeader(t *testing.T) {
	r := gin.New()
	r.Use(Auth(testSecret))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_InvalidFormat(t *testing.T) {
	r := gin.New()
	r.Use(Auth(testSecret))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Token abc")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ExpiredToken(t *testing.T) {
	userID := uuid.New()
	token := makeToken(t, testSecret, userID, time.Now().Add(-time.Hour))

	r := gin.New()
	r.Use(Auth(testSecret))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_WrongSecret(t *testing.T) {
	userID := uuid.New()
	token := makeToken(t, "wrong-secret", userID, time.Now().Add(time.Hour))

	r := gin.New()
	r.Use(Auth(testSecret))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
