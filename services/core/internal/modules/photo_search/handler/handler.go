package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/core/internal/modules/photo_search/domain"
	"github.com/foodsea/core/internal/platform/httputil"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type searchByPhotoExecutor interface {
	Execute(ctx context.Context, req domain.SearchByPhotoRequest) (domain.SearchResult, error)
}

type Handler struct {
	searchByPhoto searchByPhotoExecutor
	maxImageBytes int64
}

func NewHandler(searchByPhoto searchByPhotoExecutor, maxImageBytes int64) *Handler {
	return &Handler{searchByPhoto: searchByPhoto, maxImageBytes: maxImageBytes}
}

// SearchByPhoto godoc
// @Summary      Search products by photo
// @Description  Searches product candidates by image and OCR text
// @Tags         Products
// @Accept       multipart/form-data
// @Produce      json
// @Param        image     formData  file    true   "Product image (jpeg/png)"
// @Param        ocr_text  formData  string  true   "OCR text extracted on device"
// @Param        top_k     formData  integer false  "Top-K candidates, default 5"
// @Success      200       {object}  httputil.Response{data=searchByPhotoResponse}
// @Failure      400       {object}  httputil.Response
// @Failure      413       {object}  httputil.Response
// @Failure      415       {object}  httputil.Response
// @Failure      422       {object}  httputil.Response
// @Failure      503       {object}  httputil.Response
// @Router       /products/photo-search [post]
func (h *Handler) SearchByPhoto(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxImageBytes)

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, httputil.Response{Error: "image is too large"})
			return
		}
		httputil.BadRequest(c, "image field is required")
		return
	}
	defer file.Close()

	mimeType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		c.JSON(http.StatusUnsupportedMediaType, httputil.Response{Error: "unsupported image mime type"})
		return
	}

	rawTopK := strings.TrimSpace(c.PostForm("top_k"))
	topK := 0
	if rawTopK != "" {
		value, parseErr := strconv.Atoi(rawTopK)
		if parseErr != nil {
			httputil.HandleError(c, &sherrors.ValidationError{Field: "top_k", Message: "must be an integer"})
			return
		}
		topK = value
	}

	ocrText := strings.TrimSpace(c.PostForm("ocr_text"))
	if ocrText == "" {
		httputil.BadRequest(c, "ocr_text is required")
		return
	}

	imageBytes, err := io.ReadAll(file)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, httputil.Response{Error: "image is too large"})
			return
		}
		httputil.BadRequest(c, "failed to read image")
		return
	}

	result, err := h.searchByPhoto.Execute(c.Request.Context(), domain.SearchByPhotoRequest{
		Image:         imageBytes,
		ImageMimeType: mimeType,
		OCRText:       ocrText,
		TopK:          topK,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	httputil.OK(c, toResponse(result))
}
