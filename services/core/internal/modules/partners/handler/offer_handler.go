package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/partners/domain"
	"github.com/foodsea/core/internal/platform/httputil"
)

type offerListExecutor interface {
	Execute(ctx context.Context, productID uuid.UUID, hasDiscountOnly bool) ([]domain.OfferWithStore, error)
}

// OfferHandler handles offer-related HTTP endpoints.
type OfferHandler struct {
	listOffers offerListExecutor
}

func NewOfferHandler(listOffers offerListExecutor) *OfferHandler {
	return &OfferHandler{listOffers: listOffers}
}

// ListOffersByProduct godoc
// @Summary      Get offers for a product
// @Description  Returns all partner store offers for a given product, sorted by price (UC-07 compare prices)
// @Tags         Partners
// @Produce      json
// @Param        id           path   string  true   "Product UUID"
// @Param        has_discount query  boolean false  "Return only offers with a discount"
// @Success      200  {object}  httputil.Response{data=[]handler.OfferResponse}
// @Failure      400  {object}  httputil.Response
// @Failure      404  {object}  httputil.Response
// @Router       /products/{id}/offers [get]
func (h *OfferHandler) ListOffersByProduct(c *gin.Context) {
	id, ok := httputil.ParseUUID(c, "id")
	if !ok {
		return
	}

	hasDiscountOnly := c.Query("has_discount") == "true"

	offers, err := h.listOffers.Execute(c.Request.Context(), id, hasDiscountOnly)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	resp := make([]OfferResponse, len(offers))
	for i, o := range offers {
		resp[i] = toOfferResponse(o)
	}
	httputil.OK(c, resp)
}
