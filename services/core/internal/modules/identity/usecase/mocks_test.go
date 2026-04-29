package usecase_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/identity/domain"
)

// MockUserRepository mocks domain.UserRepository.
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, u *domain.User, passwordHash string) error {
	args := m.Called(ctx, u, passwordHash)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	args := m.Called(ctx, phone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetPasswordHash(ctx context.Context, id uuid.UUID) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

func (m *MockUserRepository) SetOnboardingDone(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockPasswordHasher mocks domain.PasswordHasher.
type MockPasswordHasher struct {
	mock.Mock
}

func (m *MockPasswordHasher) Hash(plain string) (string, error) {
	args := m.Called(plain)
	return args.String(0), args.Error(1)
}

func (m *MockPasswordHasher) Verify(hash, plain string) error {
	args := m.Called(hash, plain)
	return args.Error(0)
}

// MockTokenService mocks domain.TokenService.
type MockTokenService struct {
	mock.Mock
}

func (m *MockTokenService) IssuePair(ctx context.Context, userID uuid.UUID) (domain.TokenPair, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(domain.TokenPair), args.Error(1)
}

func (m *MockTokenService) ValidateAccess(token string) (domain.Claims, error) {
	args := m.Called(token)
	return args.Get(0).(domain.Claims), args.Error(1)
}

func (m *MockTokenService) RotateRefresh(ctx context.Context, refresh string) (domain.TokenPair, error) {
	args := m.Called(ctx, refresh)
	return args.Get(0).(domain.TokenPair), args.Error(1)
}

func (m *MockTokenService) Revoke(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// helpers

func ptr(s string) *string { return &s }

func fakePair() domain.TokenPair {
	return domain.TokenPair{
		Access:           "access-token",
		Refresh:          "refresh-token",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
}

func fakeUser() *domain.User {
	email := "test@example.com"
	return &domain.User{
		ID:    uuid.New(),
		Email: &email,
	}
}
