package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/foodsea/core/internal/platform/middleware"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/", func(c *gin.Context) {
		id, ok := middleware.RequestIDFromContext(c.Request.Context())
		assert.True(t, ok)
		assert.NotEmpty(t, id)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRequestID_ForwardsExistingHeader(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/", func(c *gin.Context) {
		id, _ := middleware.RequestIDFromContext(c.Request.Context())
		assert.Equal(t, "custom-id-123", id)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "custom-id-123")
	r.ServeHTTP(w, req)

	assert.Equal(t, "custom-id-123", w.Header().Get("X-Request-ID"))
}
