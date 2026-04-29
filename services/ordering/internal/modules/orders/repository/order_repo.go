package repository

import (
	"context"
	"log/slog"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/foodsea/ordering/ent"
	entorder "github.com/foodsea/ordering/ent/order"
	entosh "github.com/foodsea/ordering/ent/orderstatushistory"
	"github.com/foodsea/ordering/internal/modules/orders/domain"
	shared "github.com/foodsea/ordering/internal/shared/domain"
	sherrors "github.com/foodsea/ordering/internal/shared/errors"
)

// OrderRepository is the Ent-backed implementation of domain.OrderRepository.
type OrderRepository struct {
	client *ent.Client
	log    *slog.Logger
}

// NewOrderRepository creates an OrderRepository.
func NewOrderRepository(client *ent.Client, log *slog.Logger) *OrderRepository {
	return &OrderRepository{client: client, log: log}
}

// CreatePending transactionally inserts order + items + first history entry (status=created).
func (r *OrderRepository) CreatePending(ctx context.Context, o *domain.Order) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	o.ID = uuid.New()
	o.Status = shared.StatusCreated

	created, err := tx.Order.Create().
		SetID(o.ID).
		SetUserID(o.UserID).
		SetNillableOptimizationResultID(o.OptimizationResultID).
		SetTotalKopecks(o.TotalKopecks).
		SetDeliveryKopecks(o.DeliveryKopecks).
		SetStatus(shared.StatusCreated.String()).
		Save(ctx)
	if err != nil {
		return err
	}
	o.CreatedAt = created.CreatedAt
	o.UpdatedAt = created.UpdatedAt

	if len(o.Items) > 0 {
		bulk := make([]*ent.OrderItemCreate, len(o.Items))
		for i, it := range o.Items {
			it.ID = uuid.New()
			bulk[i] = tx.OrderItem.Create().
				SetID(it.ID).
				SetOrderID(o.ID).
				SetProductID(it.ProductID).
				SetProductName(it.ProductName).
				SetStoreID(it.StoreID).
				SetStoreName(it.StoreName).
				SetQuantity(it.Quantity).
				SetPriceKopecks(it.PriceKopecks)
			o.Items[i] = it
		}
		if _, err = tx.OrderItem.CreateBulk(bulk...).Save(ctx); err != nil {
			return err
		}
	}

	now := time.Now()
	_, err = tx.OrderStatusHistory.Create().
		SetOrderID(o.ID).
		SetStatus(shared.StatusCreated.String()).
		SetChangedAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	o.History = []domain.StatusChange{
		{Status: shared.StatusCreated, ChangedAt: now},
	}

	return tx.Commit()
}

// GetByID returns an order with eagerly loaded items and history.
func (r *OrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	o, err := r.client.Order.Query().
		Where(entorder.IDEQ(id)).
		WithItems().
		WithHistory(func(q *ent.OrderStatusHistoryQuery) {
			q.Order(ent.Desc(entosh.FieldChangedAt))
		}).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, err
	}
	return entOrderToDomain(o), nil
}

// GetByIDForUser returns ErrNotFound if the order belongs to a different user.
func (r *OrderRepository) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*domain.Order, error) {
	o, err := r.client.Order.Query().
		Where(entorder.IDEQ(id), entorder.UserIDEQ(userID)).
		WithItems().
		WithHistory(func(q *ent.OrderStatusHistoryQuery) {
			q.Order(ent.Desc(entosh.FieldChangedAt))
		}).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, err
	}
	return entOrderToDomain(o), nil
}

// ListByUser returns paginated orders for a user, sorted by created_at DESC.
func (r *OrderRepository) ListByUser(ctx context.Context, userID uuid.UUID, p shared.Pagination) ([]domain.Order, int, error) {
	query := r.client.Order.Query().Where(entorder.UserIDEQ(userID))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := query.
		Order(entorder.ByCreatedAt(sql.OrderDesc())).
		Offset(p.Offset()).
		Limit(p.Limit()).
		WithItems().
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	orders := make([]domain.Order, len(rows))
	for i, o := range rows {
		orders[i] = *entOrderToDomain(o)
	}
	return orders, total, nil
}

// TransitionStatus atomically transitions order status with optimistic locking.
// Returns ErrConflict if the current status does not allow the transition.
func (r *OrderRepository) TransitionStatus(ctx context.Context, id uuid.UUID, to shared.OrderStatus, comment *string) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	o, err := tx.Order.Query().Where(entorder.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return sherrors.ErrNotFound
		}
		return err
	}

	current := shared.OrderStatus(o.Status)
	if !current.CanTransitionTo(to) {
		return sherrors.ErrConflict
	}

	// Optimistic update: WHERE status = current prevents lost-update races.
	n, err := tx.Order.Update().
		Where(entorder.IDEQ(id), entorder.StatusEQ(o.Status)).
		SetStatus(to.String()).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		// Concurrent update changed the status first.
		return sherrors.ErrConflict
	}

	_, err = tx.OrderStatusHistory.Create().
		SetOrderID(id).
		SetStatus(to.String()).
		SetNillableComment(comment).
		SetChangedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// entOrderToDomain maps an Ent Order (with loaded edges) to the domain Order.
func entOrderToDomain(o *ent.Order) *domain.Order {
	items := make([]domain.OrderItem, len(o.Edges.Items))
	for i, it := range o.Edges.Items {
		items[i] = domain.OrderItem{
			ID:           it.ID,
			ProductID:    it.ProductID,
			ProductName:  it.ProductName,
			StoreID:      it.StoreID,
			StoreName:    it.StoreName,
			Quantity:     it.Quantity,
			PriceKopecks: it.PriceKopecks,
		}
	}

	history := make([]domain.StatusChange, len(o.Edges.History))
	for i, h := range o.Edges.History {
		history[i] = domain.StatusChange{
			Status:    shared.OrderStatus(h.Status),
			Comment:   h.Comment,
			ChangedAt: h.ChangedAt,
		}
	}

	return &domain.Order{
		ID:                   o.ID,
		UserID:               o.UserID,
		OptimizationResultID: o.OptimizationResultID,
		TotalKopecks:         o.TotalKopecks,
		DeliveryKopecks:      o.DeliveryKopecks,
		Status:               shared.OrderStatus(o.Status),
		Items:                items,
		History:              history,
		CreatedAt:            o.CreatedAt,
		UpdatedAt:            o.UpdatedAt,
	}
}
