package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/usecase"
	"github.com/foodsea/core/internal/platform/httputil"
	"github.com/foodsea/core/internal/platform/middleware"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type registerUseCase interface {
	Execute(ctx context.Context, creds domain.Credentials) (usecase.RegisterResult, error)
}

type loginUseCase interface {
	Execute(ctx context.Context, creds domain.Credentials) (usecase.LoginResult, error)
}

type refreshUseCase interface {
	Execute(ctx context.Context, refreshToken string) (domain.TokenPair, error)
}

type logoutUseCase interface {
	Execute(ctx context.Context, userID uuid.UUID) error
}

type oauthStartUseCase interface {
	Execute(ctx context.Context, req domain.OAuthStartRequest) (domain.OAuthStartResult, error)
}

type oauthCallbackUseCase interface {
	Execute(ctx context.Context, req domain.OAuthCallbackRequest) (domain.OAuthCallbackResult, error)
	ExecuteToken(ctx context.Context, req domain.OAuthTokenCallbackRequest) (domain.OAuthTokenCallbackResult, error)
}

type AuthHandler struct {
	register      registerUseCase
	login         loginUseCase
	refresh       refreshUseCase
	logout        logoutUseCase
	oauthStart    oauthStartUseCase
	oauthCallback oauthCallbackUseCase
}

func NewAuthHandler(
	register registerUseCase,
	login loginUseCase,
	refresh refreshUseCase,
	logout logoutUseCase,
	oauthStart oauthStartUseCase,
	oauthCallback oauthCallbackUseCase,
) *AuthHandler {
	return &AuthHandler{
		register:      register,
		login:         login,
		refresh:       refresh,
		logout:        logout,
		oauthStart:    oauthStart,
		oauthCallback: oauthCallback,
	}
}

// Register godoc
// @Summary      Register a new user
// @Description  Creates a new user account with phone or email and returns JWT tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body RegisterRequest true "Registration credentials"
// @Success      200  {object} httputil.Response{data=AuthResponse}
// @Failure      400  {object} httputil.Response
// @Failure      409  {object} httputil.Response
// @Failure      500  {object} httputil.Response
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	result, err := h.register.Execute(c.Request.Context(), domain.Credentials{
		Phone:    req.Phone,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toAuthResponse(result.User, result.TokenPair))
}

// Login godoc
// @Summary      Login
// @Description  Authenticates a user and returns JWT tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "Login credentials"
// @Success      200  {object} httputil.Response{data=AuthResponse}
// @Failure      400  {object} httputil.Response
// @Failure      401  {object} httputil.Response
// @Failure      500  {object} httputil.Response
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	result, err := h.login.Execute(c.Request.Context(), domain.Credentials{
		Phone:    req.Phone,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toAuthResponse(result.User, result.TokenPair))
}

// Refresh godoc
// @Summary      Refresh tokens
// @Description  Exchanges a valid refresh token for a new token pair
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body RefreshRequest true "Refresh token"
// @Success      200  {object} httputil.Response{data=TokenPairResponse}
// @Failure      400  {object} httputil.Response
// @Failure      401  {object} httputil.Response
// @Failure      500  {object} httputil.Response
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	pair, err := h.refresh.Execute(c.Request.Context(), req.RefreshToken)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toTokenPairResponse(pair))
}

// Logout godoc
// @Summary      Logout
// @Description  Revokes all refresh tokens for the current user
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      204
// @Failure      401  {object} httputil.Response
// @Failure      500  {object} httputil.Response
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, sherrors.ErrUnauthorized)
		return
	}

	if err := h.logout.Execute(c.Request.Context(), userID); err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.NoContent(c)
}

