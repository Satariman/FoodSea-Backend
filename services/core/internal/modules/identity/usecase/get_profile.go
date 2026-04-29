package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/identity/domain"
)

type GetProfile struct {
	users domain.UserRepository
}

func NewGetProfile(users domain.UserRepository) *GetProfile {
	return &GetProfile{users: users}
}

func (g *GetProfile) Execute(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return g.users.GetByID(ctx, userID)
}
