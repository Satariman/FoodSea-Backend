package orders

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/ordering/ent"
	"github.com/foodsea/ordering/internal/modules/orders/domain"
	"github.com/foodsea/ordering/internal/modules/orders/events"
	"github.com/foodsea/ordering/internal/modules/orders/handler"
	"github.com/foodsea/ordering/internal/modules/orders/repository"
	"github.com/foodsea/ordering/internal/modules/orders/usecase"
	"github.com/foodsea/ordering/internal/platform/kafka"
)

// Deps holds the dependencies needed to build the orders Module.
type Deps struct {
	Ent      *ent.Client
	Producer *kafka.Producer // order.events topic producer
	Log      *slog.Logger
}

// Module wires together all layers of the orders bounded context.
type Module struct {
	CreatePending *usecase.CreateOrderPending
	Confirm       *usecase.ConfirmOrder
	Cancel        *usecase.CancelOrder
	UpdateStatus  *usecase.UpdateStatus
	Get           *usecase.GetOrder
	List          *usecase.ListOrders

	handler *handler.OrderHandler
}

// NewModule constructs the Module, wiring repository, publisher, and use cases.
func NewModule(deps Deps) *Module {
	repo := repository.NewOrderRepository(deps.Ent, deps.Log)
	pub := events.NewKafkaPublisher(deps.Producer, deps.Log)

	m := &Module{
		CreatePending: usecase.NewCreateOrderPending(repo, pub, deps.Log),
		Confirm:       usecase.NewConfirmOrder(repo, pub, deps.Log),
		Cancel:        usecase.NewCancelOrder(repo, pub, deps.Log),
		UpdateStatus:  usecase.NewUpdateStatus(repo, pub, deps.Log),
		Get:           usecase.NewGetOrder(repo),
		List:          usecase.NewListOrders(repo),
	}
	m.handler = handler.NewOrderHandler(m.Get, m.List, m.UpdateStatus)
	return m
}

// RegisterRoutes mounts the order HTTP routes on the protected router group.
func (m *Module) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/orders", m.handler.ListOrders)
	protected.GET("/orders/:id", m.handler.GetOrder)
	protected.PATCH("/orders/:id/status", m.handler.UpdateStatus)
}

// ─── Facade (used by saga-module) ────────────────────────────────────────────

// Facade is the interface exported to the saga orchestrator.
type Facade interface {
	CreatePending(ctx context.Context, input usecase.CreatePendingInput) (*domain.Order, error)
	Confirm(ctx context.Context, orderID uuid.UUID) error
	Cancel(ctx context.Context, orderID uuid.UUID, reason string) error
}

type facadeAdapter struct{ m *Module }

// OrderFacade returns the Facade interface backed by this Module.
func (m *Module) OrderFacade() Facade { return &facadeAdapter{m: m} }

func (f *facadeAdapter) CreatePending(ctx context.Context, input usecase.CreatePendingInput) (*domain.Order, error) {
	return f.m.CreatePending.Execute(ctx, input)
}

func (f *facadeAdapter) Confirm(ctx context.Context, orderID uuid.UUID) error {
	return f.m.Confirm.Execute(ctx, orderID)
}

func (f *facadeAdapter) Cancel(ctx context.Context, orderID uuid.UUID, reason string) error {
	return f.m.Cancel.Execute(ctx, orderID, reason)
}
