package usecase

import (
	"context"

	"github.com/google/uuid"
)

// RemoveLiveActivity deletes live activity push-token binding for the order.
type RemoveLiveActivity struct {
	repo Repository
}

func NewRemoveLiveActivity(repo Repository) *RemoveLiveActivity {
	return &RemoveLiveActivity{repo: repo}
}

func (uc *RemoveLiveActivity) Execute(ctx context.Context, userID, orderID uuid.UUID) error {
	return uc.repo.DeleteLiveActivity(ctx, userID, orderID)
}
