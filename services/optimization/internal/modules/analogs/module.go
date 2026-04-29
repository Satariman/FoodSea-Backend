package analogs

import (
	"log/slog"

	"github.com/foodsea/optimization/internal/modules/analogs/domain"
	analoggrpc "github.com/foodsea/optimization/internal/modules/analogs/grpc"
	"github.com/foodsea/optimization/internal/modules/analogs/usecase"
	"github.com/foodsea/optimization/internal/platform/cache"
	pbml "github.com/foodsea/proto/ml"
)

// Deps holds external dependencies for analogs module.
type Deps struct {
	MLClient pbml.AnalogServiceClient
	Cache    cache.Cache
	Log      *slog.Logger
}

// Module bundles analogs use cases and provider adapter.
type Module struct {
	GetAnalogsForProduct *usecase.GetAnalogsForProduct
	Provider             domain.AnalogProvider
}

func NewModule(deps Deps) *Module {
	provider := analoggrpc.NewMLClient(deps.MLClient, deps.Log)
	return &Module{
		GetAnalogsForProduct: usecase.NewGetAnalogsForProduct(provider, deps.Cache, deps.Log),
		Provider:             provider,
	}
}
