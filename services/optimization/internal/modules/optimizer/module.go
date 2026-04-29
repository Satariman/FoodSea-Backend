package optimizer

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/grpc"

	"github.com/foodsea/optimization/ent"
	analogsdomain "github.com/foodsea/optimization/internal/modules/analogs/domain"
	analogsusecase "github.com/foodsea/optimization/internal/modules/analogs/usecase"
	"github.com/foodsea/optimization/internal/modules/optimizer/algorithm"
	"github.com/foodsea/optimization/internal/modules/optimizer/events"
	optimizergrpc "github.com/foodsea/optimization/internal/modules/optimizer/grpc"
	"github.com/foodsea/optimization/internal/modules/optimizer/handler"
	"github.com/foodsea/optimization/internal/modules/optimizer/repository"
	"github.com/foodsea/optimization/internal/modules/optimizer/usecase"
	"github.com/foodsea/optimization/internal/platform/cache"
	"github.com/foodsea/optimization/internal/platform/kafka"
	pbcore "github.com/foodsea/proto/core"
	pbopt "github.com/foodsea/proto/optimization"
)

// Deps holds dependencies needed for optimizer module.
type Deps struct {
	Ent                  *ent.Client
	CartClient           pbcore.CartServiceClient
	OfferClient          pbcore.OfferServiceClient
	AnalogProvider       analogsdomain.AnalogProvider
	GetAnalogsForProduct *analogsusecase.GetAnalogsForProduct
	Producer             *kafka.Producer
	Cache                cache.Cache
	Timeout              time.Duration
	Log                  *slog.Logger
}

// Module contains optimizer use-cases and transports.
type Module struct {
	RunOptimization *usecase.RunOptimization
	GetResult       *usecase.GetResult
	LockResult      *usecase.LockResult
	UnlockResult    *usecase.UnlockResult

	handler    *handler.Handler
	grpcServer *optimizergrpc.OptimizationServer
	repo       *repository.ResultRepo
	log        *slog.Logger
}

func NewModule(deps *Deps) *Module {
	repo := repository.NewResultRepo(deps.Ent, deps.Log)
	pub := events.NewKafkaPublisher(deps.Producer, deps.Log)
	algo := algorithm.New()

	run := usecase.NewRunOptimization(
		deps.CartClient,
		deps.OfferClient,
		deps.AnalogProvider,
		algo,
		repo,
		pub,
		deps.Cache,
		deps.Timeout,
		deps.Log,
	)
	get := usecase.NewGetResult(repo, deps.Log)
	lock := usecase.NewLockResult(repo, pub, deps.Log)
	unlock := usecase.NewUnlockResult(repo, pub, deps.Log)

	m := &Module{
		RunOptimization: run,
		GetResult:       get,
		LockResult:      lock,
		UnlockResult:    unlock,
		handler:         handler.NewHandler(run, get, deps.GetAnalogsForProduct, deps.Log),
		grpcServer:      optimizergrpc.NewOptimizationServer(get, lock, unlock, deps.Log),
		repo:            repo,
		log:             deps.Log,
	}

	return m
}

// RegisterRoutes mounts protected optimizer routes.
func (m *Module) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/optimize", m.handler.RunOptimization)
	protected.GET("/optimize/:id", m.handler.GetResult)
	protected.GET("/analogs/:product_id", m.handler.GetAnalogs)
}

// RegisterGRPC registers OptimizationService on gRPC server.
func (m *Module) RegisterGRPC(server grpc.ServiceRegistrar) {
	pbopt.RegisterOptimizationServiceServer(server, m.grpcServer)
}

// HandleCartEvent invalidates cached optimization results for the cart owner.
func (m *Module) HandleCartEvent(ctx context.Context, event *kafka.Event) error {
	if event == nil {
		return nil
	}

	if !strings.HasPrefix(event.EventType, "cart.") {
		return nil
	}

	var payload struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		m.log.WarnContext(ctx, "failed to decode cart event payload", "event_type", event.EventType, "error", err)
		return nil
	}
	if payload.UserID == "" {
		return nil
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		m.log.WarnContext(ctx, "invalid user_id in cart event", "event_type", event.EventType, "user_id", payload.UserID, "error", err)
		return nil
	}

	deleted, err := m.repo.DeleteActiveByUser(ctx, userID)
	if err != nil {
		return err
	}
	if deleted > 0 {
		m.log.InfoContext(ctx, "invalidated optimization results due cart event", "user_id", userID, "event_type", event.EventType, "deleted", deleted)
	}

	return nil
}

// ExpireOld marks active results older than cutoff as expired.
func (m *Module) ExpireOld(ctx context.Context, cutoff time.Time) (int, error) {
	return m.repo.ExpireOld(ctx, cutoff)
}
