package apns

import (
	"testing"

	"github.com/sideshow/apns2"
	"github.com/stretchr/testify/assert"
)

func TestClassifyReason(t *testing.T) {
	testCases := []struct {
		name   string
		reason string
		class  OutcomeClass
	}{
		{name: "unregistered", reason: apns2.ReasonUnregistered, class: OutcomeClassInvalidToken},
		{name: "bad_device_token", reason: apns2.ReasonBadDeviceToken, class: OutcomeClassInvalidToken},
		{name: "token_not_for_topic", reason: apns2.ReasonDeviceTokenNotForTopic, class: OutcomeClassInvalidToken},
		{name: "too_many_requests", reason: apns2.ReasonTooManyRequests, class: OutcomeClassTransient},
		{name: "internal_server_error", reason: apns2.ReasonInternalServerError, class: OutcomeClassTransient},
		{name: "other_reason", reason: apns2.ReasonBadTopic, class: OutcomeClassPermanent},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.class, ClassifyReason(tc.reason))
		})
	}
}

func TestOutcomeFromResponse_Sent(t *testing.T) {
	response := &apns2.Response{
		StatusCode: apns2.StatusSent,
		Reason:     "",
		ApnsID:     "apns-id",
	}

	outcome := OutcomeFromResponse(response)
	assert.True(t, outcome.Sent)
	assert.Equal(t, OutcomeClassSuccess, outcome.Class)
	assert.Equal(t, "apns-id", outcome.ApnsID)
}

func TestOutcomeFromResponse_NotSent(t *testing.T) {
	response := &apns2.Response{
		StatusCode: 410,
		Reason:     apns2.ReasonUnregistered,
		ApnsID:     "apns-id",
	}

	outcome := OutcomeFromResponse(response)
	assert.False(t, outcome.Sent)
	assert.Equal(t, OutcomeClassInvalidToken, outcome.Class)
}

func TestNewAlertNotificationHeaders(t *testing.T) {
	notification := NewAlertNotification("token", "com.foodsea.app", []byte(`{"aps":{"alert":"ok"}}`))

	assert.Equal(t, "token", notification.DeviceToken)
	assert.Equal(t, "com.foodsea.app", notification.Topic)
	assert.Equal(t, apns2.PriorityHigh, notification.Priority)
	assert.Equal(t, apns2.PushTypeAlert, notification.PushType)
}

func TestNewLiveActivityNotificationHeaders(t *testing.T) {
	notification := NewLiveActivityNotification("token", "com.foodsea.app", []byte(`{"aps":{"event":"update"}}`))

	assert.Equal(t, "token", notification.DeviceToken)
	assert.Equal(t, "com.foodsea.app.push-type.liveactivity", notification.Topic)
	assert.Equal(t, apns2.PriorityHigh, notification.Priority)
	assert.Equal(t, apns2.PushTypeLiveActivity, notification.PushType)
}
