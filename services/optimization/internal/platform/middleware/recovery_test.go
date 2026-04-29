package middleware_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/foodsea/optimization/internal/platform/middleware"
)

func init() { gin.SetMode(gin.TestMode) }

func TestRecovery_PanicReturns500(t *testing.T) {
	log := slog.Default()
	r := gin.New()
	r.Use(middleware.Recovery(log))
	r.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRecovery_NormalRequest(t *testing.T) {
	log := slog.Default()
	r := gin.New()
	r.Use(middleware.Recovery(log))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
