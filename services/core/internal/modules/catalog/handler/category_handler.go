package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/platform/httputil"
)

type categoryLister interface {
	Execute(ctx context.Context) ([]domain.Category, error)
}

// CategoryHandler handles category-related HTTP endpoints.
type CategoryHandler struct {
	listCategories categoryLister
}

func NewCategoryHandler(listCategories categoryLister) *CategoryHandler {
	return &CategoryHandler{listCategories: listCategories}
}

// ListCategories godoc
// @Summary      List categories
// @Description  Returns the full two-level category tree
// @Tags         Categories
// @Produce      json
// @Success      200  {object}  httputil.Response{data=[]handler.CategoryResponse}
// @Failure      500  {object}  httputil.Response
// @Router       /categories [get]
func (h *CategoryHandler) ListCategories(c *gin.Context) {
	tree, err := h.listCategories.Execute(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	resp := make([]CategoryResponse, 0, len(tree))
	for _, cat := range tree {
		resp = append(resp, toCategoryResponse(cat))
	}

	httputil.OK(c, resp)
}
