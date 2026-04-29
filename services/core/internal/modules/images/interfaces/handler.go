package interfaces

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/core/internal/platform/httputil"
)

const maxUploadSize = 5 << 20 // 5 MB

type imageUploader interface {
	Execute(ctx context.Context, productID uuid.UUID, filename string, reader io.Reader, contentType string) (string, error)
}

type imageDeleter interface {
	Execute(ctx context.Context, productID uuid.UUID) error
}

// Handler handles product image HTTP endpoints.
type Handler struct {
	upload imageUploader
	delete imageDeleter
}

func NewHandler(upload imageUploader, delete imageDeleter) *Handler {
	return &Handler{upload: upload, delete: delete}
}

// UploadImage godoc
// @Summary      Upload product image
// @Description  Uploads an image for the product (multipart/form-data, field "image"). Max 5 MB. Allowed: image/jpeg, image/png, image/webp.
// @Tags         Admin
// @Accept       multipart/form-data
// @Produce      json
// @Param        id     path      string  true  "Product UUID"
// @Param        image  formData  file    true  "Image file"
// @Success      200    {object}  httputil.Response{data=imageURLResponse}
// @Failure      400    {object}  httputil.Response
// @Failure      404    {object}  httputil.Response
// @Failure      422    {object}  httputil.Response
// @Router       /admin/products/{id}/image [post]
func (h *Handler) UploadImage(c *gin.Context) {
	productID, ok := httputil.ParseUUID(c, "id")
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		httputil.BadRequest(c, "image field is required")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	url, err := h.upload.Execute(c.Request.Context(), productID, header.Filename, file, contentType)
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, imageURLResponse{ImageURL: url})
}

// DeleteImage godoc
// @Summary      Delete product image
// @Description  Removes the image associated with the product
// @Tags         Admin
// @Produce      json
// @Param        id  path  string  true  "Product UUID"
// @Success      204
// @Failure      404  {object}  httputil.Response
// @Router       /admin/products/{id}/image [delete]
func (h *Handler) DeleteImage(c *gin.Context) {
	productID, ok := httputil.ParseUUID(c, "id")
	if !ok {
		return
	}

	if err := h.delete.Execute(c.Request.Context(), productID); err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.NoContent(c)
}

type imageURLResponse struct {
	ImageURL string `json:"image_url"`
}
