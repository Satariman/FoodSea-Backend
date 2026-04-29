package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type TokenService interface {
	IssuePair(ctx context.Context, userID uuid.UUID) (TokenPair, error)
	ValidateAccess(token string) (Claims, error)
	RotateRefresh(ctx context.Context, refresh string) (TokenPair, error)
	Revoke(ctx context.Context, userID uuid.UUID) error
}

type TokenPair struct {
	Access           string
	Refresh          string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type Claims struct {
	UserID uuid.UUID
}
