package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/foodsea/core/internal/platform/middleware"
)

func newBufLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestLogger_2xx_IsInfo(t *testing.T) {
	buf := &bytes.Buffer{}
	r := gin.New()
	r.Use(middleware.Logger(newBufLogger(buf)))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	assert.True(t, strings.Contains(buf.String(), "INFO"), "2xx should log at INFO level")
}

func TestLogger_4xx_IsWarn(t *testing.T) {
	buf := &bytes.Buffer{}
	r := gin.New()
	r.Use(middleware.Logger(newBufLogger(buf)))
	r.GET("/bad", func(c *gin.Context) { c.Status(http.StatusBadRequest) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bad", nil))

	assert.True(t, strings.Contains(buf.String(), "WARN"), "4xx should log at WARN level")
}

func TestLogger_5xx_IsError(t *testing.T) {
	buf := &bytes.Buffer{}
	r := gin.New()
	r.Use(middleware.Logger(newBufLogger(buf)))
	r.GET("/err", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/err", nil))

	assert.True(t, strings.Contains(buf.String(), "ERROR"), "5xx should log at ERROR level")
}
