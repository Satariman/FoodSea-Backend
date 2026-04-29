package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	shared "github.com/foodsea/ordering/internal/shared/domain"
)

// ConfirmOrder transitions an order to confirmed status. Called by the Saga orchestrator.
type ConfirmOrder struct {
	repo      domain.OrderRepository
	publisher domain.OrderEventPublisher
	log       *slog.Logger
}

func NewConfirmOrder(repo domain.OrderRepository, publisher domain.OrderEventPublisher, log *slog.Logger) *ConfirmOrder {
	return &ConfirmOrder{repo: repo, publisher: publisher, log: log}
}

func (uc *ConfirmOrder) Execute(ctx context.Context, orderID uuid.UUID) error {
	if err := uc.repo.TransitionStatus(ctx, orderID, shared.StatusConfirmed, nil); err != nil {
		return err
	}

	if err := uc.publisher.OrderConfirmed(ctx, orderID); err != nil {
		uc.log.WarnContext(ctx, "order.confirmed event publish failed", "order_id", orderID, "error", err)
	}
	if err := uc.publisher.OrderStatusChanged(ctx, orderID, shared.StatusCreated, shared.StatusConfirmed); err != nil {
		uc.log.WarnContext(ctx, "order.status_changed event publish failed", "order_id", orderID, "error", err)
	}
	return nil
}
