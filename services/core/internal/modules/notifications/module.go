package notifications

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/notifications/apns"
	"github.com/foodsea/core/internal/modules/notifications/consumer"
	"github.com/foodsea/core/internal/modules/notifications/handler"
	"github.com/foodsea/core/internal/modules/notifications/repository"
	"github.com/foodsea/core/internal/modules/notifications/usecase"
	kafkaplatform "github.com/foodsea/core/internal/platform/kafka"
)

// Deps holds external dependencies required to build the notifications module.
type Deps struct {
	Ent          *ent.Client
	Log          *slog.Logger
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
	APNS         *apns.Client
}

// Module is the DI container for notifications.
type Module struct {
	RegisterDevice       *usecase.RegisterDevice
	RemoveDevices        *usecase.RemoveDevices
	RegisterLiveActivity *usecase.RegisterLiveActivity
	RemoveLiveActivity   *usecase.RemoveLiveActivity

	orderEventsConsumer *consumer.Runner

	handler *handler.Handler
}

// NewModule wires notifications dependencies and returns a ready module.
func NewModule(deps Deps) *Module {
	repo := repository.NewRepository(deps.Ent)

	registerDeviceUC := usecase.NewRegisterDevice(repo)
	removeDevicesUC := usecase.NewRemoveDevices(repo)
	registerLiveActivityUC := usecase.NewRegisterLiveActivity(repo)
	removeLiveActivityUC := usecase.NewRemoveLiveActivity(repo)

	h := handler.NewHandler(registerDeviceUC, removeDevicesUC, registerLiveActivityUC, removeLiveActivityUC)
	orderEventsHandler := consumer.NewHandler(deps.Log, repo, deps.APNS)
	kafkaConsumer := kafkaplatform.NewConsumer(deps.KafkaBrokers, deps.KafkaTopic, deps.KafkaGroupID, deps.Log)
	orderEventsConsumer := consumer.NewRunner(kafkaConsumer, orderEventsHandler)

	return &Module{
		RegisterDevice:       registerDeviceUC,
		RemoveDevices:        removeDevicesUC,
		RegisterLiveActivity: registerLiveActivityUC,
		RemoveLiveActivity:   removeLiveActivityUC,
		orderEventsConsumer:  orderEventsConsumer,
		handler:              h,
	}
}

// RegisterRoutes mounts notifications routes on protected router group.
func (m *Module) RegisterRoutes(protected *gin.RouterGroup) {
	n := protected.Group("/notifications")
	n.POST("/devices", m.handler.RegisterDevice)
	n.DELETE("/devices", m.handler.RemoveDevices)
	n.POST("/orders/:orderId/live-activity", m.handler.RegisterLiveActivity)
	n.DELETE("/orders/:orderId/live-activity", m.handler.RemoveLiveActivity)
}

func (m *Module) RunOrderEventsConsumer(ctx context.Context) error {
	return m.orderEventsConsumer.Run(ctx)
}

func (m *Module) CloseOrderEventsConsumer() error {
	return m.orderEventsConsumer.Close()
}
