package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthStateStore interface {
	Create(ctx context.Context, session domain.OAuthSession) (string, error)
	Consume(ctx context.Context, state string) (domain.OAuthSession, error)
}

type OAuthStart struct {
	states                   OAuthStateStore
	providers                map[domain.OAuthProviderKind]domain.OAuthProvider
	allowedLegacyRedirectURI map[string]struct{}
	allowedNativeRedirectURI map[string]struct{}
	stateTTL                 time.Duration
}

func NewOAuthStart(
	states OAuthStateStore,
	providers []domain.OAuthProvider,
	allowedLegacyRedirectURIs []string,
	allowedNativeRedirectURIs []string,
	stateTTL time.Duration,
) *OAuthStart {
	providersMap := make(map[domain.OAuthProviderKind]domain.OAuthProvider, len(providers))
	for _, provider := range providers {
		providersMap[provider.Name()] = provider
	}

	allowedLegacy := make(map[string]struct{}, len(allowedLegacyRedirectURIs))
	for _, uri := range allowedLegacyRedirectURIs {
		allowedLegacy[uri] = struct{}{}
	}
	allowedNative := make(map[string]struct{}, len(allowedNativeRedirectURIs))
	for _, uri := range allowedNativeRedirectURIs {
		allowedNative[uri] = struct{}{}
	}

	return &OAuthStart{
		states:                   states,
		providers:                providersMap,
		allowedLegacyRedirectURI: allowedLegacy,
		allowedNativeRedirectURI: allowedNative,
		stateTTL:                 stateTTL,
	}
}

func (o *OAuthStart) Execute(ctx context.Context, req domain.OAuthStartRequest) (domain.OAuthStartResult, error) {
	mode := req.Mode
	if mode == "" {
		mode = domain.OAuthFlowModeLegacy
	}

	provider, ok := o.providers[req.Provider]
	if !ok {
		return domain.OAuthStartResult{}, fmt.Errorf("%w: unsupported oauth provider", sherrors.ErrInvalidInput)
	}

	allowed := o.allowedLegacyRedirectURI
	if mode == domain.OAuthFlowModeNative {
		allowed = o.allowedNativeRedirectURI
	}
	if _, ok := allowed[req.RedirectTo]; !ok {
		return domain.OAuthStartResult{}, fmt.Errorf("%w: redirect uri is not allowed", sherrors.ErrInvalidInput)
	}

	now := time.Now()
	session := domain.OAuthSession{
		Provider:   req.Provider,
		Mode:       mode,
		RedirectTo: req.RedirectTo,
		CreatedAt:  now,
		ExpiresAt:  now.Add(o.stateTTL),
	}
	if mode == domain.OAuthFlowModeNative {
		nonce, err := randomURLToken(32)
		if err != nil {
			return domain.OAuthStartResult{}, fmt.Errorf("generate oauth nonce: %w", err)
		}
		verifier, err := randomURLToken(48)
		if err != nil {
			return domain.OAuthStartResult{}, fmt.Errorf("generate oauth pkce verifier: %w", err)
		}
		session.Nonce = nonce
		session.PKCEVerifier = verifier
	}

	state, err := o.states.Create(ctx, session)
	if err != nil {
		return domain.OAuthStartResult{}, err
	}

	authURL, err := provider.AuthURL(ctx, state, session)
	if err != nil {
		return domain.OAuthStartResult{}, err
	}

	return domain.OAuthStartResult{
		AuthURL: authURL,
		State:   state,
	}, nil
}

func randomURLToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
