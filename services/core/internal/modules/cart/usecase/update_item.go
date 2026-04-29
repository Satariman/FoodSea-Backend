package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/cart/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type UpdateItem struct {
	repo      domain.CartRepository
	publisher domain.CartEventPublisher
	log       *slog.Logger
}

func NewUpdateItem(repo domain.CartRepository, publisher domain.CartEventPublisher, log *slog.Logger) *UpdateItem {
	return &UpdateItem{repo: repo, publisher: publisher, log: log}
}

func (uc *UpdateItem) Execute(ctx context.Context, userID, productID uuid.UUID, qty int16) error {
	if qty < 1 || qty > 99 {
		return fmt.Errorf("quantity must be between 1 and 99: %w", sherrors.ErrInvalidInput)
	}

	if err := uc.repo.UpdateItemQuantity(ctx, userID, productID, qty); err != nil {
		return err
	}

	if err := uc.publisher.ItemUpdated(ctx, userID, productID, qty); err != nil {
		uc.log.WarnContext(ctx, "failed to publish ItemUpdated event", "error", err)
	}
	return nil
}
