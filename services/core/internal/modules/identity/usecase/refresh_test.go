package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestRefresh_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("valid refresh token → new pair", func(t *testing.T) {
		tokens := &MockTokenService{}
		tokens.On("RotateRefresh", ctx, "valid-refresh").Return(fakePair(), nil)

		uc := usecase.NewRefresh(tokens)
		pair, err := uc.Execute(ctx, "valid-refresh")

		require.NoError(t, err)
		assert.Equal(t, "access-token", pair.Access)
	})

	t.Run("empty token → invalid input", func(t *testing.T) {
		uc := usecase.NewRefresh(&MockTokenService{})
		_, err := uc.Execute(ctx, "")

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrInvalidInput))
	})

	t.Run("expired or unknown token → unauthorized", func(t *testing.T) {
		tokens := &MockTokenService{}
		tokens.On("RotateRefresh", ctx, "bad-refresh").Return(domain.TokenPair{}, sherrors.ErrUnauthorized)

		uc := usecase.NewRefresh(tokens)
		_, err := uc.Execute(ctx, "bad-refresh")

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrUnauthorized))
	})
}
