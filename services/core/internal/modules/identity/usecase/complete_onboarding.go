package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/identity/domain"
)

type CompleteOnboarding struct {
	users domain.UserRepository
}

func NewCompleteOnboarding(users domain.UserRepository) *CompleteOnboarding {
	return &CompleteOnboarding{users: users}
}

func (c *CompleteOnboarding) Execute(ctx context.Context, userID uuid.UUID) error {
	return c.users.SetOnboardingDone(ctx, userID)
}
