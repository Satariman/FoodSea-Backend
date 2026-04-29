package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	analogsdomain "github.com/foodsea/optimization/internal/modules/analogs/domain"
	analogsusecase "github.com/foodsea/optimization/internal/modules/analogs/usecase"
	"github.com/foodsea/optimization/internal/modules/optimizer/domain"
	"github.com/foodsea/optimization/internal/platform/httputil"
	"github.com/foodsea/optimization/internal/platform/middleware"
	sherrors "github.com/foodsea/optimization/internal/shared/errors"
)

type runOptimizationExecutor interface {
	Execute(ctx context.Context, userID uuid.UUID) (*domain.OptimizationResult, error)
}

type getResultExecutor interface {
	Execute(ctx context.Context, resultID uuid.UUID) (*domain.OptimizationResult, error)
}

type getAnalogsExecutor interface {
	Execute(ctx context.Context, productID uuid.UUID, topK int) ([]analogsdomain.Analog, error)
}

// Handler exposes optimizer HTTP endpoints.
type Handler struct {
	runOptimization runOptimizationExecutor
	getResult       getResultExecutor
	getAnalogs      getAnalogsExecutor
	log             *slog.Logger
}

func NewHandler(
	runOptimization runOptimizationExecutor,
	getResult getResultExecutor,
	getAnalogs *analogsusecase.GetAnalogsForProduct,
	log *slog.Logger,
) *Handler {
	return &Handler{
		runOptimization: runOptimization,
		getResult:       getResult,
		getAnalogs:      getAnalogs,
		log:             log,
	}
}

// RunOptimization godoc
// @Summary      Run cart optimization
// @Tags         optimization
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  httputil.Response{data=OptimizationResultResponse}
// @Failure      400  {object}  httputil.Response
// @Failure      401  {object}  httputil.Response
// @Failure      500  {object}  httputil.Response
// @Router       /optimize [post]
func (h *Handler) RunOptimization(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, sherrors.ErrUnauthorized)
		return
	}

	result, err := h.runOptimization.Execute(c.Request.Context(), userID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httputil.OK(c, toOptimizationResultResponse(result))
}

// GetResult godoc
// @Summary      Get optimization result by ID
// @Tags         optimization
// @Security     BearerAuth
// @Produce      json
// @Param        id   path  string  true  "Result UUID"
// @Success      200  {object}  httputil.Response{data=OptimizationResultResponse}
// @Failure      400  {object}  httputil.Response
// @Failure      401  {object}  httputil.Response
// @Failure      404  {object}  httputil.Response
// @Router       /optimize/{id} [get]
func (h *Handler) GetResult(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, sherrors.ErrUnauthorized)
		return
	}

	resultID, ok := httputil.ParseUUID(c, "id")
	if !ok {
		return
	}

	result, err := h.getResult.Execute(c.Request.Context(), resultID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	if result.UserID != userID {
		httputil.HandleError(c, sherrors.ErrNotFound)
		return
	}

	httputil.OK(c, toOptimizationResultResponse(result))
}

// GetAnalogs godoc
// @Summary      Get analogs for product card
// @Tags         optimization
// @Security     BearerAuth
// @Produce      json
// @Param        product_id  path   string  true   "Product UUID"
// @Param        top_k       query  int     false  "Top K analogs (default=5, max=20)"
// @Success      200  {object}  httputil.Response{data=AnalogsResponse}
// @Failure      400  {object}  httputil.Response
// @Failure      401  {object}  httputil.Response
// @Router       /analogs/{product_id} [get]
func (h *Handler) GetAnalogs(c *gin.Context) {
	if _, err := middleware.UserIDFromContext(c.Request.Context()); err != nil {
		httputil.HandleError(c, sherrors.ErrUnauthorized)
		return
	}

	productID, ok := httputil.ParseUUID(c, "product_id")
	if !ok {
		return
	}

	topK := 5
	if raw := c.Query("top_k"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			httputil.BadRequest(c, "invalid top_k")
			return
		}
		topK = value
	}
	if topK <= 0 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}

	analogs, err := h.getAnalogs.Execute(c.Request.Context(), productID, topK)
	if err != nil {
		h.writeError(c, err)
		return
	}

	resp := AnalogsResponse{Analogs: make([]AnalogDTO, len(analogs))}
	for i, analog := range analogs {
		resp.Analogs[i] = AnalogDTO{
			ProductID:       analog.ProductID.String(),
			ProductName:     analog.ProductName,
			Score:           analog.Score,
			MinPriceKopecks: analog.MinPriceKopecks,
		}
	}

	httputil.OK(c, resp)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrEmptyCart), errors.Is(err, domain.ErrNoOffers), errors.Is(err, domain.ErrNoFeasibleSolution), errors.Is(err, domain.ErrDeliveryIncomplete):
		httputil.BadRequest(c, err.Error())
	case errors.Is(err, domain.ErrResultNotFound):
		httputil.HandleError(c, sherrors.ErrNotFound)
	case errors.Is(err, domain.ErrResultLocked), errors.Is(err, domain.ErrResultNotActive), errors.Is(err, domain.ErrResultNotLocked):
		httputil.HandleError(c, sherrors.ErrConflict)
	default:
		h.log.ErrorContext(c.Request.Context(), "optimizer handler error", "error", err)
		c.JSON(http.StatusInternalServerError, httputil.Response{Error: "internal server error"})
	}
}
