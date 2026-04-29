package identity

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/foodsea/core/ent"
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
}

func NewModule(deps Deps) *Module {
	userRepo := repository.NewUserRepo(deps.Ent)
	hasher := repository.NewBcryptHasher()
	tokenSvc := repository.NewJWTTokenService(
		deps.JWT.Secret,
		deps.JWT.AccessTTL,
		deps.JWT.RefreshTTL,
		deps.Redis,
	)

	reg := usecase.NewRegister(userRepo, hasher, tokenSvc)
	loginUC := usecase.NewLogin(userRepo, hasher, tokenSvc)
	refUC := usecase.NewRefresh(tokenSvc)
	outUC := usecase.NewLogout(tokenSvc)
	profUC := usecase.NewGetProfile(userRepo)
	onbUC := usecase.NewCompleteOnboarding(userRepo)

	authH := handler.NewAuthHandler(reg, loginUC, refUC, outUC)
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

	protected.POST("/auth/logout", m.authHandler.Logout)

	users := protected.Group("/users")
	users.GET("/me", m.userHandler.Me)
	users.POST("/me/onboarding", m.userHandler.CompleteOnboarding)
}
