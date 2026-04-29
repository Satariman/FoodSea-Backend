package handler

import (
	"context"
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/search/domain"
	"github.com/foodsea/core/internal/platform/httputil"
)

type searchExecutor interface {
	Execute(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error)
}

// SearchHandler handles the GET /search endpoint.
type SearchHandler struct {
	searchProducts searchExecutor
}

func NewSearchHandler(searchProducts searchExecutor) *SearchHandler {
	return &SearchHandler{searchProducts: searchProducts}
}

// Search godoc
// @Summary      Search products
// @Description  Full-text search over products with filters, sorting, and pagination
// @Tags         Search
// @Produce      json
// @Param        q              query  string  true   "Search query (min 2 chars)"
// @Param        category_id    query  string  false  "Category UUID"
// @Param        subcategory_id query  string  false  "Subcategory UUID"
// @Param        brand_id       query  string  false  "Brand UUID"
// @Param        store_id       query  string  false  "Store UUID — limits min_price and offers_count to this store"
// @Param        min_price      query  integer false  "Minimum price in kopecks"
// @Param        max_price      query  integer false  "Maximum price in kopecks"
// @Param        in_stock       query  boolean false  "Only in-stock products"
// @Param        has_discount   query  boolean false  "Only products with at least one discounted offer"
// @Param        sort           query  string  false  "Sort: relevance | price_asc | price_desc | name_asc | name_desc | discount_desc"
// @Param        page           query  integer false  "Page number (default 1)"
// @Param        page_size      query  integer false  "Page size (default 20, max 100)"
// @Success      200  {object}  httputil.Response{data=[]handler.SearchResultItemResponse,meta=httputil.Meta}
// @Failure      400  {object}  httputil.Response
// @Router       /search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	text := c.Query("q")
	pagination := httputil.ParsePagination(c)

	q := domain.SearchQuery{
		Text:            text,
		InStockOnly:     c.Query("in_stock") == "true",
		HasDiscountOnly: c.Query("has_discount") == "true",
		Sort:            domain.SortOption(c.Query("sort")),
		Pagination:      pagination,
	}

	if raw := c.Query("category_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httputil.BadRequest(c, "invalid category_id format")
			return
		}
		q.CategoryID = &id
	}
	if raw := c.Query("subcategory_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httputil.BadRequest(c, "invalid subcategory_id format")
			return
		}
		q.SubcategoryID = &id
	}
	if raw := c.Query("brand_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httputil.BadRequest(c, "invalid brand_id format")
			return
		}
		q.BrandID = &id
	}
	if raw := c.Query("store_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httputil.BadRequest(c, "invalid store_id format")
			return
		}
		q.StoreID = &id
	}
	if raw := c.Query("min_price"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			httputil.BadRequest(c, "invalid min_price: must be a non-negative integer")
			return
		}
		q.MinPriceKopecks = &v
	}
	if raw := c.Query("max_price"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			httputil.BadRequest(c, "invalid max_price: must be a non-negative integer")
			return
		}
		q.MaxPriceKopecks = &v
	}

	result, err := h.searchProducts.Execute(c.Request.Context(), q)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	resp := make([]SearchResultItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		resp = append(resp, toSearchResultItemResponse(item))
	}

	pageSize := pagination.PageSize
	totalPages := int(math.Ceil(float64(result.Total) / float64(pageSize)))
	if totalPages == 0 && result.Total > 0 {
		totalPages = 1
	}

	httputil.OKWithMeta(c, resp, &httputil.Meta{
		Page:       pagination.Page,
		PageSize:   pageSize,
		TotalCount: result.Total,
		TotalPages: totalPages,
	})
}
