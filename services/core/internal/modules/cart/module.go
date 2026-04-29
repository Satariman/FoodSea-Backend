package cart

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	googlegrpc "google.golang.org/grpc"

	pb "github.com/foodsea/proto/core"

	"github.com/foodsea/core/ent"
	cartgrpc "github.com/foodsea/core/internal/modules/cart/grpc"
	"github.com/foodsea/core/internal/modules/cart/events"
	"github.com/foodsea/core/internal/modules/cart/handler"
	"github.com/foodsea/core/internal/modules/cart/repository"
	"github.com/foodsea/core/internal/modules/cart/usecase"
	"github.com/foodsea/core/internal/platform/kafka"
)

// Deps holds external dependencies required to build the cart module.
type Deps struct {
	Ent      *ent.Client
	Producer *kafka.Producer
	Log      *slog.Logger
}

// Module is the DI container for the cart module.
type Module struct {
	GetCart     *usecase.GetCart
	AddItem     *usecase.AddItem
	UpdateItem  *usecase.UpdateItem
	RemoveItem  *usecase.RemoveItem
	ClearCart   *usecase.ClearCart
	RestoreCart *usecase.RestoreCart

	httpHandler *handler.CartHandler
	cartServer  *cartgrpc.CartServer
}

// NewModule wires all cart dependencies and returns a ready-to-use Module.
func NewModule(deps Deps) *Module {
	repo := repository.NewCartRepo(deps.Ent)
	publisher := events.NewKafkaPublisher(deps.Producer)

	getCartUC := usecase.NewGetCart(repo)
	addItemUC := usecase.NewAddItem(repo, publisher, deps.Log)
	updateItemUC := usecase.NewUpdateItem(repo, publisher, deps.Log)
	removeItemUC := usecase.NewRemoveItem(repo, publisher, deps.Log)
	clearCartUC := usecase.NewClearCart(repo, publisher, deps.Log)
	restoreCartUC := usecase.NewRestoreCart(repo, deps.Log)

	h := handler.NewCartHandler(getCartUC, addItemUC, updateItemUC, removeItemUC, clearCartUC)
	srv := cartgrpc.NewCartServer(getCartUC, clearCartUC, restoreCartUC, deps.Log)

	return &Module{
		GetCart:     getCartUC,
		AddItem:     addItemUC,
		UpdateItem:  updateItemUC,
		RemoveItem:  removeItemUC,
		ClearCart:   clearCartUC,
		RestoreCart: restoreCartUC,
		httpHandler: h,
		cartServer:  srv,
	}
}

// RegisterRoutes mounts all cart routes onto the protected router group.
func (m *Module) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/cart", m.httpHandler.GetCart)
	protected.POST("/cart/items", m.httpHandler.AddItem)
	protected.PUT("/cart/items/:product_id", m.httpHandler.UpdateItem)
	protected.DELETE("/cart/items/:product_id", m.httpHandler.RemoveItem)
	protected.DELETE("/cart", m.httpHandler.ClearCart)
}

// RegisterGRPC registers the CartService gRPC server.
func (m *Module) RegisterGRPC(srv *googlegrpc.Server) {
	pb.RegisterCartServiceServer(srv, m.cartServer)
}
