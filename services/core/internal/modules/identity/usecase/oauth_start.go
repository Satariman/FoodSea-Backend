package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthStateStore interface {
	Create(ctx context.Context, session domain.OAuthSession) (string, error)
	Consume(ctx context.Context, state string) (domain.OAuthSession, error)
}

type OAuthProvider interface {
	Name() domain.OAuthProviderKind
	AuthURL(state string) (string, error)
	Exchange(ctx context.Context, code string) (domain.OAuthProviderProfile, error)
}

type OAuthStart struct {
	states             OAuthStateStore
	providers          map[domain.OAuthProviderKind]OAuthProvider
	allowedRedirectURI map[string]struct{}
	stateTTL           time.Duration
}

func NewOAuthStart(
	states OAuthStateStore,
	providers []OAuthProvider,
	allowedRedirectURIs []string,
	stateTTL time.Duration,
) *OAuthStart {
	providersMap := make(map[domain.OAuthProviderKind]OAuthProvider, len(providers))
	for _, provider := range providers {
		providersMap[provider.Name()] = provider
	}

	allowed := make(map[string]struct{}, len(allowedRedirectURIs))
	for _, uri := range allowedRedirectURIs {
		allowed[uri] = struct{}{}
	}

	return &OAuthStart{
		states:             states,
		providers:          providersMap,
		allowedRedirectURI: allowed,
		stateTTL:           stateTTL,
	}
}

func (o *OAuthStart) Execute(ctx context.Context, req domain.OAuthStartRequest) (domain.OAuthStartResult, error) {
	provider, ok := o.providers[req.Provider]
	if !ok {
		return domain.OAuthStartResult{}, fmt.Errorf("%w: unsupported oauth provider", sherrors.ErrInvalidInput)
	}

	if _, ok := o.allowedRedirectURI[req.RedirectTo]; !ok {
		return domain.OAuthStartResult{}, fmt.Errorf("%w: redirect uri is not allowed", sherrors.ErrInvalidInput)
	}

	now := time.Now()
	session := domain.OAuthSession{
		Provider:   req.Provider,
		RedirectTo: req.RedirectTo,
		CreatedAt:  now,
		ExpiresAt:  now.Add(o.stateTTL),
	}

	state, err := o.states.Create(ctx, session)
	if err != nil {
		return domain.OAuthStartResult{}, err
	}

	authURL, err := provider.AuthURL(state)
	if err != nil {
		return domain.OAuthStartResult{}, err
	}

	return domain.OAuthStartResult{
		AuthURL: authURL,
		State:   state,
	}, nil
}
