package domain

import (
	"context"

	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, u *User, passwordHash string) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByPhone(ctx context.Context, phone string) (*User, error)
	GetPasswordHash(ctx context.Context, id uuid.UUID) (string, error)
	SetOnboardingDone(ctx context.Context, id uuid.UUID) error
}
