package repository

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/platform/config"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type YandexOAuthProvider struct {
	cfg    config.OAuthProviderConfig
	client *http.Client
}

func NewYandexOAuthProvider(cfg config.OAuthProviderConfig, client *http.Client) *YandexOAuthProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &YandexOAuthProvider{cfg: cfg, client: client}
}

func (p *YandexOAuthProvider) Name() domain.OAuthProviderKind {
	return domain.OAuthProviderYandex
}

func (p *YandexOAuthProvider) AuthURL(_ context.Context, state string, session domain.OAuthSession) (string, error) {
	scope := "login:email login:avatar"
	if len(p.cfg.Scopes) > 0 {
		scope = strings.Join(p.cfg.Scopes, " ")
	}

	q := url.Values{}
	q.Set("client_id", p.cfg.ClientID)
	q.Set("redirect_uri", session.RedirectTo)
	q.Set("response_type", "code")
	q.Set("scope", scope)
	q.Set("state", state)

	return p.cfg.AuthURL + "?" + q.Encode(), nil
}

func (p *YandexOAuthProvider) Exchange(ctx context.Context, code string, session domain.OAuthSession) (domain.OAuthProviderProfile, error) {
	type tokenResp struct {
		AccessToken string `json:"access_token"`
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)
	form.Set("redirect_uri", session.RedirectTo)

	var token tokenResp
	if err := postFormAndDecodeJSON(ctx, p.client, p.cfg.TokenURL, form, &token); err != nil {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: token exchange failed", sherrors.ErrUnauthorized)
	}

	userInfo, err := p.fetchUserInfo(ctx, token.AccessToken)
	if err != nil {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: userinfo request failed", sherrors.ErrUnauthorized)
	}
	return profileFromYandexUserInfo(userInfo), nil
}

func (p *YandexOAuthProvider) ProfileFromToken(ctx context.Context, accessToken string) (domain.OAuthProviderProfile, error) {
	if strings.TrimSpace(accessToken) == "" {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: yandex access token is empty", sherrors.ErrInvalidInput)
	}
	userInfo, err := p.fetchUserInfo(ctx, accessToken)
	if err != nil {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: userinfo request failed", sherrors.ErrUnauthorized)
	}
	return profileFromYandexUserInfo(userInfo), nil
}

type yandexUserInfoResp struct {
	ID    string  `json:"id"`
	Email *string `json:"default_email"`
}

func (p *YandexOAuthProvider) fetchUserInfo(ctx context.Context, accessToken string) (yandexUserInfoResp, error) {
	var userInfo yandexUserInfoResp
	if err := getJSONWithHeaders(ctx, p.client, p.cfg.UserInfoURL, map[string]string{
		"Authorization": "OAuth " + accessToken,
	}, &userInfo); err != nil {
		return yandexUserInfoResp{}, err
	}
	if strings.TrimSpace(userInfo.ID) == "" {
		return yandexUserInfoResp{}, fmt.Errorf("%w: userinfo missing id", sherrors.ErrUnauthorized)
	}
	return userInfo, nil
}

func profileFromYandexUserInfo(userInfo yandexUserInfoResp) domain.OAuthProviderProfile {
	emailVerified := userInfo.Email != nil && strings.TrimSpace(*userInfo.Email) != ""
	if !emailVerified {
		userInfo.Email = nil
	}

	return domain.OAuthProviderProfile{
		Provider:       domain.OAuthProviderYandex,
		ProviderUserID: userInfo.ID,
		Email:          userInfo.Email,
		EmailVerified:  emailVerified,
	}
}
