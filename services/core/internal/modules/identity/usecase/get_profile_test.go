package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestGetProfile_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("existing user returns profile", func(t *testing.T) {
		repo := &MockUserRepository{}
		u := fakeUser()
		repo.On("GetByID", ctx, u.ID).Return(u, nil)

		uc := usecase.NewGetProfile(repo)
		result, err := uc.Execute(ctx, u.ID)

		require.NoError(t, err)
		assert.Equal(t, u.ID, result.ID)
	})

	t.Run("non-existing user → not found", func(t *testing.T) {
		repo := &MockUserRepository{}
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, sherrors.ErrNotFound)

		uc := usecase.NewGetProfile(repo)
		_, err := uc.Execute(ctx, id)

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrNotFound))
	})
}
