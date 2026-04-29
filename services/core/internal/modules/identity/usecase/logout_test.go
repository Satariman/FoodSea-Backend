package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/usecase"
)

func TestLogout_Execute(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("revokes all sessions", func(t *testing.T) {
		tokens := &MockTokenService{}
		tokens.On("Revoke", ctx, userID).Return(nil)

		uc := usecase.NewLogout(tokens)
		err := uc.Execute(ctx, userID)

		require.NoError(t, err)
		tokens.AssertExpectations(t)
	})

	t.Run("revoke error propagated", func(t *testing.T) {
		tokens := &MockTokenService{}
		tokens.On("Revoke", ctx, userID).Return(errors.New("redis error"))

		uc := usecase.NewLogout(tokens)
		err := uc.Execute(ctx, userID)

		assert.Error(t, err)
	})
}
