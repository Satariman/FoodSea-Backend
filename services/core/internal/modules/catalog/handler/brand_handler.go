package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/platform/httputil"
)

type brandLister interface {
	Execute(ctx context.Context) ([]domain.Brand, error)
}

// BrandHandler handles brand-related HTTP endpoints.
type BrandHandler struct {
	listBrands brandLister
}

func NewBrandHandler(listBrands brandLister) *BrandHandler {
	return &BrandHandler{listBrands: listBrands}
}

// ListBrands godoc
// @Summary      List brands
// @Description  Returns all brands for use on the filter screen
// @Tags         Brands
// @Produce      json
// @Success      200  {object}  httputil.Response{data=[]handler.BrandResponse}
// @Failure      500  {object}  httputil.Response
// @Router       /brands [get]
func (h *BrandHandler) ListBrands(c *gin.Context) {
	brands, err := h.listBrands.Execute(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	resp := make([]BrandResponse, 0, len(brands))
	for _, b := range brands {
		resp = append(resp, toBrandResponse(b))
	}

	httputil.OK(c, resp)
}
