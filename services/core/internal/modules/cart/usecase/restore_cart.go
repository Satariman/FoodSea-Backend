package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/cart/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type RestoreCart struct {
	repo domain.CartRepository
	log  *slog.Logger
}

func NewRestoreCart(repo domain.CartRepository, log *slog.Logger) *RestoreCart {
	return &RestoreCart{repo: repo, log: log}
}

func (uc *RestoreCart) Execute(ctx context.Context, userID uuid.UUID, items []domain.CartItem) error {
	for _, item := range items {
		if item.Quantity < 1 || item.Quantity > 99 {
			return fmt.Errorf("quantity must be between 1 and 99: %w", sherrors.ErrInvalidInput)
		}
	}

	if err := uc.repo.Restore(ctx, userID, items); err != nil {
		return err
	}

	uc.log.InfoContext(ctx, "cart restored via saga", "user_id", userID, "items_count", len(items))
	return nil
}
