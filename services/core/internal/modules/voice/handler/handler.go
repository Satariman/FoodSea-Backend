package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/foodsea/core/internal/modules/voice/domain"
	"github.com/foodsea/core/internal/platform/httputil"
)

type useCase interface {
	Execute(ctx context.Context, text, locale string) ([]domain.VoiceItem, []string, error)
}

type Handler struct {
	uc useCase
}

func NewHandler(uc useCase) *Handler {
	return &Handler{uc: uc}
}

// ParseVoice godoc
// @Summary      Parse a voice/text shopping list
// @Description  Forwards the user's spoken or typed shopping list to the ML service and returns matched products.
// @Tags         Voice
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      handler.ParseVoiceRequestDTO  true  "Parse voice request"
// @Success      200   {object}  httputil.Response{data=handler.ParseVoiceResponseDTO}
// @Failure      400   {object}  httputil.Response
// @Failure      401   {object}  httputil.Response
// @Failure      503   {object}  httputil.Response
// @Router       /voice/parse [post]
func (h *Handler) ParseVoice(c *gin.Context) {
	var req ParseVoiceRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, "invalid JSON")
		return
	}
	if err := req.Validate(); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	items, unmatched, err := h.uc.Execute(c.Request.Context(), req.Text, req.LocaleOrDefault())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, httputil.Response{Error: "voice service unavailable"})
		return
	}

	httputil.OK(c, ToResponseDTO(items, unmatched))
}
