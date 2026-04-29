package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/platform/httputil"
	"github.com/foodsea/core/internal/platform/middleware"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type getProfileUseCase interface {
	Execute(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

type completeOnboardingUseCase interface {
	Execute(ctx context.Context, userID uuid.UUID) error
}

type UserHandler struct {
	getProfile         getProfileUseCase
	completeOnboarding completeOnboardingUseCase
}

func NewUserHandler(getProfile getProfileUseCase, completeOnboarding completeOnboardingUseCase) *UserHandler {
	return &UserHandler{
		getProfile:         getProfile,
		completeOnboarding: completeOnboarding,
	}
}

// Me godoc
// @Summary      Get current user profile
// @Description  Returns the authenticated user's profile
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object} httputil.Response{data=UserResponse}
// @Failure      401  {object} httputil.Response
// @Failure      404  {object} httputil.Response
// @Failure      500  {object} httputil.Response
// @Router       /users/me [get]
func (h *UserHandler) Me(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, sherrors.ErrUnauthorized)
		return
	}

	u, err := h.getProfile.Execute(c.Request.Context(), userID)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toUserResponse(u))
}

// CompleteOnboarding godoc
// @Summary      Complete onboarding
// @Description  Marks the onboarding as done for the current user (idempotent)
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      204
// @Failure      401  {object} httputil.Response
// @Failure      500  {object} httputil.Response
// @Router       /users/me/onboarding [post]
func (h *UserHandler) CompleteOnboarding(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, sherrors.ErrUnauthorized)
		return
	}

	if err := h.completeOnboarding.Execute(c.Request.Context(), userID); err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.NoContent(c)
}
