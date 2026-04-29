package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	shared "github.com/foodsea/ordering/internal/shared/domain"
	sherrors "github.com/foodsea/ordering/internal/shared/errors"
)

// CancelOrder transitions an order to cancelled status. Called by Saga during compensation.
type CancelOrder struct {
	repo      domain.OrderRepository
	publisher domain.OrderEventPublisher
	log       *slog.Logger
}

func NewCancelOrder(repo domain.OrderRepository, publisher domain.OrderEventPublisher, log *slog.Logger) *CancelOrder {
	return &CancelOrder{repo: repo, publisher: publisher, log: log}
}

func (uc *CancelOrder) Execute(ctx context.Context, orderID uuid.UUID, reason string) error {
	if reason == "" {
		return sherrors.ErrInvalidInput
	}

	prevOrder, err := uc.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	prevStatus := prevOrder.Status

	if err = uc.repo.TransitionStatus(ctx, orderID, shared.StatusCancelled, &reason); err != nil {
		return err
	}

	if err = uc.publisher.OrderCancelled(ctx, orderID, reason); err != nil {
		uc.log.WarnContext(ctx, "order.cancelled event publish failed", "order_id", orderID, "error", err)
	}
	if err = uc.publisher.OrderStatusChanged(ctx, orderID, prevStatus, shared.StatusCancelled); err != nil {
		uc.log.WarnContext(ctx, "order.status_changed event publish failed", "order_id", orderID, "error", err)
	}
	return nil
}
