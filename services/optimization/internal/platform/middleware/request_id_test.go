package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequestID_GeneratesIfMissing(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		id, ok := RequestIDFromContext(c.Request.Context())
		assert.True(t, ok)
		assert.NotEmpty(t, id)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	r.ServeHTTP(w, req)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRequestID_ForwardsExisting(t *testing.T) {
	existingID := "custom-request-id-abc"
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		id, ok := RequestIDFromContext(c.Request.Context())
		assert.True(t, ok)
		assert.Equal(t, existingID, id)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Request-ID", existingID)
	r.ServeHTTP(w, req)
	assert.Equal(t, existingID, w.Header().Get("X-Request-ID"))
}
