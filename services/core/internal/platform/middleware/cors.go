package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig holds the CORS policy for the service.
type CORSConfig struct {
	AllowedOrigins []string
}

// DefaultCORSConfig returns a config suitable for development + iOS client.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"app://foodsea", "http://localhost:3000", "http://localhost:8080"},
	}
}

// CORS sets cross-origin headers. It handles OPTIONS preflight requests.
func CORS(cfg CORSConfig) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
		} else if len(cfg.AllowedOrigins) > 0 {
			c.Header("Access-Control-Allow-Origin", cfg.AllowedOrigins[0])
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers",
			"Authorization, Content-Type, X-Request-ID")
		c.Header("Access-Control-Allow-Methods",
			strings.Join([]string{
				http.MethodGet, http.MethodPost, http.MethodPut,
				http.MethodDelete, http.MethodOptions,
			}, ", "))

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
