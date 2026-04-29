package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger logs each request after completion. Level depends on status code:
// 5xx → ERROR, 4xx → WARN, 2xx/3xx → INFO.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []any{
			"status", status,
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		}

		if reqID, ok := c.Request.Context().Value(requestIDKey).(string); ok {
			attrs = append(attrs, "request_id", reqID)
		}

		switch {
		case status >= 500:
			log.ErrorContext(c.Request.Context(), "request", attrs...)
		case status >= 400:
			log.WarnContext(c.Request.Context(), "request", attrs...)
		default:
			log.InfoContext(c.Request.Context(), "request", attrs...)
		}
	}
}
