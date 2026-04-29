package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/cart/domain"
)

type RemoveItem struct {
	repo      domain.CartRepository
	publisher domain.CartEventPublisher
	log       *slog.Logger
}

func NewRemoveItem(repo domain.CartRepository, publisher domain.CartEventPublisher, log *slog.Logger) *RemoveItem {
	return &RemoveItem{repo: repo, publisher: publisher, log: log}
}

func (uc *RemoveItem) Execute(ctx context.Context, userID, productID uuid.UUID) error {
	if err := uc.repo.RemoveItem(ctx, userID, productID); err != nil {
		return err
	}

	if err := uc.publisher.ItemRemoved(ctx, userID, productID); err != nil {
		uc.log.WarnContext(ctx, "failed to publish ItemRemoved event", "error", err)
	}
	return nil
}
