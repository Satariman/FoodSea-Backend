package apns

import (
	"testing"

	"github.com/sideshow/apns2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pushClientMock struct {
	calls int
}

func (m *pushClientMock) Push(_ *apns2.Notification) (*apns2.Response, error) {
	m.calls++
	return &apns2.Response{StatusCode: apns2.StatusSent}, nil
}

func TestClientPush_RoutesByEnvironment(t *testing.T) {
	sandbox := &pushClientMock{}
	production := &pushClientMock{}
	client := NewClientWithPushers(sandbox, production)

	_, err := client.Push("sandbox", &apns2.Notification{})
	require.NoError(t, err)
	assert.Equal(t, 1, sandbox.calls)
	assert.Equal(t, 0, production.calls)

	_, err = client.Push("production", &apns2.Notification{})
	require.NoError(t, err)
	assert.Equal(t, 1, sandbox.calls)
	assert.Equal(t, 1, production.calls)
}

func TestClientPush_UnknownEnvironment(t *testing.T) {
	client := NewClientWithPushers(&pushClientMock{}, &pushClientMock{})

	_, err := client.Push("staging", &apns2.Notification{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported environment")
}
