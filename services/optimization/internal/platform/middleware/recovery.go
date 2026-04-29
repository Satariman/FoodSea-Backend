package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/optimization/internal/platform/httputil"
)

// Recovery catches panics, logs the stack trace, and returns HTTP 500.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(c.Request.Context(), "panic recovered",
					"error", r,
					"stack", string(debug.Stack()),
					"path", c.Request.URL.Path,
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError,
					httputil.Response{Error: "internal server error"})
			}
		}()
		c.Next()
	}
}
