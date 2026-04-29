package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/core/internal/modules/partners/domain"
	"github.com/foodsea/core/internal/platform/httputil"
)

type storeListExecutor interface {
	Execute(ctx context.Context) ([]domain.Store, error)
}

// StoreHandler handles store-related HTTP endpoints.
type StoreHandler struct {
	listStores storeListExecutor
}

func NewStoreHandler(listStores storeListExecutor) *StoreHandler {
	return &StoreHandler{listStores: listStores}
}

// ListStores godoc
// @Summary      List partner stores
// @Description  Returns all active partner stores sorted by name
// @Tags         Partners
// @Produce      json
// @Success      200  {object}  httputil.Response{data=[]handler.StoreResponse}
// @Failure      500  {object}  httputil.Response
// @Router       /stores [get]
func (h *StoreHandler) ListStores(c *gin.Context) {
	stores, err := h.listStores.Execute(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	resp := make([]StoreResponse, len(stores))
	for i, s := range stores {
		resp[i] = toStoreResponse(s)
	}
	httputil.OK(c, resp)
}
