package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/cart/domain"
	"github.com/foodsea/core/internal/platform/httputil"
	"github.com/foodsea/core/internal/platform/middleware"
)

type getCartExecutor interface {
	Execute(ctx context.Context, userID uuid.UUID) (*domain.Cart, error)
}

type addItemExecutor interface {
	Execute(ctx context.Context, userID, productID uuid.UUID, qty int16) error
}

type updateItemExecutor interface {
	Execute(ctx context.Context, userID, productID uuid.UUID, qty int16) error
}

type removeItemExecutor interface {
	Execute(ctx context.Context, userID, productID uuid.UUID) error
}

type clearCartExecutor interface {
	Execute(ctx context.Context, userID uuid.UUID) error
}

// CartHandler handles all HTTP cart endpoints.
type CartHandler struct {
	getCart    getCartExecutor
	addItem    addItemExecutor
	updateItem updateItemExecutor
	removeItem removeItemExecutor
	clearCart  clearCartExecutor
}

func NewCartHandler(
	getCart getCartExecutor,
	addItem addItemExecutor,
	updateItem updateItemExecutor,
	removeItem removeItemExecutor,
	clearCart clearCartExecutor,
) *CartHandler {
	return &CartHandler{
		getCart:    getCart,
		addItem:    addItem,
		updateItem: updateItem,
		removeItem: removeItem,
		clearCart:  clearCart,
	}
}

// GetCart godoc
// @Summary      Get current cart
// @Description  Returns the authenticated user's cart with all items
// @Tags         Cart
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  httputil.Response{data=handler.CartResponse}
// @Failure      401  {object}  httputil.Response
// @Failure      500  {object}  httputil.Response
// @Router       /cart [get]
func (h *CartHandler) GetCart(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	cart, err := h.getCart.Execute(c.Request.Context(), userID)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toCartResponse(cart))
}

// AddItem godoc
// @Summary      Add item to cart
// @Description  Adds a product to the cart; increments quantity if already present
// @Tags         Cart
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      handler.AddItemRequest  true  "Add item request"
// @Success      201   {object}  httputil.Response{data=handler.CartResponse}
// @Failure      400   {object}  httputil.Response
// @Failure      401   {object}  httputil.Response
// @Failure      404   {object}  httputil.Response
// @Failure      422   {object}  httputil.Response
// @Failure      500   {object}  httputil.Response
// @Router       /cart/items [post]
func (h *CartHandler) AddItem(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	var req AddItemRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		httputil.BadRequest(c, "invalid product_id format: must be a UUID")
		return
	}

	if err = h.addItem.Execute(c.Request.Context(), userID, productID, req.Quantity); err != nil {
		httputil.HandleError(c, err)
		return
	}

	cart, err := h.getCart.Execute(c.Request.Context(), userID)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.Created(c, toCartResponse(cart))
}

// UpdateItem godoc
// @Summary      Update item quantity
// @Description  Sets a new quantity for the given product in the cart
// @Tags         Cart
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        product_id  path      string                   true  "Product UUID"
// @Param        body        body      handler.UpdateItemRequest  true  "Update quantity"
// @Success      200         {object}  httputil.Response{data=handler.CartResponse}
// @Failure      400         {object}  httputil.Response
// @Failure      401         {object}  httputil.Response
// @Failure      404         {object}  httputil.Response
// @Failure      422         {object}  httputil.Response
// @Failure      500         {object}  httputil.Response
// @Router       /cart/items/{product_id} [put]
func (h *CartHandler) UpdateItem(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	productID, ok := httputil.ParseUUID(c, "product_id")
	if !ok {
		return
	}

	var req UpdateItemRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	if err = h.updateItem.Execute(c.Request.Context(), userID, productID, req.Quantity); err != nil {
		httputil.HandleError(c, err)
		return
	}

	cart, err := h.getCart.Execute(c.Request.Context(), userID)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toCartResponse(cart))
}

// RemoveItem godoc
// @Summary      Remove item from cart
// @Description  Removes a product from the cart (idempotent)
// @Tags         Cart
// @Produce      json
// @Security     BearerAuth
// @Param        product_id  path  string  true  "Product UUID"
// @Success      204
// @Failure      400  {object}  httputil.Response
// @Failure      401  {object}  httputil.Response
// @Failure      500  {object}  httputil.Response
// @Router       /cart/items/{product_id} [delete]
func (h *CartHandler) RemoveItem(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	productID, ok := httputil.ParseUUID(c, "product_id")
	if !ok {
		return
	}

	if err = h.removeItem.Execute(c.Request.Context(), userID, productID); err != nil {
		httputil.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ClearCart godoc
// @Summary      Clear cart
// @Description  Removes all items from the authenticated user's cart
// @Tags         Cart
// @Produce      json
// @Security     BearerAuth
// @Success      204
// @Failure      401  {object}  httputil.Response
// @Failure      500  {object}  httputil.Response
// @Router       /cart [delete]
func (h *CartHandler) ClearCart(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	if err = h.clearCart.Execute(c.Request.Context(), userID); err != nil {
		httputil.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
