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

func TestRegister_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("success with email", func(t *testing.T) {
		repo := &MockUserRepository{}
		hasher := &MockPasswordHasher{}
		tokens := &MockTokenService{}

		email := "user@example.com"
		repo.On("GetByEmail", ctx, email).Return(nil, sherrors.ErrNotFound)
		repo.On("Create", ctx, mock.Anything, "hashed").Return(nil)
		hasher.On("Hash", "password1").Return("hashed", nil)
		tokens.On("IssuePair", ctx, mock.Anything).Return(fakePair(), nil)

		uc := usecase.NewRegister(repo, hasher, tokens)
		result, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "password1"})

		require.NoError(t, err)
		assert.NotNil(t, result.User)
		assert.Equal(t, "access-token", result.TokenPair.Access)
		repo.AssertExpectations(t)
		hasher.AssertExpectations(t)
		tokens.AssertExpectations(t)
	})

	t.Run("success with phone", func(t *testing.T) {
		repo := &MockUserRepository{}
		hasher := &MockPasswordHasher{}
		tokens := &MockTokenService{}

		phone := "+79001234567"
		repo.On("GetByPhone", ctx, phone).Return(nil, sherrors.ErrNotFound)
		repo.On("Create", ctx, mock.Anything, "hashed").Return(nil)
		hasher.On("Hash", "password1").Return("hashed", nil)
		tokens.On("IssuePair", ctx, mock.Anything).Return(fakePair(), nil)

		uc := usecase.NewRegister(repo, hasher, tokens)
		result, err := uc.Execute(ctx, domain.Credentials{Phone: &phone, Password: "password1"})

		require.NoError(t, err)
		assert.NotNil(t, result.User)
	})

	t.Run("both email and phone → invalid input", func(t *testing.T) {
		email, phone := "user@example.com", "+79001234567"
		uc := usecase.NewRegister(&MockUserRepository{}, &MockPasswordHasher{}, &MockTokenService{})
		_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Phone: &phone, Password: "password1"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrInvalidInput))
	})

	t.Run("no email and no phone → invalid input", func(t *testing.T) {
		uc := usecase.NewRegister(&MockUserRepository{}, &MockPasswordHasher{}, &MockTokenService{})
		_, err := uc.Execute(ctx, domain.Credentials{Password: "password1"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrInvalidInput))
	})

	t.Run("password too short → invalid input", func(t *testing.T) {
		email := "user@example.com"
		uc := usecase.NewRegister(&MockUserRepository{}, &MockPasswordHasher{}, &MockTokenService{})
		_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "short"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrInvalidInput))
	})

	t.Run("duplicate email → already exists", func(t *testing.T) {
		repo := &MockUserRepository{}
		email := "user@example.com"
		repo.On("GetByEmail", ctx, email).Return(fakeUser(), nil)

		uc := usecase.NewRegister(repo, &MockPasswordHasher{}, &MockTokenService{})
		_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "password1"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrAlreadyExists))
	})

	t.Run("duplicate phone → already exists", func(t *testing.T) {
		repo := &MockUserRepository{}
		phone := "+79001234567"
		repo.On("GetByPhone", ctx, phone).Return(fakeUser(), nil)

		uc := usecase.NewRegister(repo, &MockPasswordHasher{}, &MockTokenService{})
		_, err := uc.Execute(ctx, domain.Credentials{Phone: &phone, Password: "password1"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, sherrors.ErrAlreadyExists))
	})

	t.Run("hasher error → internal error", func(t *testing.T) {
		repo := &MockUserRepository{}
		hasher := &MockPasswordHasher{}
		email := "user@example.com"
		repo.On("GetByEmail", ctx, email).Return(nil, sherrors.ErrNotFound)
		hasher.On("Hash", "password1").Return("", errors.New("bcrypt failure"))

		uc := usecase.NewRegister(repo, hasher, &MockTokenService{})
		_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "password1"})

		require.Error(t, err)
		assert.False(t, errors.Is(err, sherrors.ErrInvalidInput))
		assert.False(t, errors.Is(err, sherrors.ErrAlreadyExists))
	})

	t.Run("token service error → internal error", func(t *testing.T) {
		repo := &MockUserRepository{}
		hasher := &MockPasswordHasher{}
		tokens := &MockTokenService{}
		email := "user@example.com"
		repo.On("GetByEmail", ctx, email).Return(nil, sherrors.ErrNotFound)
		repo.On("Create", ctx, mock.Anything, "hashed").Return(nil)
		hasher.On("Hash", "password1").Return("hashed", nil)
		tokens.On("IssuePair", ctx, mock.Anything).Return(domain.TokenPair{}, errors.New("redis error"))

		uc := usecase.NewRegister(repo, hasher, tokens)
		_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "password1"})

		require.Error(t, err)
	})
}
