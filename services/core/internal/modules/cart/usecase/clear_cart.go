package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/cart/domain"
)

type ClearCart struct {
	repo      domain.CartRepository
	publisher domain.CartEventPublisher
	log       *slog.Logger
}

func NewClearCart(repo domain.CartRepository, publisher domain.CartEventPublisher, log *slog.Logger) *ClearCart {
	return &ClearCart{repo: repo, publisher: publisher, log: log}
}

func (uc *ClearCart) Execute(ctx context.Context, userID uuid.UUID) error {
	if err := uc.repo.Clear(ctx, userID); err != nil {
		return err
	}

	if err := uc.publisher.Cleared(ctx, userID); err != nil {
		uc.log.WarnContext(ctx, "failed to publish Cleared event", "error", err)
	}
	return nil
}
