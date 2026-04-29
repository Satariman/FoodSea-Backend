package partners

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	googlegrpc "google.golang.org/grpc"

	pb "github.com/foodsea/proto/core"

	"github.com/foodsea/core/ent"
	partgrpc "github.com/foodsea/core/internal/modules/partners/grpc"
	"github.com/foodsea/core/internal/modules/partners/handler"
	"github.com/foodsea/core/internal/modules/partners/repository"
	"github.com/foodsea/core/internal/modules/partners/usecase"
	"github.com/foodsea/core/internal/platform/cache"
)

// Deps holds external dependencies required to build the partners module.
type Deps struct {
	Ent   *ent.Client
	Cache cache.Cache
	Log   *slog.Logger
}

// Module is the DI container for the partners module.
type Module struct {
	ListStores            *usecase.ListStores
	ListOffersByProduct   *usecase.ListOffersByProduct
	GetOffersForProducts  *usecase.GetOffersForProducts
	GetDeliveryConditions *usecase.GetDeliveryConditions
	GetBestOffer          *usecase.GetBestOffer

	storeHandler *handler.StoreHandler
	offerHandler *handler.OfferHandler
	offerServer  *partgrpc.OfferServer
}

// NewModule wires all partners dependencies and returns a ready-to-use Module.
func NewModule(deps Deps) *Module {
	storeRepo := repository.NewStoreRepo(deps.Ent)
	offerRepo := repository.NewOfferRepo(deps.Ent)
	deliveryRepo := repository.NewDeliveryRepo(deps.Ent)
	offerCache := repository.NewOfferCache(deps.Cache)

	listStoresUC := usecase.NewListStores(storeRepo, deps.Log)
	listOffersUC := usecase.NewListOffersByProduct(offerRepo, storeRepo, offerCache, deps.Log)
	getOffersUC := usecase.NewGetOffersForProducts(offerRepo, storeRepo, deps.Log)
	getDeliveryUC := usecase.NewGetDeliveryConditions(deliveryRepo, deps.Log)
	getBestOfferUC := usecase.NewGetBestOffer(offerRepo, storeRepo)

	storeH := handler.NewStoreHandler(listStoresUC)
	offerH := handler.NewOfferHandler(listOffersUC)
	offerSrv := partgrpc.NewOfferServer(getOffersUC, getDeliveryUC, deps.Log)

	return &Module{
		ListStores:            listStoresUC,
		ListOffersByProduct:   listOffersUC,
		GetOffersForProducts:  getOffersUC,
		GetDeliveryConditions: getDeliveryUC,
		GetBestOffer:          getBestOfferUC,
		storeHandler:          storeH,
		offerHandler:          offerH,
		offerServer:           offerSrv,
	}
}

// RegisterRoutes mounts all partners public routes onto the given router group.
func (m *Module) RegisterRoutes(public *gin.RouterGroup) {
	public.GET("/stores", m.storeHandler.ListStores)
	public.GET("/products/:id/offers", m.offerHandler.ListOffersByProduct)
}

// RegisterGRPC registers the OfferService gRPC server.
func (m *Module) RegisterGRPC(srv *googlegrpc.Server) {
	pb.RegisterOfferServiceServer(srv, m.offerServer)
}
