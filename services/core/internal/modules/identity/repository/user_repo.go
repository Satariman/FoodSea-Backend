package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/ent/user"
	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type UserRepo struct {
	client *ent.Client
}

func NewUserRepo(client *ent.Client) *UserRepo {
	return &UserRepo{client: client}
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User, passwordHash string) error {
	created, err := r.client.User.Create().
		SetID(u.ID).
		SetPasswordHash(passwordHash).
		SetOnboardingDone(u.OnboardingDone).
		SetNillableEmail(u.Email).
		SetNillablePhone(u.Phone).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return sherrors.ErrAlreadyExists
		}
		return fmt.Errorf("creating user: %w", err)
	}

	u.CreatedAt = created.CreatedAt
	u.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, err := r.client.User.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return toDomainUser(u), nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, err := r.client.User.Query().Where(user.Email(email)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return toDomainUser(u), nil
}

func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	u, err := r.client.User.Query().Where(user.Phone(phone)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting user by phone: %w", err)
	}
	return toDomainUser(u), nil
}

func (r *UserRepo) GetPasswordHash(ctx context.Context, id uuid.UUID) (string, error) {
	u, err := r.client.User.Query().
		Where(user.ID(id)).
		Select(user.FieldPasswordHash).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", sherrors.ErrNotFound
		}
		return "", fmt.Errorf("getting password hash: %w", err)
	}
	return u.PasswordHash, nil
}

func (r *UserRepo) SetOnboardingDone(ctx context.Context, id uuid.UUID) error {
	err := r.client.User.UpdateOneID(id).SetOnboardingDone(true).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return sherrors.ErrNotFound
		}
		return fmt.Errorf("setting onboarding done: %w", err)
	}
	return nil
}

func toDomainUser(u *ent.User) *domain.User {
	return &domain.User{
		ID:             u.ID,
		Phone:          u.Phone,
		Email:          u.Email,
		OnboardingDone: u.OnboardingDone,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
	}
}