// OAuthStart godoc
// @Summary      Start OAuth flow
// @Description  Builds provider auth URL and creates a state token
// @Tags         auth
// @Produce      json
// @Param        provider path string true "OAuth provider"
// @Param        redirect_uri query string true "Redirect URI"
// @Success      200 {object} httputil.Response{data=OAuthStartResponse}
// @Failure      400 {object} httputil.Response
// @Failure      500 {object} httputil.Response
// @Router       /auth/oauth/{provider}/start [get]
func (h *AuthHandler) OAuthStart(c *gin.Context) {
	provider, err := domain.ParseOAuthProviderName(c.Param("provider"))
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	redirectURI := c.Query("redirect_uri")
	result, err := h.oauthStart.Execute(c.Request.Context(), domain.OAuthStartRequest{
		Provider:   provider,
		RedirectTo: redirectURI,
		Mode:       domain.OAuthFlowModeLegacy,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, OAuthStartResponse{
		AuthURL: result.AuthURL,
		State:   result.State,
	})
}

// OAuthNativeStart godoc
// @Summary      Start native OAuth flow
// @Description  Builds provider auth URL and creates a native state token
// @Tags         auth
// @Produce      json
// @Param        provider path string true "OAuth provider"
// @Param        redirect_uri query string true "Native redirect URI"
// @Success      200 {object} httputil.Response{data=OAuthStartResponse}
// @Failure      400 {object} httputil.Response
// @Failure      500 {object} httputil.Response
// @Router       /auth/oauth/native/{provider}/start [get]
func (h *AuthHandler) OAuthNativeStart(c *gin.Context) {
	provider, err := domain.ParseOAuthProviderName(c.Param("provider"))
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	redirectURI := c.Query("redirect_uri")
	result, err := h.oauthStart.Execute(c.Request.Context(), domain.OAuthStartRequest{
		Provider:   provider,
		RedirectTo: redirectURI,
		Mode:       domain.OAuthFlowModeNative,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, OAuthStartResponse{
		AuthURL: result.AuthURL,
		State:   result.State,
	})
}

// OAuthCallback godoc
// @Summary      Finish OAuth flow
// @Description  Exchanges OAuth code and returns auth tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        provider path string true "OAuth provider"
// @Param        body body OAuthCallbackRequest true "OAuth callback payload"
// @Success      200 {object} httputil.Response{data=AuthResponse}
// @Failure      400 {object} httputil.Response
// @Failure      401 {object} httputil.Response
// @Failure      409 {object} httputil.Response
// @Failure      500 {object} httputil.Response
// @Router       /auth/oauth/{provider}/callback [post]
func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	provider, err := domain.ParseOAuthProviderName(c.Param("provider"))
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	var req OAuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	result, err := h.oauthCallback.Execute(c.Request.Context(), domain.OAuthCallbackRequest{
		Provider:    provider,
		State:       req.State,
		Code:        req.Code,
		RedirectURI: req.RedirectURI,
		Mode:        domain.OAuthFlowModeLegacy,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toAuthResponse(result.User, result.TokenPair))
}

// OAuthNativeCallback godoc
// @Summary      Finish native OAuth flow
// @Description  Exchanges OAuth code from native client and returns auth tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        provider path string true "OAuth provider"
// @Param        body body OAuthCallbackRequest true "OAuth callback payload"
// @Success      200 {object} httputil.Response{data=AuthResponse}
// @Failure      400 {object} httputil.Response
// @Failure      401 {object} httputil.Response
// @Failure      409 {object} httputil.Response
// @Failure      500 {object} httputil.Response
// @Router       /auth/oauth/native/{provider}/callback [post]
func (h *AuthHandler) OAuthNativeCallback(c *gin.Context) {
	provider, err := domain.ParseOAuthProviderName(c.Param("provider"))
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	var req OAuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	result, err := h.oauthCallback.Execute(c.Request.Context(), domain.OAuthCallbackRequest{
		Provider:    provider,
		State:       req.State,
		Code:        req.Code,
		RedirectURI: req.RedirectURI,
		Mode:        domain.OAuthFlowModeNative,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toAuthResponse(result.User, result.TokenPair))
}

// OAuthNativeSDKCallback godoc
// @Summary      Finish native OAuth SDK flow
// @Description  Accepts provider access token from mobile SDK and returns FoodSea auth tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        provider path string true "OAuth provider"
// @Param        body body OAuthNativeSDKCallbackRequest true "OAuth SDK callback payload"
// @Success      200 {object} httputil.Response{data=AuthResponse}
// @Failure      400 {object} httputil.Response
// @Failure      401 {object} httputil.Response
// @Failure      409 {object} httputil.Response
// @Failure      500 {object} httputil.Response
// @Router       /auth/oauth/native/{provider}/sdk/callback [post]
func (h *AuthHandler) OAuthNativeSDKCallback(c *gin.Context) {
	provider, err := domain.ParseOAuthProviderName(c.Param("provider"))
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	var req OAuthNativeSDKCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	result, err := h.oauthCallback.ExecuteToken(c.Request.Context(), domain.OAuthTokenCallbackRequest{
		Provider:    provider,
		AccessToken: req.AccessToken,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toAuthResponse(result.User, result.TokenPair))
}
