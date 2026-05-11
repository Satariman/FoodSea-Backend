package identity

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/handler"
	"github.com/foodsea/core/internal/modules/identity/repository"
	"github.com/foodsea/core/internal/modules/identity/usecase"
	"github.com/foodsea/core/internal/platform/cache"
	"github.com/foodsea/core/internal/platform/config"
)

type Deps struct {
	Ent   *ent.Client
	Redis *redis.Client
	Cache cache.Cache
	Log   *slog.Logger
	JWT   config.JWTConfig
	OAuth config.OAuthConfig

	HTTPClient *http.Client
}

type Module struct {
	register           *usecase.Register
	login              *usecase.Login
	refresh            *usecase.Refresh
	logout             *usecase.Logout
	getProfile         *usecase.GetProfile
	completeOnboarding *usecase.CompleteOnboarding
	authHandler        *handler.AuthHandler
	userHandler        *handler.UserHandler
	legacyOAuthEnabled bool
	nativeOAuthEnabled bool
	yandexSDKEnabled   bool
}

func NewModule(deps Deps) *Module {
	httpClient := deps.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	userRepo := repository.NewUserRepo(deps.Ent)
	oauthIdentityRepo := repository.NewOAuthIdentityRepo(deps.Ent)
	stateStore := repository.NewOAuthStateStore(deps.Redis, deps.OAuth.StateTTL)
	hasher := repository.NewBcryptHasher()
	tokenSvc := repository.NewJWTTokenService(
		deps.JWT.Secret,
		deps.JWT.AccessTTL,
		deps.JWT.RefreshTTL,
		deps.Redis,
	)
	providers := make([]domain.OAuthProvider, 0, 2)
	var googleLegacyProvider *repository.GoogleOAuthProvider
	if deps.OAuth.Google.Enabled {
		googleLegacyProvider = repository.NewGoogleOAuthProvider(deps.OAuth.Google, httpClient)
	}
	var googleNativeProvider *repository.GoogleOAuthProvider
	if deps.OAuth.GoogleNative.Enabled {
		googleNativeProvider = repository.NewGoogleOAuthProvider(deps.OAuth.GoogleNative, httpClient)
	}
	if googleLegacyProvider != nil || googleNativeProvider != nil {
		providers = append(providers, repository.NewGoogleDualOAuthProvider(googleLegacyProvider, googleNativeProvider))
	}

	if deps.OAuth.Yandex.Enabled {
		providers = append(providers, repository.NewYandexOAuthProvider(deps.OAuth.Yandex, httpClient))
	}

	reg := usecase.NewRegister(userRepo, hasher, tokenSvc)
	loginUC := usecase.NewLogin(userRepo, hasher, tokenSvc)
	refUC := usecase.NewRefresh(tokenSvc)
	outUC := usecase.NewLogout(tokenSvc)
	oauthStartUC := usecase.NewOAuthStart(
		stateStore,
		providers,
		deps.OAuth.AllowedRedirectURIs,
		deps.OAuth.NativeAllowedRedirectURIs,
		deps.OAuth.StateTTL,
	)
	oauthCallbackUC := usecase.NewOAuthCallback(stateStore, providers, oauthIdentityRepo, userRepo, tokenSvc)
	profUC := usecase.NewGetProfile(userRepo)
	onbUC := usecase.NewCompleteOnboarding(userRepo)

	authH := handler.NewAuthHandler(reg, loginUC, refUC, outUC, oauthStartUC, oauthCallbackUC)
	userH := handler.NewUserHandler(profUC, onbUC)

	return &Module{
		register:           reg,
		login:              loginUC,
		refresh:            refUC,
		logout:             outUC,
		getProfile:         profUC,
		completeOnboarding: onbUC,
		authHandler:        authH,
		userHandler:        userH,
		legacyOAuthEnabled: deps.OAuth.LegacyEnabled,
		nativeOAuthEnabled: deps.OAuth.NativeEnabled,
		yandexSDKEnabled:   deps.OAuth.YandexNativeSDKEnabled,
	}
}

// RegisterRoutes mounts identity routes.
// public — unauthenticated routes (register, login, refresh).
// protected — routes that require auth middleware applied by the caller.
func (m *Module) RegisterRoutes(public, protected *gin.RouterGroup) {
	auth := public.Group("/auth")
	auth.POST("/register", m.authHandler.Register)
	auth.POST("/login", m.authHandler.Login)
	auth.POST("/refresh", m.authHandler.Refresh)
	if m.legacyOAuthEnabled {
		auth.GET("/oauth/:provider/start", m.authHandler.OAuthStart)
		auth.POST("/oauth/:provider/callback", m.authHandler.OAuthCallback)
	}
	if m.nativeOAuthEnabled {
		auth.GET("/oauth/native/:provider/start", m.authHandler.OAuthNativeStart)
		auth.POST("/oauth/native/:provider/callback", m.authHandler.OAuthNativeCallback)
		if m.yandexSDKEnabled {
			auth.POST("/oauth/native/:provider/sdk/callback", m.authHandler.OAuthNativeSDKCallback)
		}
	}

	protected.POST("/auth/logout", m.authHandler.Logout)

	users := protected.Group("/users")
	users.GET("/me", m.userHandler.Me)
	users.POST("/me/onboarding", m.userHandler.CompleteOnboarding)
}
