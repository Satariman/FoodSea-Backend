package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID injects an X-Request-ID into the request context and response header.
// If the header is already present it is forwarded as-is; otherwise a new UUID is generated.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		ctx := context.WithValue(c.Request.Context(), requestIDKey, id)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}
