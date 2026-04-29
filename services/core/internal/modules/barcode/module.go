package barcode

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/core/internal/modules/barcode/handler"
	"github.com/foodsea/core/internal/modules/catalog"
	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	cataloghandler "github.com/foodsea/core/internal/modules/catalog/handler"
)

// Deps holds barcode module external dependencies.
type Deps struct {
	ProductGetter catalog.ProductGetter
	Log           *slog.Logger
}

// Module is the barcode DI container.
type Module struct {
	h *handler.Handler
}

// NewModule wires the barcode module.
func NewModule(deps Deps) *Module {
	render := func(d *catalogdomain.ProductDetail) cataloghandler.ProductDetailResponse {
		return cataloghandler.ToProductDetailResponse(d)
	}
	return &Module{h: handler.New(deps.ProductGetter, render)}
}

// RegisterRoutes mounts GET /barcode/:code onto the public router group.
func (m *Module) RegisterRoutes(public *gin.RouterGroup) {
	public.GET("/barcode/:code", m.h.GetByBarcode)
}
