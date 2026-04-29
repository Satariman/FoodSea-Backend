package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/modules/orders/domain"
	shared "github.com/foodsea/ordering/internal/shared/domain"
)

// CreatePendingInput carries snapshot data from the optimization result.
type CreatePendingInput struct {
	UserID               uuid.UUID
	OptimizationResultID *uuid.UUID
	Items                []OrderItemSnapshot
	TotalKopecks         int64
	DeliveryKopecks      int64
}

// OrderItemSnapshot is the snapshot data for a single item at order creation time.
type OrderItemSnapshot struct {
	ProductID    uuid.UUID
	ProductName  string
	StoreID      uuid.UUID
	StoreName    string
	Quantity     int16
	PriceKopecks int64
}

// CreateOrderPending is an internal use case invoked only by the Saga orchestrator.
type CreateOrderPending struct {
	repo      domain.OrderRepository
	publisher domain.OrderEventPublisher
	log       *slog.Logger
}

// NewCreateOrderPending constructs the use case.
func NewCreateOrderPending(repo domain.OrderRepository, publisher domain.OrderEventPublisher, log *slog.Logger) *CreateOrderPending {
	return &CreateOrderPending{repo: repo, publisher: publisher, log: log}
}

// Execute creates the order and publishes OrderCreated (best-effort).
func (uc *CreateOrderPending) Execute(ctx context.Context, input CreatePendingInput) (*domain.Order, error) {
	items := make([]domain.OrderItem, len(input.Items))
	for i, it := range input.Items {
		items[i] = domain.OrderItem{
			ProductID:    it.ProductID,
			ProductName:  it.ProductName,
			StoreID:      it.StoreID,
			StoreName:    it.StoreName,
			Quantity:     it.Quantity,
			PriceKopecks: it.PriceKopecks,
		}
	}

	order := &domain.Order{
		UserID:               input.UserID,
		OptimizationResultID: input.OptimizationResultID,
		TotalKopecks:         input.TotalKopecks,
		DeliveryKopecks:      input.DeliveryKopecks,
		Status:               shared.StatusCreated,
		Items:                items,
	}

	if err := uc.repo.CreatePending(ctx, order); err != nil {
		return nil, err
	}

	// Best-effort: publish event but do not fail the order on Kafka error.
	if err := uc.publisher.OrderCreated(ctx, order); err != nil {
		uc.log.WarnContext(ctx, "order.created event publish failed",
			"order_id", order.ID,
			"error", err,
		)
	}

	return order, nil
}
