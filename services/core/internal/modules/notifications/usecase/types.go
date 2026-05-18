package usecase

import (
	"context"

	"github.com/google/uuid"
)

// DeviceRegistration is an input for registering/updating an APNs device token.
type DeviceRegistration struct {
	UserID      uuid.UUID
	APNSToken   string
	BundleID    string
	Environment string
	AppVersion  *string
}

// LiveActivityRegistration is an input for registering/updating a Live Activity push token.
type LiveActivityRegistration struct {
	UserID      uuid.UUID
	OrderID     uuid.UUID
	PushToken   string
	BundleID    string
	Environment string
}

// Repository defines storage operations required by notifications use-cases.
type Repository interface {
	UpsertDevice(ctx context.Context, in DeviceRegistration) error
	DeleteDevices(ctx context.Context, userID uuid.UUID) error
	UpsertLiveActivity(ctx context.Context, in LiveActivityRegistration) error
	DeleteLiveActivity(ctx context.Context, userID, orderID uuid.UUID) error
}
