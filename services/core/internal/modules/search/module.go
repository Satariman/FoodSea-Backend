package search

import (
	"database/sql"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/search/handler"
	"github.com/foodsea/core/internal/modules/search/repository"
	"github.com/foodsea/core/internal/modules/search/usecase"
	"github.com/foodsea/core/internal/platform/cache"
)

// Deps holds the external dependencies required to build the search module.
type Deps struct {
	Ent   *ent.Client
	DB    *sql.DB
	Cache cache.Cache
	Log   *slog.Logger
}

// Module is the DI container for the search module.
type Module struct {
	SearchProducts *usecase.SearchProducts
	searchHandler  *handler.SearchHandler
}

// NewModule wires all search dependencies and returns a ready-to-use Module.
func NewModule(deps Deps) *Module {
	searchRepo := repository.NewSearchRepo(deps.DB)
	searchCache := repository.NewSearchCache(deps.Cache)

	searchProductsUC := usecase.NewSearchProducts(searchRepo, searchCache, deps.Log)
	searchH := handler.NewSearchHandler(searchProductsUC)

	return &Module{
		SearchProducts: searchProductsUC,
		searchHandler:  searchH,
	}
}

// RegisterRoutes mounts the search public route onto the given router group.
func (m *Module) RegisterRoutes(public *gin.RouterGroup) {
	public.GET("/search", m.searchHandler.Search)
}
