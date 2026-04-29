package handler

import (
	"context"
	"math"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/platform/httputil"
)

type productGetter interface {
	Execute(ctx context.Context, id uuid.UUID) (*domain.ProductDetail, error)
}

type productLister interface {
	Execute(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int, error)
}

type barcodeGetter interface {
	Execute(ctx context.Context, code string) (*domain.ProductDetail, error)
}

// ProductHandler handles product-related HTTP endpoints.
type ProductHandler struct {
	getProduct    productGetter
	listProducts  productLister
	getByBarcode  barcodeGetter
}

func NewProductHandler(getProduct productGetter, listProducts productLister, getByBarcode barcodeGetter) *ProductHandler {
	return &ProductHandler{
		getProduct:   getProduct,
		listProducts: listProducts,
		getByBarcode: getByBarcode,
	}
}

// GetProduct godoc
// @Summary      Get product by ID
// @Description  Returns the full product card including KBJU and related entities
// @Tags         Products
// @Produce      json
// @Param        id   path      string  true  "Product UUID"
// @Success      200  {object}  httputil.Response{data=handler.ProductDetailResponse}
// @Failure      400  {object}  httputil.Response
// @Failure      404  {object}  httputil.Response
// @Router       /products/{id} [get]
func (h *ProductHandler) GetProduct(c *gin.Context) {
	id, ok := httputil.ParseUUID(c, "id")
	if !ok {
		return
	}

	product, err := h.getProduct.Execute(c.Request.Context(), id)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toProductDetailResponse(product))
}

// ListProducts godoc
// @Summary      List products
// @Description  Returns a paginated list of products with optional filters
// @Tags         Products
// @Produce      json
// @Param        category_id     query  string  false  "Category UUID"
// @Param        subcategory_id  query  string  false  "Subcategory UUID"
// @Param        brand_id        query  string  false  "Brand UUID"
// @Param        in_stock        query  bool    false  "Only in-stock products"
// @Param        page            query  int     false  "Page number (default 1)"
// @Param        page_size       query  int     false  "Page size (default 20, max 100)"
// @Param        sort            query  string  false  "Sort: name_asc | name_desc | created_desc"
// @Success      200  {object}  httputil.Response{data=[]handler.ProductBriefResponse,meta=httputil.Meta}
// @Failure      400  {object}  httputil.Response
// @Router       /products [get]
func (h *ProductHandler) ListProducts(c *gin.Context) {
	filter := domain.ProductFilter{
		Pagination: httputil.ParsePagination(c),
		Sort:       domain.ProductSort(c.Query("sort")),
		InStockOnly: c.Query("in_stock") == "true",
	}

	if raw := c.Query("category_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httputil.BadRequest(c, "invalid category_id format")
			return
		}
		filter.CategoryID = &id
	}

	if raw := c.Query("subcategory_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httputil.BadRequest(c, "invalid subcategory_id format")
			return
		}
		filter.SubcategoryID = &id
	}

	if raw := c.Query("brand_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httputil.BadRequest(c, "invalid brand_id format")
			return
		}
		filter.BrandID = &id
	}

	products, total, err := h.listProducts.Execute(c.Request.Context(), filter)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	resp := make([]ProductBriefResponse, 0, len(products))
	for _, p := range products {
		resp = append(resp, toProductBriefResponse(p))
	}

	pageSize := filter.Pagination.PageSize
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	httputil.OKWithMeta(c, resp, &httputil.Meta{
		Page:       filter.Pagination.Page,
		PageSize:   pageSize,
		TotalCount: total,
		TotalPages: totalPages,
	})
}

// GetProductByBarcode godoc
// @Summary      Get product by barcode
// @Description  Looks up a product by EAN-8 or EAN-13 barcode
// @Tags         Products
// @Produce      json
// @Param        code  path  string  true  "EAN-8 or EAN-13 barcode"
// @Success      200   {object}  httputil.Response{data=handler.ProductDetailResponse}
// @Failure      400   {object}  httputil.Response
// @Failure      404   {object}  httputil.Response
// @Router       /products/barcode/{code} [get]
func (h *ProductHandler) GetProductByBarcode(c *gin.Context) {
	code := c.Param("code")

	product, err := h.getByBarcode.Execute(c.Request.Context(), code)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toProductDetailResponse(product))
}

