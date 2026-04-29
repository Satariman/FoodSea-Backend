package usecase

import (
	"context"
	"errors"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type LoginResult struct {
	User      *domain.User
	TokenPair domain.TokenPair
}

type Login struct {
	users  domain.UserRepository
	hasher domain.PasswordHasher
	tokens domain.TokenService
}

func NewLogin(users domain.UserRepository, hasher domain.PasswordHasher, tokens domain.TokenService) *Login {
	return &Login{users: users, hasher: hasher, tokens: tokens}
}

func (l *Login) Execute(ctx context.Context, creds domain.Credentials) (LoginResult, error) {
	var u *domain.User
	var err error

	switch {
	case creds.Email != nil && *creds.Email != "":
		u, err = l.users.GetByEmail(ctx, *creds.Email)
	case creds.Phone != nil && *creds.Phone != "":
		u, err = l.users.GetByPhone(ctx, *creds.Phone)
	default:
		return LoginResult{}, sherrors.ErrUnauthorized
	}

	if err != nil {
		if errors.Is(err, sherrors.ErrNotFound) {
			return LoginResult{}, sherrors.ErrUnauthorized
		}
		return LoginResult{}, err
	}

	hash, err := l.users.GetPasswordHash(ctx, u.ID)
	if err != nil {
		return LoginResult{}, err
	}

	if err := l.hasher.Verify(hash, creds.Password); err != nil {
		return LoginResult{}, sherrors.ErrUnauthorized
	}

	pair, err := l.tokens.IssuePair(ctx, u.ID)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{User: u, TokenPair: pair}, nil
}
