package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthCallback struct {
	states     OAuthStateStore
	providers  map[domain.OAuthProviderKind]domain.OAuthProvider
	identities domain.OAuthIdentityRepository
	users      domain.UserRepository
	tokens     domain.TokenService
}

type oauthResolveOptions struct {
	disallowEmailLink bool
	appleFullName     *string
}

func NewOAuthCallback(
	states OAuthStateStore,
	providers []domain.OAuthProvider,
	identities domain.OAuthIdentityRepository,
	users domain.UserRepository,
	tokens domain.TokenService,
) *OAuthCallback {
	providersMap := make(map[domain.OAuthProviderKind]domain.OAuthProvider, len(providers))
	for _, provider := range providers {
		providersMap[provider.Name()] = provider
	}

	return &OAuthCallback{
		states:     states,
		providers:  providersMap,
		identities: identities,
		users:      users,
		tokens:     tokens,
	}
}

func (o *OAuthCallback) Execute(ctx context.Context, req domain.OAuthCallbackRequest) (domain.OAuthCallbackResult, error) {
	if req.Provider == "" || strings.TrimSpace(req.State) == "" || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.RedirectURI) == "" {
		return domain.OAuthCallbackResult{}, fmt.Errorf("%w: malformed oauth callback input", sherrors.ErrInvalidInput)
	}
	mode := req.Mode
	if mode == "" {
		mode = domain.OAuthFlowModeLegacy
	}

	provider, ok := o.providers[req.Provider]
	if !ok {
		return domain.OAuthCallbackResult{}, fmt.Errorf("%w: unsupported oauth provider", sherrors.ErrInvalidInput)
	}

	session, err := o.states.Consume(ctx, req.State)
	if err != nil {
		if errors.Is(err, sherrors.ErrUnauthorized) {
			return domain.OAuthCallbackResult{}, sherrors.ErrUnauthorized
		}
		return domain.OAuthCallbackResult{}, err
	}
	if session.Provider != req.Provider || session.RedirectTo == "" {
		return domain.OAuthCallbackResult{}, sherrors.ErrUnauthorized
	}
	sessionMode := session.Mode
	if sessionMode == "" {
		sessionMode = domain.OAuthFlowModeLegacy
	}
	if sessionMode != mode {
		return domain.OAuthCallbackResult{}, sherrors.ErrUnauthorized
	}
	if session.RedirectTo != req.RedirectURI {
		return domain.OAuthCallbackResult{}, sherrors.ErrUnauthorized
	}

	profile, err := provider.Exchange(ctx, req.Code, session)
	if err != nil {
		if errors.Is(err, sherrors.ErrUnauthorized) {
			return domain.OAuthCallbackResult{}, sherrors.ErrUnauthorized
		}
		return domain.OAuthCallbackResult{}, err
	}

	u, err := o.resolveUser(ctx, req.Provider, profile, oauthResolveOptions{})
	if err != nil {
		return domain.OAuthCallbackResult{}, err
	}

	pair, err := o.tokens.IssuePair(ctx, u.ID)
	if err != nil {
		return domain.OAuthCallbackResult{}, err
	}

	return domain.OAuthCallbackResult{
		User:      u,
		TokenPair: pair,
	}, nil
}

