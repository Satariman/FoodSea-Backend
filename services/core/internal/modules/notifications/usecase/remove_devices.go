package usecase

import (
	"context"

	"github.com/google/uuid"
)

// RemoveDevices deletes all APNs device tokens of the user.
type RemoveDevices struct {
	repo Repository
}

func NewRemoveDevices(repo Repository) *RemoveDevices {
	return &RemoveDevices{repo: repo}
}

func (uc *RemoveDevices) Execute(ctx context.Context, userID uuid.UUID) error {
	return uc.repo.DeleteDevices(ctx, userID)
}
