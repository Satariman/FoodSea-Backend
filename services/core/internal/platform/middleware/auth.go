package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/foodsea/core/internal/platform/httputil"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type accessClaims struct {
	jwt.RegisteredClaims
}

// Auth validates a Bearer JWT and puts the user UUID into the request context.
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				httputil.Response{Error: "missing authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				httputil.Response{Error: "invalid authorization format"})
			return
		}

		token, err := jwt.ParseWithClaims(parts[1], &accessClaims{},
			func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			},
			jwt.WithExpirationRequired(),
		)

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				httputil.Response{Error: "invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(*accessClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				httputil.Response{Error: "invalid or expired token"})
			return
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				httputil.Response{Error: "invalid or expired token"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), userIDKey, userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// UserIDFromContext returns the authenticated user UUID from the context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, sherrors.ErrUnauthorized
	}
	return id, nil
}

// WithUserIDContext returns a new context carrying the given user UUID.
// Used in tests and middleware to inject the authenticated user.
func WithUserIDContext(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
