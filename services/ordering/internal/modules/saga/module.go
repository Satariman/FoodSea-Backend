package saga

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	pb_core "github.com/foodsea/proto/core"
	pb_opt "github.com/foodsea/proto/optimization"

	"github.com/foodsea/ordering/ent"
	"github.com/foodsea/ordering/internal/modules/orders"
	ordersusecase "github.com/foodsea/ordering/internal/modules/orders/usecase"
	"github.com/foodsea/ordering/internal/modules/saga/domain"
	"github.com/foodsea/ordering/internal/modules/saga/handler"
	"github.com/foodsea/ordering/internal/modules/saga/infra"
	"github.com/foodsea/ordering/internal/modules/saga/repository"
	"github.com/foodsea/ordering/internal/platform/kafka"
)

// Deps holds all external dependencies needed by the saga module.
type Deps struct {
	Ent             *ent.Client
	OrdersFacade    orders.Facade
	CartClient      pb_core.CartServiceClient
	OptClient       pb_opt.OptimizationServiceClient
	CommandProducer *kafka.Producer // saga.commands topic
	ReplyProducer   *kafka.Producer // saga.replies topic (for audit)
	ReplyConsumer   *kafka.Consumer // saga.replies topic (consumer)
	Log             *slog.Logger
	StepTimeout     time.Duration
	MaxCompAttempts int
}

// Module wires all saga layers together.
type Module struct {
	Orchestrator  *Orchestrator
	replyConsumer *ReplyConsumer
	handler       *handler.SagaHandler
	repo          *repository.SagaRepo
}

// NewModule constructs the Module.
func NewModule(deps Deps) *Module {
	repo := repository.NewSagaRepo(deps.Ent)

	cartClient := infra.NewCoreClient(deps.CartClient)
	optClient := infra.NewOptimizationClient(deps.OptClient)
	auditPub := infra.NewAuditPublisher(deps.CommandProducer, deps.ReplyProducer, deps.Log)

	ordersParticipant := newOrdersParticipantAdapter(deps.OrdersFacade)

	orch := NewOrchestrator(
		repo,
		cartClient,
		optClient,
		ordersParticipant,
		auditPub,
		deps.Log,
		OrchestratorConfig{
			StepTimeout:     deps.StepTimeout,
			MaxCompAttempts: deps.MaxCompAttempts,
		},
	)

	h := handler.NewSagaHandler(orch, repo)
	rc := NewReplyConsumer(deps.ReplyConsumer, deps.Log)

	return &Module{
		Orchestrator:  orch,
		replyConsumer: rc,
		handler:       h,
		repo:          repo,
	}
}

// RegisterRoutes mounts saga HTTP routes on the protected router group.
func (m *Module) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/orders", m.handler.PlaceOrder)
	protected.GET("/orders/:id/saga", m.handler.GetSagaState)
}

// RunConsumer starts the saga.replies consumer loop. Blocks until ctx is done.
func (m *Module) RunConsumer(ctx context.Context) error {
	return m.replyConsumer.Run(ctx)
}

// RecoverPending resumes all saga states that were left pending or compensating
// (e.g., after a service restart). Should be called once at startup.
func (m *Module) RecoverPending(ctx context.Context) error {
	pending, err := m.repo.ListPending(ctx)
	if err != nil {
		return err
	}
	for _, s := range pending {
		m.Orchestrator.log.InfoContext(ctx, "recovering saga",
			"saga_id", s.ID,
			"status", s.Status,
			"current_step", s.CurrentStep,
		)
		if _, err := m.Orchestrator.Resume(ctx, s); err != nil {
			m.Orchestrator.log.ErrorContext(ctx, "saga recovery failed",
				"saga_id", s.ID, "error", err)
		}
	}
	return nil
}

// ─── ordersParticipantAdapter ─────────────────────────────────────────────────

// ordersParticipantAdapter adapts the orders.Facade to domain.OrdersParticipant,
// converting between saga domain types and orders usecase types.
type ordersParticipantAdapter struct {
	facade orders.Facade
}

func newOrdersParticipantAdapter(facade orders.Facade) *ordersParticipantAdapter {
	return &ordersParticipantAdapter{facade: facade}
}

func (a *ordersParticipantAdapter) CreatePending(ctx context.Context, input domain.CreatePendingInput) (*domain.Order, error) {
	ucItems := make([]ordersusecase.OrderItemSnapshot, len(input.Items))
	for i, it := range input.Items {
		ucItems[i] = ordersusecase.OrderItemSnapshot{
			ProductID:    it.ProductID,
			ProductName:  it.ProductName,
			StoreID:      it.StoreID,
			StoreName:    it.StoreName,
			Quantity:     it.Quantity,
			PriceKopecks: it.PriceKopecks,
		}
	}
	order, err := a.facade.CreatePending(ctx, ordersusecase.CreatePendingInput{
		UserID:               input.UserID,
		OptimizationResultID: input.OptimizationResultID,
		Items:                ucItems,
		TotalKopecks:         input.TotalKopecks,
		DeliveryKopecks:      input.DeliveryKopecks,
	})
	if err != nil {
		return nil, err
	}
	return &domain.Order{ID: order.ID}, nil
}

func (a *ordersParticipantAdapter) Confirm(ctx context.Context, orderID uuid.UUID) error {
	return a.facade.Confirm(ctx, orderID)
}

func (a *ordersParticipantAdapter) Cancel(ctx context.Context, orderID uuid.UUID, reason string) error {
	return a.facade.Cancel(ctx, orderID, reason)
}
