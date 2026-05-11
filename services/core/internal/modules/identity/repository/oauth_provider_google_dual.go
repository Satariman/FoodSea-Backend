package repository

import (
	"context"
	"fmt"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type GoogleDualOAuthProvider struct {
	legacy *GoogleOAuthProvider
	native *GoogleOAuthProvider
}

func NewGoogleDualOAuthProvider(legacy, native *GoogleOAuthProvider) *GoogleDualOAuthProvider {
	return &GoogleDualOAuthProvider{
		legacy: legacy,
		native: native,
	}
}

func (p *GoogleDualOAuthProvider) Name() domain.OAuthProviderKind {
	return domain.OAuthProviderGoogle
}

func (p *GoogleDualOAuthProvider) AuthURL(ctx context.Context, state string, session domain.OAuthSession) (string, error) {
	provider, err := p.pickProvider(session.Mode)
	if err != nil {
		return "", err
	}
	return provider.AuthURL(ctx, state, session)
}

func (p *GoogleDualOAuthProvider) Exchange(ctx context.Context, code string, session domain.OAuthSession) (domain.OAuthProviderProfile, error) {
	provider, err := p.pickProvider(session.Mode)
	if err != nil {
		return domain.OAuthProviderProfile{}, err
	}
	return provider.Exchange(ctx, code, session)
}

func (p *GoogleDualOAuthProvider) ProfileFromToken(ctx context.Context, accessToken string) (domain.OAuthProviderProfile, error) {
	if p.native != nil {
		return p.native.ProfileFromToken(ctx, accessToken)
	}
	if p.legacy != nil {
		return p.legacy.ProfileFromToken(ctx, accessToken)
	}
	return domain.OAuthProviderProfile{}, fmt.Errorf("%w: google oauth provider is disabled", sherrors.ErrInvalidInput)
}

func (p *GoogleDualOAuthProvider) pickProvider(mode domain.OAuthFlowMode) (*GoogleOAuthProvider, error) {
	if mode == domain.OAuthFlowModeNative {
		if p.native == nil {
			return nil, fmt.Errorf("%w: native google oauth provider is disabled", sherrors.ErrInvalidInput)
		}
		return p.native, nil
	}
	if p.legacy == nil {
		return nil, fmt.Errorf("%w: legacy google oauth provider is disabled", sherrors.ErrInvalidInput)
	}
	return p.legacy, nil
}
