package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestLogin_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("success with email", func(t *testing.T) {
		repo := &MockUserRepository{}
		hasher := &MockPasswordHasher{}
		tokens := &MockTokenService{}

		u := fakeUser()
		email := *u.Email
		repo.On("GetByEmail", ctx, email).Return(u, nil)
		repo.On("GetPasswordHash", ctx, u.ID).Return("$hashed", nil)
		hasher.On("Verify", "$hashed", "password1").Return(nil)
		tokens.On("IssuePair", ctx, u.ID).Return(fakePair(), nil)

		uc := usecase.NewLogin(repo, hasher, tokens)
		result, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "password1"})

		require.NoError(t, err)
		assert.Equal(t, u.ID, result.User.ID)
		assert.Equal(t, "access-token", result.TokenPair.Access)
	})

	t.Run("success with phone", func(t *testing.T) {
		repo := &MockUserRepository{}
		hasher := &MockPasswordHasher{}
		tokens := &MockTokenService{}

		phone := "+79001234567"
		u := &domain.User{ID: fakeUser().ID, Phone: &phone}
		repo.On("GetByPhone", ctx, phone).Return(u, nil)
		repo.On("GetPasswordHash", ctx, u.ID).Return("$hashed", nil)
		hasher.On("Verify", "$hashed", "pass1234").Return(nil)
		tokens.On("IssuePair", ctx, u.ID).Return(fakePair(), nil)

		uc := usecase.NewLogin(repo, hasher, tokens)
		result, err := uc.Execute(ctx, domain.Credentials{Phone: &phone, Password: "pass1234"})

		require.NoError(t, err)
		assert.NotNil(t, result.User)
	})

	t.Run("email not found → 401 not 404", func(t *testing.T) {
		repo := &MockUserRepository{}
		email := "nobody@example.com"
		repo.On("GetByEmail", ctx, email).Return(nil, sherrors.ErrNotFound)

		uc := usecase.NewLogin(repo, &MockPasswordHasher{}, &MockTokenService{})
		_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "password1"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrUnauthorized))
		assert.False(t, errors.Is(err, sherrors.ErrNotFound))
	})

	t.Run("wrong password → 401", func(t *testing.T) {
		repo := &MockUserRepository{}
		hasher := &MockPasswordHasher{}

		u := fakeUser()
		email := *u.Email
		repo.On("GetByEmail", ctx, email).Return(u, nil)
		repo.On("GetPasswordHash", ctx, u.ID).Return("$hashed", nil)
		hasher.On("Verify", "$hashed", "wrongpass").Return(errors.New("mismatch"))

		uc := usecase.NewLogin(repo, hasher, &MockTokenService{})
		_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "wrongpass"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrUnauthorized))
	})

	t.Run("no credentials → 401", func(t *testing.T) {
		uc := usecase.NewLogin(&MockUserRepository{}, &MockPasswordHasher{}, &MockTokenService{})
		_, err := uc.Execute(ctx, domain.Credentials{Password: "password1"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrUnauthorized))
	})

	t.Run("token issue error propagated", func(t *testing.T) {
		repo := &MockUserRepository{}
		hasher := &MockPasswordHasher{}
		tokens := &MockTokenService{}

		u := fakeUser()
		email := *u.Email
		repo.On("GetByEmail", ctx, email).Return(u, nil)
		repo.On("GetPasswordHash", ctx, u.ID).Return("$hashed", nil)
		hasher.On("Verify", "$hashed", "password1").Return(nil)
		tokens.On("IssuePair", ctx, u.ID).Return(domain.TokenPair{}, errors.New("redis down"))

		uc := usecase.NewLogin(repo, hasher, tokens)
		_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "password1"})

		require.Error(t, err)
		assert.False(t, errors.Is(err, sherrors.ErrUnauthorized))
	})

	t.Run("IssuePair uses mock.Anything for uuid matching", func(t *testing.T) {
		repo := &MockUserRepository{}
		hasher := &MockPasswordHasher{}
		tokens := &MockTokenService{}

		u := fakeUser()
		email := *u.Email
		repo.On("GetByEmail", ctx, email).Return(u, nil)
		repo.On("GetPasswordHash", ctx, mock.Anything).Return("$hashed", nil)
		hasher.On("Verify", "$hashed", "password1").Return(nil)
		tokens.On("IssuePair", ctx, mock.Anything).Return(fakePair(), nil)

		uc := usecase.NewLogin(repo, hasher, tokens)
		_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "password1"})
		require.NoError(t, err)
	})
}
