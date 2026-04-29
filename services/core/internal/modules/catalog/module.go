package catalog

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	googlegrpc "google.golang.org/grpc"

	pb "github.com/foodsea/proto/core"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/catalog/domain"
	cataloggrpc "github.com/foodsea/core/internal/modules/catalog/grpc"
	"github.com/foodsea/core/internal/modules/catalog/handler"
	"github.com/foodsea/core/internal/modules/catalog/repository"
	"github.com/foodsea/core/internal/modules/catalog/usecase"
	"github.com/foodsea/core/internal/platform/cache"
)

// ProductGetter is the interface consumed by the barcode module.
type ProductGetter interface {
	ByBarcode(ctx context.Context, code string) (*domain.ProductDetail, error)
}

// Deps holds the external dependencies required to build the catalog module.
type Deps struct {
	Ent               *ent.Client
	Cache             cache.Cache
	Log               *slog.Logger
	BestOfferProvider domain.BestOfferProvider // optional; enables best_offer on product card
}

// Module is the DI container for the catalog module.
type Module struct {
	ListCategories      *usecase.ListCategories
	ListProducts        *usecase.ListProducts
	GetProduct          *usecase.GetProduct
	GetProductByBarcode *usecase.GetProductByBarcode
	ListBrands          *usecase.ListBrands

	categoryHandler *handler.CategoryHandler
	productHandler  *handler.ProductHandler
	brandHandler    *handler.BrandHandler
	catalogServer   *cataloggrpc.CatalogServer
}

// NewModule wires all catalog dependencies and returns a ready-to-use Module.
func NewModule(deps Deps) *Module {
	categoryRepo := repository.NewCategoryRepo(deps.Ent)
	brandRepo := repository.NewBrandRepo(deps.Ent)
	productRepo := repository.NewProductRepo(deps.Ent)
	productCache := repository.NewProductCache(deps.Cache)

	listCategoriesUC := usecase.NewListCategories(categoryRepo, productCache, deps.Log)
	listProductsUC := usecase.NewListProducts(productRepo, categoryRepo, deps.Log)
	getProductUC := usecase.NewGetProduct(productRepo, productCache, deps.BestOfferProvider, deps.Log)
	getProductByBarcodeUC := usecase.NewGetProductByBarcode(productRepo, productCache, deps.Log)
	listBrandsUC := usecase.NewListBrands(brandRepo, deps.Log)

	categoryH := handler.NewCategoryHandler(listCategoriesUC)
	productH := handler.NewProductHandler(getProductUC, listProductsUC, getProductByBarcodeUC)
	brandH := handler.NewBrandHandler(listBrandsUC)
	catalogSrv := cataloggrpc.NewCatalogServer(productRepo, deps.Log)

	return &Module{
		ListCategories:      listCategoriesUC,
		ListProducts:        listProductsUC,
		GetProduct:          getProductUC,
		GetProductByBarcode: getProductByBarcodeUC,
		ListBrands:          listBrandsUC,
		categoryHandler:     categoryH,
		productHandler:      productH,
		brandHandler:        brandH,
		catalogServer:       catalogSrv,
	}
}

// RegisterRoutes mounts all catalog public routes onto the given router group.
// Note: the barcode lookup route (/barcode/:code) is registered by the barcode module,
// which delegates to ProductGetter — it is NOT registered here to avoid Gin param conflicts.
func (m *Module) RegisterRoutes(public *gin.RouterGroup) {
	public.GET("/categories", m.categoryHandler.ListCategories)
	public.GET("/brands", m.brandHandler.ListBrands)
	public.GET("/products", m.productHandler.ListProducts)
	public.GET("/products/:id", m.productHandler.GetProduct)
}

// ProductGetter returns a ProductGetter backed by the GetProductByBarcode use case.
func (m *Module) ProductGetter() ProductGetter {
	return m.GetProductByBarcode
}

// RegisterGRPC registers the CatalogService gRPC server.
func (m *Module) RegisterGRPC(srv *googlegrpc.Server) {
	pb.RegisterCatalogServiceServer(srv, m.catalogServer)
}
