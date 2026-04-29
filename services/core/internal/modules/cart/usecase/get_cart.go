package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/cart/domain"
)

type GetCart struct {
	repo domain.CartRepository
}

func NewGetCart(repo domain.CartRepository) *GetCart {
	return &GetCart{repo: repo}
}

func (uc *GetCart) Execute(ctx context.Context, userID uuid.UUID) (*domain.Cart, error) {
	return uc.repo.GetByUser(ctx, userID)
}