func (o *OAuthCallback) ExecuteToken(ctx context.Context, req domain.OAuthTokenCallbackRequest) (domain.OAuthTokenCallbackResult, error) {
	if req.Provider == "" || strings.TrimSpace(req.AccessToken) == "" {
		return domain.OAuthTokenCallbackResult{}, fmt.Errorf("%w: malformed oauth token callback input", sherrors.ErrInvalidInput)
	}

	provider, ok := o.providers[req.Provider]
	if !ok {
		return domain.OAuthTokenCallbackResult{}, fmt.Errorf("%w: unsupported oauth provider", sherrors.ErrInvalidInput)
	}

	profile, err := provider.ProfileFromToken(ctx, req.AccessToken)
	if err != nil {
		if errors.Is(err, sherrors.ErrUnauthorized) {
			return domain.OAuthTokenCallbackResult{}, sherrors.ErrUnauthorized
		}
		if errors.Is(err, sherrors.ErrInvalidInput) {
			return domain.OAuthTokenCallbackResult{}, sherrors.ErrInvalidInput
		}
		return domain.OAuthTokenCallbackResult{}, err
	}

	resolveOpts := oauthResolveOptions{}
	if req.Provider == domain.OAuthProviderApple {
		if email := normalizeOptionalEmail(req.Email); email != nil {
			profile.Email = email
			profile.EmailVerified = true
		}
		resolveOpts.disallowEmailLink = true
		resolveOpts.appleFullName = normalizeOptionalFullName(req.FullName)
	}

	u, err := o.resolveUser(ctx, req.Provider, profile, resolveOpts)
	if err != nil {
		return domain.OAuthTokenCallbackResult{}, err
	}

	pair, err := o.tokens.IssuePair(ctx, u.ID)
	if err != nil {
		return domain.OAuthTokenCallbackResult{}, err
	}

	return domain.OAuthTokenCallbackResult{
		User:      u,
		TokenPair: pair,
	}, nil
}

func (o *OAuthCallback) resolveUser(
	ctx context.Context,
	provider domain.OAuthProviderKind,
	profile domain.OAuthProviderProfile,
	opts oauthResolveOptions,
) (*domain.User, error) {
	identity, err := o.identities.GetByProviderUserID(ctx, provider, profile.ProviderUserID)
	if err == nil {
		return o.users.GetByID(ctx, identity.UserID)
	}
	if !errors.Is(err, sherrors.ErrNotFound) {
		return nil, err
	}

	if opts.disallowEmailLink {
		if profile.Email != nil {
			if _, getErr := o.users.GetByEmail(ctx, *profile.Email); getErr == nil {
				return nil, sherrors.ErrConflict
			} else if !errors.Is(getErr, sherrors.ErrNotFound) {
				return nil, getErr
			}
		}

		newUser := &domain.User{
			ID:       uuid.New(),
			Email:    profile.Email,
			FullName: opts.appleFullName,
		}
		if err := o.users.CreateOAuth(ctx, newUser); err != nil {
			return nil, err
		}
		return o.linkIdentityToUser(ctx, newUser, provider, profile)
	}

	if profile.Email == nil || !profile.EmailVerified {
		return nil, sherrors.ErrConflict
	}

	existingUser, err := o.users.GetByEmail(ctx, *profile.Email)
	switch {
	case err == nil:
		return o.linkIdentityToUser(ctx, existingUser, provider, profile)
	case errors.Is(err, sherrors.ErrNotFound):
		newUser := &domain.User{
			ID:    uuid.New(),
			Email: profile.Email,
		}
		if err := o.users.CreateOAuth(ctx, newUser); err != nil {
			return nil, err
		}
		return o.linkIdentityToUser(ctx, newUser, provider, profile)
	default:
		return nil, err
	}
}

func normalizeOptionalEmail(email *string) *string {
	if email == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*email)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeOptionalFullName(fullName *string) *string {
	if fullName == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*fullName)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (o *OAuthCallback) linkIdentityToUser(
	ctx context.Context,
	u *domain.User,
	provider domain.OAuthProviderKind,
	profile domain.OAuthProviderProfile,
) (*domain.User, error) {
	identity := &domain.OAuthIdentity{
		ID:             uuid.New(),
		UserID:         u.ID,
		Provider:       provider,
		ProviderUserID: profile.ProviderUserID,
		Email:          profile.Email,
	}

	if err := o.identities.Create(ctx, identity); err != nil {
		if errors.Is(err, sherrors.ErrAlreadyExists) {
			existing, getErr := o.identities.GetByProviderUserID(ctx, provider, profile.ProviderUserID)
			if getErr != nil {
				return nil, getErr
			}
			return o.users.GetByID(ctx, existing.UserID)
		}
		return nil, err
	}

	return u, nil
}
