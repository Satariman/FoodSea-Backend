package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/notifications/handler"
	"github.com/foodsea/core/internal/modules/notifications/usecase"
	"github.com/foodsea/core/internal/platform/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type mockRegisterDevice struct{ mock.Mock }

type mockRemoveDevices struct{ mock.Mock }

type mockRegisterLiveActivity struct{ mock.Mock }

type mockRemoveLiveActivity struct{ mock.Mock }

func (m *mockRegisterDevice) Execute(ctx context.Context, in usecase.DeviceRegistration) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

func (m *mockRemoveDevices) Execute(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockRegisterLiveActivity) Execute(ctx context.Context, in usecase.LiveActivityRegistration) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

func (m *mockRemoveLiveActivity) Execute(ctx context.Context, userID, orderID uuid.UUID) error {
	args := m.Called(ctx, userID, orderID)
	return args.Error(0)
}

func newRouter(h *handler.Handler, userID *uuid.UUID) *gin.Engine {
	r := gin.New()
	if userID != nil {
		r.Use(func(c *gin.Context) {
			ctx := middleware.WithUserIDContext(c.Request.Context(), *userID)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		})
	}

	r.POST("/notifications/devices", h.RegisterDevice)
	r.DELETE("/notifications/devices", h.RemoveDevices)
	r.POST("/notifications/orders/:orderId/live-activity", h.RegisterLiveActivity)
	r.DELETE("/notifications/orders/:orderId/live-activity", h.RemoveLiveActivity)
	return r
}

func TestRegisterDevice_Success204(t *testing.T) {
	regD := &mockRegisterDevice{}
	remD := &mockRemoveDevices{}
	regLA := &mockRegisterLiveActivity{}
	remLA := &mockRemoveLiveActivity{}

	userID := uuid.New()
	regD.On("Execute", mock.Anything, mock.MatchedBy(func(in usecase.DeviceRegistration) bool {
		return in.UserID == userID &&
			in.BundleID == "com.foodsea.app" &&
			in.Environment == "sandbox" &&
			in.APNSToken == "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	})).Return(nil).Once()

	h := handler.NewHandler(regD, remD, regLA, remLA)
	r := newRouter(h, &userID)

	body := map[string]any{
		"apns_token":  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"bundle_id":   "com.foodsea.app",
		"environment": "sandbox",
	}
	raw, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/notifications/devices", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	regD.AssertExpectations(t)
}

func TestRegisterDevice_InvalidAPNSToken400(t *testing.T) {
	h := handler.NewHandler(&mockRegisterDevice{}, &mockRemoveDevices{}, &mockRegisterLiveActivity{}, &mockRemoveLiveActivity{})
	userID := uuid.New()
	r := newRouter(h, &userID)

	body := map[string]any{
		"apns_token":  "zz",
		"bundle_id":   "com.foodsea.app",
		"environment": "sandbox",
	}
	raw, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/notifications/devices", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegisterDevice_InvalidEnvironment400(t *testing.T) {
	h := handler.NewHandler(&mockRegisterDevice{}, &mockRemoveDevices{}, &mockRegisterLiveActivity{}, &mockRemoveLiveActivity{})
	userID := uuid.New()
	r := newRouter(h, &userID)

	body := map[string]any{
		"apns_token":  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"bundle_id":   "com.foodsea.app",
		"environment": "dev",
	}
	raw, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/notifications/devices", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveDevices_NoAuth401(t *testing.T) {
	h := handler.NewHandler(&mockRegisterDevice{}, &mockRemoveDevices{}, &mockRegisterLiveActivity{}, &mockRemoveLiveActivity{})
	r := newRouter(h, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/notifications/devices", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegisterLiveActivity_InvalidPushToken400(t *testing.T) {
	h := handler.NewHandler(&mockRegisterDevice{}, &mockRemoveDevices{}, &mockRegisterLiveActivity{}, &mockRemoveLiveActivity{})
	userID := uuid.New()
	r := newRouter(h, &userID)

	body := map[string]any{
		"push_token":  "not-hex",
		"bundle_id":   "com.foodsea.app",
		"environment": "production",
	}
	raw, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/notifications/orders/"+uuid.New().String()+"/live-activity", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveLiveActivity_Idempotent204(t *testing.T) {
	regD := &mockRegisterDevice{}
	remD := &mockRemoveDevices{}
	regLA := &mockRegisterLiveActivity{}
	remLA := &mockRemoveLiveActivity{}

	userID := uuid.New()
	orderID := uuid.New()
	remLA.On("Execute", mock.Anything, userID, orderID).Return(nil).Twice()

	h := handler.NewHandler(regD, remD, regLA, remLA)
	r := newRouter(h, &userID)

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodDelete, "/notifications/orders/"+orderID.String()+"/live-activity", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusNoContent, w1.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodDelete, "/notifications/orders/"+orderID.String()+"/live-activity", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNoContent, w2.Code)

	remLA.AssertExpectations(t)
}
