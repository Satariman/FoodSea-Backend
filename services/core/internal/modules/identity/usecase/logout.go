package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/identity/domain"
)

type Logout struct {
	tokens domain.TokenService
}

func NewLogout(tokens domain.TokenService) *Logout {
	return &Logout{tokens: tokens}
}

func (l *Logout) Execute(ctx context.Context, userID uuid.UUID) error {
	return l.tokens.Revoke(ctx, userID)
}
