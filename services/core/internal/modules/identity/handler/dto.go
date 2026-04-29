package handler

import (
	"time"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/identity/domain"
)

type RegisterRequest struct {
	Phone    *string `json:"phone"    binding:"omitempty,e164"`
	Email    *string `json:"email"    binding:"omitempty,email"`
	Password string  `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Phone    *string `json:"phone"    binding:"omitempty,e164"`
	Email    *string `json:"email"    binding:"omitempty,email"`
	Password string  `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type UserResponse struct {
	ID             uuid.UUID `json:"id"`
	Phone          *string   `json:"phone,omitempty"`
	Email          *string   `json:"email,omitempty"`
	OnboardingDone bool      `json:"onboarding_done"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AuthResponse struct {
	User             UserResponse `json:"user"`
	Access           string       `json:"access_token"`
	Refresh          string       `json:"refresh_token"`
	AccessExpiresAt  time.Time    `json:"access_expires_at"`
	RefreshExpiresAt time.Time    `json:"refresh_expires_at"`
}

type TokenPairResponse struct {
	Access           string    `json:"access_token"`
	Refresh          string    `json:"refresh_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

func toUserResponse(u *domain.User) UserResponse {
	return UserResponse{
		ID:             u.ID,
		Phone:          u.Phone,
		Email:          u.Email,
		OnboardingDone: u.OnboardingDone,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
	}
}

func toAuthResponse(u *domain.User, pair domain.TokenPair) AuthResponse {
	return AuthResponse{
		User:             toUserResponse(u),
		Access:           pair.Access,
		Refresh:          pair.Refresh,
		AccessExpiresAt:  pair.AccessExpiresAt,
		RefreshExpiresAt: pair.RefreshExpiresAt,
	}
}

func toTokenPairResponse(pair domain.TokenPair) TokenPairResponse {
	return TokenPairResponse{
		Access:           pair.Access,
		Refresh:          pair.Refresh,
		AccessExpiresAt:  pair.AccessExpiresAt,
		RefreshExpiresAt: pair.RefreshExpiresAt,
	}
}
