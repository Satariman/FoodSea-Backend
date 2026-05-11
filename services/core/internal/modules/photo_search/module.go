package photo_search

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/photo_search/grpc"
	"github.com/foodsea/core/internal/modules/photo_search/handler"
	"github.com/foodsea/core/internal/modules/photo_search/usecase"
	pbml "github.com/foodsea/proto/ml"
)

type productLoader interface {
	Execute(ctx context.Context, id uuid.UUID) (*catalogdomain.ProductDetail, error)
}

type Deps struct {
	MLClient      pbml.AnalogServiceClient
	ProductLoader productLoader
	MaxImageBytes int64
}

type Module struct {
	SearchByPhoto *usecase.SearchByPhoto
	handler       *handler.Handler
}

func NewModule(deps Deps) *Module {
	mlClient := grpc.NewMLClient(deps.MLClient)
	searchUC := usecase.NewSearchByPhoto(mlClient, deps.ProductLoader)
	h := handler.NewHandler(searchUC, deps.MaxImageBytes)

	return &Module{
		SearchByPhoto: searchUC,
		handler:       h,
	}
}

func (m *Module) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/products/photo-search", m.handler.SearchByPhoto)
}
