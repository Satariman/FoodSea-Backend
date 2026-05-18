package internal

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAlertPayload(t *testing.T) {
	raw, err := BuildAlertPayload("ord-123", StatusCreated)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))

	assert.Equal(t, "ord-123", payload["order_id"])
	assert.Equal(t, "created", payload["status"])

	aps, ok := payload["aps"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Мы приняли ваш заказ", aps["alert"])
}

func TestBuildAlertPayload_UnknownStatus(t *testing.T) {
	_, err := BuildAlertPayload("ord-123", OrderStatus("other"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown order status")
}

func TestBuildLiveActivityUpdatePayload(t *testing.T) {
	timestamp := time.Unix(1_700_000_000, 0)
	staleDate := time.Unix(1_700_000_600, 0)

	raw, err := BuildLiveActivityUpdatePayload("ord-123", StatusConfirmed, timestamp, staleDate)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))

	aps, ok := payload["aps"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "update", aps["event"])
	assert.EqualValues(t, timestamp.Unix(), aps["timestamp"])
	assert.EqualValues(t, staleDate.Unix(), aps["stale-date"])
	_, hasDismissalDate := aps["dismissal-date"]
	assert.False(t, hasDismissalDate)

	contentState, ok := aps["content-state"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ord-123", contentState["order_id"])
	assert.Equal(t, "confirmed", contentState["status"])
}

func TestBuildLiveActivityEndPayload(t *testing.T) {
	timestamp := time.Unix(1_700_001_000, 0)
	dismissalDate := time.Unix(1_700_001_300, 0)

	raw, err := BuildLiveActivityEndPayload("ord-456", StatusDelivered, timestamp, dismissalDate)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))

	aps, ok := payload["aps"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "end", aps["event"])
	assert.EqualValues(t, timestamp.Unix(), aps["timestamp"])
	assert.EqualValues(t, dismissalDate.Unix(), aps["dismissal-date"])
	_, hasStaleDate := aps["stale-date"]
	assert.False(t, hasStaleDate)

	contentState, ok := aps["content-state"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ord-456", contentState["order_id"])
	assert.Equal(t, "delivered", contentState["status"])
}
