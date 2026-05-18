package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthProviderKind string
type OAuthFlowMode string

const (
	OAuthProviderGoogle OAuthProviderKind = "google"
	OAuthProviderApple  OAuthProviderKind = "apple"
	OAuthProviderVK     OAuthProviderKind = "vk"
	OAuthProviderYandex OAuthProviderKind = "yandex"

	OAuthFlowModeLegacy OAuthFlowMode = "legacy"
	OAuthFlowModeNative OAuthFlowMode = "native"
)

func ParseOAuthProvider(raw string) (OAuthProviderKind, error) {
	switch OAuthProviderKind(strings.ToLower(strings.TrimSpace(raw))) {
	case OAuthProviderGoogle:
		return OAuthProviderGoogle, nil
	case OAuthProviderApple:
		return OAuthProviderApple, nil
	case OAuthProviderVK:
		return OAuthProviderVK, nil
	case OAuthProviderYandex:
		return OAuthProviderYandex, nil
	default:
		return "", fmt.Errorf("%w: unsupported oauth provider %q", sherrors.ErrInvalidInput, raw)
	}
}

func ParseOAuthProviderName(raw string) (OAuthProviderKind, error) {
	return ParseOAuthProvider(raw)
}

type OAuthSession struct {
	State        string
	Provider     OAuthProviderKind
	Mode         OAuthFlowMode
	RedirectTo   string
	Nonce        string
	PKCEVerifier string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

type OAuthStartRequest struct {
	Provider   OAuthProviderKind
	RedirectTo string
	Mode       OAuthFlowMode
}

type OAuthStartResult struct {
	AuthURL string
	State   string
}

type OAuthCallbackRequest struct {
	Provider    OAuthProviderKind
	State       string
	Code        string
	RedirectURI string
	Mode        OAuthFlowMode
}

type OAuthCallbackResult struct {
	User      *User
	TokenPair TokenPair
}

type OAuthTokenCallbackRequest struct {
	Provider    OAuthProviderKind
	AccessToken string
	FullName    *string
	Email       *string
}

type OAuthTokenCallbackResult struct {
	User      *User
	TokenPair TokenPair
}

type OAuthProviderProfile struct {
	Provider       OAuthProviderKind
	ProviderUserID string
	Email          *string
	EmailVerified  bool
	Name           *string
	AvatarURL      *string
}

type OAuthIdentity struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       OAuthProviderKind
	ProviderUserID string
	Email          *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OAuthStateStore interface {
	Save(ctx context.Context, session OAuthSession) error
	GetAndDelete(ctx context.Context, state string) (OAuthSession, error)
}

type OAuthProvider interface {
	Name() OAuthProviderKind
	AuthURL(ctx context.Context, state string, session OAuthSession) (string, error)
	Exchange(ctx context.Context, code string, session OAuthSession) (OAuthProviderProfile, error)
	ProfileFromToken(ctx context.Context, accessToken string) (OAuthProviderProfile, error)
}

type OAuthIdentityRepository interface {
	GetByProviderUserID(ctx context.Context, provider OAuthProviderKind, providerUserID string) (*OAuthIdentity, error)
	Create(ctx context.Context, identity *OAuthIdentity) error
}
