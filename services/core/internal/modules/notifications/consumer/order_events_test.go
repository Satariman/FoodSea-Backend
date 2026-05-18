package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/notifications/apns"
	"github.com/foodsea/core/internal/modules/notifications/repository"
	kafkaplatform "github.com/foodsea/core/internal/platform/kafka"
)

type pushResult struct {
	outcome apns.Outcome
	err     error
}

type fakePushClient struct {
	mu      sync.Mutex
	results map[apns2.EPushType][]pushResult
	calls   []*apns2.Notification
}

func (f *fakePushClient) Push(_ string, notification *apns2.Notification) (apns.Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, notification)
	queue := f.results[notification.PushType]
	if len(queue) == 0 {
		return apns.Outcome{Class: apns.OutcomeClassSuccess, Sent: true}, nil
	}
	picked := queue[0]
	f.results[notification.PushType] = queue[1:]
	return picked.outcome, picked.err
}

func (f *fakePushClient) callsFor(pushType apns2.EPushType) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	total := 0
	for _, call := range f.calls {
		if call.PushType == pushType {
			total++
		}
	}
	return total
}

func (f *fakePushClient) firstCall(pushType apns2.EPushType) *apns2.Notification {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, call := range f.calls {
		if call.PushType == pushType {
			return call
		}
	}
	return nil
}

type fakeStore struct {
	mu sync.Mutex

	device       *repository.UserDevice
	liveActivity *repository.OrderLiveActivity

	getDeviceErr       error
	getLiveActivityErr error
	deleteDevicesErr   error
	deleteLiveErr      error

	deletedDevices      []uuid.UUID
	deletedLiveActivity []struct {
		userID  uuid.UUID
		orderID uuid.UUID
	}
}

func (f *fakeStore) GetUserDevice(_ context.Context, _ uuid.UUID) (*repository.UserDevice, error) {
	if f.getDeviceErr != nil {
		return nil, f.getDeviceErr
	}
	return f.device, nil
}

func (f *fakeStore) GetOrderLiveActivity(_ context.Context, _ uuid.UUID) (*repository.OrderLiveActivity, error) {
	if f.getLiveActivityErr != nil {
		return nil, f.getLiveActivityErr
	}
	return f.liveActivity, nil
}

func (f *fakeStore) DeleteDevices(_ context.Context, userID uuid.UUID) error {
	if f.deleteDevicesErr != nil {
		return f.deleteDevicesErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedDevices = append(f.deletedDevices, userID)
	return nil
}

func (f *fakeStore) DeleteLiveActivity(_ context.Context, userID, orderID uuid.UUID) error {
	if f.deleteLiveErr != nil {
		return f.deleteLiveErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedLiveActivity = append(f.deletedLiveActivity, struct {
		userID  uuid.UUID
		orderID uuid.UUID
	}{userID: userID, orderID: orderID})
	return nil
}

func TestHandle_SkipWhenNoDevice(t *testing.T) {
	h, push, store, _ := setupHandler(t)
	store.device = nil

	err := h.Handle(context.Background(), makeEvent(t, "e-no-device", "confirmed"))
	require.NoError(t, err)
	assert.Empty(t, push.calls)
}

func TestHandle_LiveActivityMissing_SendsOnlyAlert(t *testing.T) {
	h, push, store, _ := setupHandler(t)
	store.liveActivity = nil

	err := h.Handle(context.Background(), makeEvent(t, "e-alert-only", "confirmed"))
	require.NoError(t, err)
	assert.Equal(t, 1, push.callsFor(apns2.PushTypeAlert))
	assert.Equal(t, 0, push.callsFor(apns2.PushTypeLiveActivity))
}

func TestHandle_TransientRetryThenSuccess(t *testing.T) {
	h, push, _, sleeps := setupHandler(t)
	push.results[apns2.PushTypeAlert] = []pushResult{
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, StatusCode: 500, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, StatusCode: 429, Reason: "TooManyRequests"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassSuccess, Sent: true}},
	}

	err := h.Handle(context.Background(), makeEvent(t, "e-retry", "confirmed"))
	require.NoError(t, err)

	assert.Equal(t, 3, push.callsFor(apns2.PushTypeAlert))
	require.Len(t, *sleeps, 2)
	assert.Equal(t, time.Second, (*sleeps)[0])
	assert.Equal(t, 4*time.Second, (*sleeps)[1])
}

