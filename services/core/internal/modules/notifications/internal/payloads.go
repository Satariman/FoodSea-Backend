package internal

import (
	"encoding/json"
	"time"
)

const (
	LiveActivityEventUpdate = "update"
	LiveActivityEventEnd    = "end"
)

type alertAPS struct {
	Alert string `json:"alert"`
}

type alertPayload struct {
	APS     alertAPS    `json:"aps"`
	OrderID string      `json:"order_id"`
	Status  OrderStatus `json:"status"`
}

type liveActivityContentState struct {
	OrderID string      `json:"order_id"`
	Status  OrderStatus `json:"status"`
}

type liveActivityAPS struct {
	Timestamp     int64                    `json:"timestamp"`
	Event         string                   `json:"event"`
	ContentState  liveActivityContentState `json:"content-state"`
	StaleDate     *int64                   `json:"stale-date,omitempty"`
	DismissalDate *int64                   `json:"dismissal-date,omitempty"`
}

type liveActivityPayload struct {
	APS liveActivityAPS `json:"aps"`
}

// BuildAlertPayload constructs APNs alert JSON payload for order status updates.
func BuildAlertPayload(orderID string, status OrderStatus) ([]byte, error) {
	message, err := MessageForStatusRU(status)
	if err != nil {
		return nil, err
	}

	payload := alertPayload{
		APS: alertAPS{
			Alert: message,
		},
		OrderID: orderID,
		Status:  status,
	}

	return json.Marshal(payload)
}

// BuildLiveActivityUpdatePayload builds APNs live activity update payload.
func BuildLiveActivityUpdatePayload(orderID string, status OrderStatus, timestamp, staleDate time.Time) ([]byte, error) {
	return buildLiveActivityPayload(orderID, status, LiveActivityEventUpdate, timestamp, &staleDate, nil)
}

// BuildLiveActivityEndPayload builds APNs live activity end payload.
func BuildLiveActivityEndPayload(orderID string, status OrderStatus, timestamp, dismissalDate time.Time) ([]byte, error) {
	return buildLiveActivityPayload(orderID, status, LiveActivityEventEnd, timestamp, nil, &dismissalDate)
}

func buildLiveActivityPayload(orderID string, status OrderStatus, event string, timestamp time.Time, staleDate, dismissalDate *time.Time) ([]byte, error) {
	// Reuse status validation from templates to keep statuses constrained to known values.
	if _, err := MessageForStatusRU(status); err != nil {
		return nil, err
	}

	payload := liveActivityPayload{
		APS: liveActivityAPS{
			Timestamp: timestamp.Unix(),
			Event:     event,
			ContentState: liveActivityContentState{
				OrderID: orderID,
				Status:  status,
			},
		},
	}

	if staleDate != nil {
		value := staleDate.Unix()
		payload.APS.StaleDate = &value
	}

	if dismissalDate != nil {
		value := dismissalDate.Unix()
		payload.APS.DismissalDate = &value
	}

	return json.Marshal(payload)
}
