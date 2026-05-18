package handler

import (
	"context"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/notifications/usecase"
	"github.com/foodsea/core/internal/platform/httputil"
	"github.com/foodsea/core/internal/platform/middleware"
)

type registerDeviceExecutor interface {
	Execute(ctx context.Context, in usecase.DeviceRegistration) error
}

type removeDevicesExecutor interface {
	Execute(ctx context.Context, userID uuid.UUID) error
}

type registerLiveActivityExecutor interface {
	Execute(ctx context.Context, in usecase.LiveActivityRegistration) error
}

type removeLiveActivityExecutor interface {
	Execute(ctx context.Context, userID, orderID uuid.UUID) error
}

// Handler contains HTTP endpoints for notifications settings.
type Handler struct {
	registerDevice       registerDeviceExecutor
	removeDevices        removeDevicesExecutor
	registerLiveActivity registerLiveActivityExecutor
	removeLiveActivity   removeLiveActivityExecutor
}

func NewHandler(
	registerDevice registerDeviceExecutor,
	removeDevices removeDevicesExecutor,
	registerLiveActivity registerLiveActivityExecutor,
	removeLiveActivity removeLiveActivityExecutor,
) *Handler {
	return &Handler{
		registerDevice:       registerDevice,
		removeDevices:        removeDevices,
		registerLiveActivity: registerLiveActivity,
		removeLiveActivity:   removeLiveActivity,
	}
}

func (h *Handler) RegisterDevice(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	var req RegisterDeviceRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	if !isHex(req.APNSToken) || len(req.APNSToken) != 64 {
		httputil.BadRequest(c, "apns_token must be a 64-char hex string")
		return
	}

	if err = h.registerDevice.Execute(c.Request.Context(), usecase.DeviceRegistration{
		UserID:      userID,
		APNSToken:   strings.ToLower(req.APNSToken),
		BundleID:    req.BundleID,
		Environment: req.Environment,
		AppVersion:  req.AppVersion,
	}); err != nil {
		httputil.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) RemoveDevices(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	if err = h.removeDevices.Execute(c.Request.Context(), userID); err != nil {
		httputil.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) RegisterLiveActivity(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	orderID := strings.TrimSpace(c.Param("orderId"))
	if orderID == "" {
		httputil.BadRequest(c, "orderId is required")
		return
	}
	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		httputil.BadRequest(c, "orderId must be a UUID")
		return
	}

	var req RegisterLiveActivityRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	if !isHex(req.PushToken) || len(req.PushToken) == 0 {
		httputil.BadRequest(c, "push_token must be a non-empty hex string")
		return
	}

	if err = h.registerLiveActivity.Execute(c.Request.Context(), usecase.LiveActivityRegistration{
		UserID:      userID,
		OrderID:     orderUUID,
		PushToken:   strings.ToLower(req.PushToken),
		BundleID:    req.BundleID,
		Environment: req.Environment,
	}); err != nil {
		httputil.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) RemoveLiveActivity(c *gin.Context) {
	userID, err := middleware.UserIDFromContext(c.Request.Context())
	if err != nil {
		httputil.HandleError(c, err)
		return
	}

	orderID := strings.TrimSpace(c.Param("orderId"))
	if orderID == "" {
		httputil.BadRequest(c, "orderId is required")
		return
	}
	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		httputil.BadRequest(c, "orderId must be a UUID")
		return
	}

	if err = h.removeLiveActivity.Execute(c.Request.Context(), userID, orderUUID); err != nil {
		httputil.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func isHex(value string) bool {
	if value == "" {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