func TestHandle_PoisonPillAfterThreeRedeliveries(t *testing.T) {
	h, push, _, _ := setupHandler(t)
	push.results[apns2.PushTypeAlert] = []pushResult{
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
		{outcome: apns.Outcome{Class: apns.OutcomeClassTransient, Reason: "InternalServerError"}},
	}

	event := makeEvent(t, "e-poison", "confirmed")
	err1 := h.Handle(context.Background(), event)
	err2 := h.Handle(context.Background(), event)
	err3 := h.Handle(context.Background(), event)
	err4 := h.Handle(context.Background(), event)

	assert.Error(t, err1)
	assert.Error(t, err2)
	assert.Error(t, err3)
	assert.NoError(t, err4, "message should be committed after 3 redeliveries")
}

func TestHandle_InvalidTokenCleanup(t *testing.T) {
	h, push, store, _ := setupHandler(t)
	push.results[apns2.PushTypeAlert] = []pushResult{{outcome: apns.Outcome{Class: apns.OutcomeClassInvalidToken}}}
	push.results[apns2.PushTypeLiveActivity] = []pushResult{{outcome: apns.Outcome{Class: apns.OutcomeClassInvalidToken}}}

	err := h.Handle(context.Background(), makeEvent(t, "e-invalid-token", "confirmed"))
	require.NoError(t, err)

	require.Len(t, store.deletedDevices, 1)
	assert.Equal(t, store.device.UserID, store.deletedDevices[0])
	require.Len(t, store.deletedLiveActivity, 1)
	assert.Equal(t, store.liveActivity.UserID, store.deletedLiveActivity[0].userID)
	assert.Equal(t, store.liveActivity.OrderID, store.deletedLiveActivity[0].orderID)
}

func TestHandle_TerminalStatusSendsEndAndDeletesLiveActivity(t *testing.T) {
	h, push, store, _ := setupHandler(t)
	push.results[apns2.PushTypeLiveActivity] = []pushResult{{outcome: apns.Outcome{Class: apns.OutcomeClassSuccess, Sent: true}}}

	err := h.Handle(context.Background(), makeEvent(t, "e-terminal", "delivered"))
	require.NoError(t, err)

	notification := push.firstCall(apns2.PushTypeLiveActivity)
	require.NotNil(t, notification)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(notification.Payload.([]byte), &payload))
	aps, ok := payload["aps"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "end", aps["event"])

	require.Len(t, store.deletedLiveActivity, 1)
	assert.Equal(t, store.liveActivity.OrderID, store.deletedLiveActivity[0].orderID)
}

func TestHandle_APNSIDDeterministic(t *testing.T) {
	first := deterministicAPNSID("event-1", "token-1")
	second := deterministicAPNSID("event-1", "token-1")
	third := deterministicAPNSID("event-1", "token-2")

	assert.Equal(t, first, second)
	assert.NotEqual(t, first, third)
	_, err := uuid.Parse(first)
	assert.NoError(t, err)
}

func setupHandler(t *testing.T) (*Handler, *fakePushClient, *fakeStore, *[]time.Duration) {
	t.Helper()

	orderID := uuid.New()
	userID := uuid.New()

	push := &fakePushClient{results: map[apns2.EPushType][]pushResult{}}
	store := &fakeStore{
		device: &repository.UserDevice{
			UserID:      userID,
			APNSToken:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			BundleID:    "com.foodsea.app",
			Environment: "sandbox",
		},
		liveActivity: &repository.OrderLiveActivity{
			OrderID:     orderID,
			UserID:      userID,
			PushToken:   "deadbeef",
			BundleID:    "com.foodsea.app",
			Environment: "sandbox",
		},
	}

	h := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), store, push)
	h.backoffSchedule = []time.Duration{time.Second, 4 * time.Second, 16 * time.Second}
	var sleeps []time.Duration
	h.sleep = func(_ context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}

	return h, push, store, &sleeps
}

func makeEvent(t *testing.T, eventID, status string) kafkaplatform.Event {
	t.Helper()

	orderID := uuid.New()
	userID := uuid.New()

	payload := map[string]any{
		"order_id":    orderID.String(),
		"user_id":     userID.String(),
		"status":      status,
		"occurred_at": time.Now().UTC(),
	}

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	return kafkaplatform.Event{
		EventID:   eventID,
		EventType: "order.status_changed",
		Timestamp: time.Now().UTC(),
		Source:    "ordering-service",
		Payload:   raw,
	}
}

func TestHandle_PropagatesDBError(t *testing.T) {
	h, _, store, _ := setupHandler(t)
	store.getDeviceErr = errors.New("db is down")

	err := h.Handle(context.Background(), makeEvent(t, "e-db-fail", "confirmed"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading user device")
}
