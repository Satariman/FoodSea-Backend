package usecase

import "context"

// RegisterLiveActivity stores or updates a push token for order live activity.
type RegisterLiveActivity struct {
	repo Repository
}

func NewRegisterLiveActivity(repo Repository) *RegisterLiveActivity {
	return &RegisterLiveActivity{repo: repo}
}

func (uc *RegisterLiveActivity) Execute(ctx context.Context, in LiveActivityRegistration) error {
	return uc.repo.UpsertLiveActivity(ctx, in)
}
