package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/usecase"
)

func TestCompleteOnboarding_Execute(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("marks onboarding done", func(t *testing.T) {
		repo := &MockUserRepository{}
		repo.On("SetOnboardingDone", ctx, userID).Return(nil)

		uc := usecase.NewCompleteOnboarding(repo)
		err := uc.Execute(ctx, userID)

		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("idempotent — second call also succeeds", func(t *testing.T) {
		repo := &MockUserRepository{}
		repo.On("SetOnboardingDone", ctx, userID).Return(nil).Times(2)

		uc := usecase.NewCompleteOnboarding(repo)
		require.NoError(t, uc.Execute(ctx, userID))
		require.NoError(t, uc.Execute(ctx, userID))

		repo.AssertExpectations(t)
	})
}
