package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type RegisterResult struct {
	User      *domain.User
	TokenPair domain.TokenPair
}

type Register struct {
	users  domain.UserRepository
	hasher domain.PasswordHasher
	tokens domain.TokenService
}

func NewRegister(users domain.UserRepository, hasher domain.PasswordHasher, tokens domain.TokenService) *Register {
	return &Register{users: users, hasher: hasher, tokens: tokens}
}

func (r *Register) Execute(ctx context.Context, creds domain.Credentials) (RegisterResult, error) {
	if err := creds.Validate(); err != nil {
		return RegisterResult{}, err
	}

	if creds.Email != nil {
		if _, err := r.users.GetByEmail(ctx, *creds.Email); err == nil {
			return RegisterResult{}, fmt.Errorf("%w: email already registered", sherrors.ErrAlreadyExists)
		} else if !errors.Is(err, sherrors.ErrNotFound) {
			return RegisterResult{}, err
		}
	}
	if creds.Phone != nil {
		if _, err := r.users.GetByPhone(ctx, *creds.Phone); err == nil {
			return RegisterResult{}, fmt.Errorf("%w: phone already registered", sherrors.ErrAlreadyExists)
		} else if !errors.Is(err, sherrors.ErrNotFound) {
			return RegisterResult{}, err
		}
	}

	hash, err := r.hasher.Hash(creds.Password)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("hashing password: %w", err)
	}

	u := &domain.User{
		ID:    uuid.New(),
		Phone: creds.Phone,
		Email: creds.Email,
	}

	if err := r.users.Create(ctx, u, hash); err != nil {
		return RegisterResult{}, err
	}

	pair, err := r.tokens.IssuePair(ctx, u.ID)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("issuing tokens: %w", err)
	}

	return RegisterResult{User: u, TokenPair: pair}, nil
}
