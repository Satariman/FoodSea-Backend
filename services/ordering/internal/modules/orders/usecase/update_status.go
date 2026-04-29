package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	shared "github.com/foodsea/ordering/internal/shared/domain"
	sherrors "github.com/foodsea/ordering/internal/shared/errors"
)

// UpdateStatus is the admin-facing use case for manual status transitions (demo/internal).
type UpdateStatus struct {
	repo      domain.OrderRepository
	publisher domain.OrderEventPublisher
	log       *slog.Logger
}

func NewUpdateStatus(repo domain.OrderRepository, publisher domain.OrderEventPublisher, log *slog.Logger) *UpdateStatus {
	return &UpdateStatus{repo: repo, publisher: publisher, log: log}
}

func (uc *UpdateStatus) Execute(ctx context.Context, orderID uuid.UUID, to shared.OrderStatus, comment *string) error {
	order, err := uc.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}

	if !order.Status.CanTransitionTo(to) {
		return sherrors.ErrConflict
	}

	prevStatus := order.Status
	if err = uc.repo.TransitionStatus(ctx, orderID, to, comment); err != nil {
		return err
	}

	if err = uc.publisher.OrderStatusChanged(ctx, orderID, prevStatus, to); err != nil {
		uc.log.WarnContext(ctx, "order.status_changed event publish failed", "order_id", orderID, "error", err)
	}
	return nil
}
