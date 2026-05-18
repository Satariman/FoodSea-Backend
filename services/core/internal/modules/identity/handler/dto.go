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

type OAuthStartResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

type OAuthCallbackRequest struct {
	Code        string `json:"code" binding:"required"`
	State       string `json:"state" binding:"required"`
	RedirectURI string `json:"redirect_uri" binding:"required"`
}

type OAuthNativeSDKCallbackRequest struct {
	AccessToken string `json:"access_token" binding:"required"`
}

type OAuthNativeAppleCallbackRequest struct {
	IdentityToken string  `json:"identity_token" binding:"required"`
	FullName      *string `json:"full_name,omitempty"`
	Email         *string `json:"email,omitempty" binding:"omitempty,email"`
}

type OAuthNativeCallbackRequest struct {
	Code          string  `json:"code,omitempty"`
	State         string  `json:"state,omitempty"`
	RedirectURI   string  `json:"redirect_uri,omitempty"`
	IdentityToken string  `json:"identity_token,omitempty"`
	FullName      *string `json:"full_name,omitempty"`
	Email         *string `json:"email,omitempty"`
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
