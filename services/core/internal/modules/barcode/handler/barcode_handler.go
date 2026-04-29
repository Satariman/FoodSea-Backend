package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	catalogdto "github.com/foodsea/core/internal/modules/catalog/handler"
	"github.com/foodsea/core/internal/platform/httputil"
)

// ProductByBarcode is satisfied by catalog.GetProductByBarcode.
type ProductByBarcode interface {
	ByBarcode(ctx context.Context, code string) (*catalogdomain.ProductDetail, error)
}

// Handler handles GET /barcode/:code.
type Handler struct {
	getter ProductByBarcode
	render func(*catalogdomain.ProductDetail) catalogdto.ProductDetailResponse
}

// New creates a barcode Handler.
func New(getter ProductByBarcode, render func(*catalogdomain.ProductDetail) catalogdto.ProductDetailResponse) *Handler {
	return &Handler{getter: getter, render: render}
}

// GetByBarcode godoc
// @Summary     Получить товар по штрихкоду
// @Description Возвращает карточку товара по его штрихкоду (EAN-8 или EAN-13)
// @Tags        Barcode
// @Produce     json
// @Param       code path string true "Штрихкод (8 или 13 цифр)"
// @Success     200 {object} httputil.Response{data=catalogdto.ProductDetailResponse}
// @Failure     400 {object} httputil.Response{error=string}
// @Failure     404 {object} httputil.Response{error=string}
// @Router      /barcode/{code} [get]
func (h *Handler) GetByBarcode(c *gin.Context) {
	code := c.Param("code")
	detail, err := h.getter.ByBarcode(c.Request.Context(), code)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}
	httputil.OK(c, h.render(detail))
}
