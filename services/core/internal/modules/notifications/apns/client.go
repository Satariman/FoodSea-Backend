package apns

import (
	"fmt"
	"strings"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"

	platformconfig "github.com/foodsea/core/internal/platform/config"
)

type environment string

const (
	environmentSandbox    environment = "sandbox"
	environmentProduction environment = "production"
)

// PushClient abstracts APNs push transport.
type PushClient interface {
	Push(*apns2.Notification) (*apns2.Response, error)
}

// Client routes APNs requests to sandbox or production client based on environment.
type Client struct {
	sandbox    PushClient
	production PushClient
}

type noopPushClient struct{}

func (noopPushClient) Push(*apns2.Notification) (*apns2.Response, error) {
	return &apns2.Response{StatusCode: 200}, nil
}

// NewClient creates a dual APNs client (sandbox + production) using token auth.
func NewClient(cfg platformconfig.APNSConfig) (*Client, error) {
	privateKey := strings.ReplaceAll(cfg.PrivateKey, "\\n", "\n")
	authKey, err := token.AuthKeyFromBytes([]byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("apns: parse private key: %w", err)
	}

	buildToken := func() *token.Token {
		return &token.Token{
			AuthKey: authKey,
			KeyID:   cfg.KeyID,
			TeamID:  cfg.TeamID,
		}
	}

	return &Client{
		sandbox:    apns2.NewTokenClient(buildToken()).Development(),
		production: apns2.NewTokenClient(buildToken()).Production(),
	}, nil
}

// NewNoopClient returns a client that marks pushes as successful without sending them.
// Useful in non-production environments when APNS credentials are intentionally absent.
func NewNoopClient() *Client {
	noop := noopPushClient{}
	return &Client{
		sandbox:    noop,
		production: noop,
	}
}

// NewClientWithPushers is intended for tests/custom transports.
func NewClientWithPushers(sandbox, production PushClient) *Client {
	return &Client{
		sandbox:    sandbox,
		production: production,
	}
}

// Push sends APNs notification to configured environment and maps response into typed outcome.
func (c *Client) Push(env string, notification *apns2.Notification) (Outcome, error) {
	target, err := c.resolveEnvironment(env)
	if err != nil {
		return Outcome{}, err
	}

	response, err := target.Push(notification)
	if err != nil {
		return Outcome{}, fmt.Errorf("apns: push failed: %w", err)
	}

	return OutcomeFromResponse(response), nil
}

func (c *Client) resolveEnvironment(env string) (PushClient, error) {
	switch environment(strings.ToLower(env)) {
	case environmentSandbox:
		return c.sandbox, nil
	case environmentProduction:
		return c.production, nil
	default:
		return nil, fmt.Errorf("apns: unsupported environment %q", env)
	}
}

// NewAlertNotification creates APNs alert notification with mandatory headers.
func NewAlertNotification(deviceToken, bundleID string, payload []byte) *apns2.Notification {
	return &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       bundleID,
		Priority:    apns2.PriorityHigh,
		PushType:    apns2.PushTypeAlert,
		Payload:     payload,
	}
}

// NewLiveActivityNotification creates APNs liveactivity notification with mandatory headers.
func NewLiveActivityNotification(deviceToken, bundleID string, payload []byte) *apns2.Notification {
	return &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       bundleID + ".push-type.liveactivity",
		Priority:    apns2.PriorityHigh,
		PushType:    apns2.PushTypeLiveActivity,
		Payload:     payload,
	}
}
