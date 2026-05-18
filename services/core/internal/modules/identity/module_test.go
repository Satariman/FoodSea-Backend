package identity

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/foodsea/core/internal/modules/identity/handler"
)

func TestModule_RegisterRoutes_OAuthFeatureFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newModule := func(legacyEnabled, nativeEnabled, yandexSDKEnabled bool) *Module {
		return &Module{
			authHandler:        handler.NewAuthHandler(nil, nil, nil, nil, nil, nil),
			userHandler:        handler.NewUserHandler(nil, nil),
			legacyOAuthEnabled: legacyEnabled,
			nativeOAuthEnabled: nativeEnabled,
			yandexSDKEnabled:   yandexSDKEnabled,
		}
	}

	hasRoute := func(routes []gin.RouteInfo, method, path string) bool {
		for _, r := range routes {
			if r.Method == method && r.Path == path {
				return true
			}
		}
		return false
	}

	t.Run("legacy on native off", func(t *testing.T) {
		r := gin.New()
		api := r.Group("/api/v1")
		m := newModule(true, false, false)
		m.RegisterRoutes(api, api)

		routes := r.Routes()
		assert.True(t, hasRoute(routes, "GET", "/api/v1/auth/oauth/:provider/start"))
		assert.True(t, hasRoute(routes, "POST", "/api/v1/auth/oauth/:provider/callback"))
		assert.False(t, hasRoute(routes, "GET", "/api/v1/auth/oauth/native/:provider/start"))
		assert.False(t, hasRoute(routes, "POST", "/api/v1/auth/oauth/native/apple/callback"))
		assert.False(t, hasRoute(routes, "POST", "/api/v1/auth/oauth/native/:provider/callback"))
		assert.False(t, hasRoute(routes, "POST", "/api/v1/auth/oauth/native/:provider/sdk/callback"))
	})

	t.Run("native on legacy off with yandex sdk", func(t *testing.T) {
		r := gin.New()
		api := r.Group("/api/v1")
		m := newModule(false, true, true)
		m.RegisterRoutes(api, api)

		routes := r.Routes()
		assert.False(t, hasRoute(routes, "GET", "/api/v1/auth/oauth/:provider/start"))
		assert.False(t, hasRoute(routes, "POST", "/api/v1/auth/oauth/:provider/callback"))
		assert.True(t, hasRoute(routes, "GET", "/api/v1/auth/oauth/native/:provider/start"))
		assert.True(t, hasRoute(routes, "POST", "/api/v1/auth/oauth/native/apple/callback"))
		assert.True(t, hasRoute(routes, "POST", "/api/v1/auth/oauth/native/:provider/callback"))
		assert.True(t, hasRoute(routes, "POST", "/api/v1/auth/oauth/native/:provider/sdk/callback"))
	})

	// sanity: route registration should not panic
	t.Run("route registration no panic", func(t *testing.T) {
		r := gin.New()
		api := r.Group("/api/v1")
		m := newModule(true, true, true)
		m.RegisterRoutes(api, api)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		r.ServeHTTP(w, req)
	})
}
