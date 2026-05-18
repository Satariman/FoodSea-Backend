package usecase

import "context"

// RegisterDevice stores or updates the user's APNs device token.
type RegisterDevice struct {
	repo Repository
}

func NewRegisterDevice(repo Repository) *RegisterDevice {
	return &RegisterDevice{repo: repo}
}

func (uc *RegisterDevice) Execute(ctx context.Context, in DeviceRegistration) error {
	return uc.repo.UpsertDevice(ctx, in)
}
