package httputil

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	sherrors "github.com/foodsea/core/internal/shared/errors"
)

// Response is the standard JSON envelope for all API responses.
type Response struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
	Meta  *Meta  `json:"meta,omitempty"`
}

// Meta carries pagination metadata.
type Meta struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{Data: data})
}

func OKWithMeta(c *gin.Context, data any, meta *Meta) {
	c.JSON(http.StatusOK, Response{Data: data, Meta: meta})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Response{Error: msg})
}

// HandleError maps sentinel errors to appropriate HTTP status codes.
// Unknown errors are logged at ERROR and returned as 500.
func HandleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sherrors.ErrNotFound):
		c.JSON(http.StatusNotFound, Response{Error: "resource not found"})
	case errors.Is(err, sherrors.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	case errors.Is(err, sherrors.ErrAlreadyExists):
		c.JSON(http.StatusConflict, Response{Error: "resource already exists"})
	case errors.Is(err, sherrors.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, Response{Error: "unauthorized"})
	case errors.Is(err, sherrors.ErrConflict):
		c.JSON(http.StatusConflict, Response{Error: err.Error()})
	default:
		var ve *sherrors.ValidationError
		if errors.As(err, &ve) {
			c.JSON(http.StatusUnprocessableEntity, Response{Error: ve.Error()})
			return
		}
		slog.ErrorContext(c.Request.Context(), "internal error", "error", err)
		c.JSON(http.StatusInternalServerError, Response{Error: "internal server error"})
	}
}
