package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/platform/config"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type GoogleOAuthProvider struct {
	cfg    config.OAuthProviderConfig
	client *http.Client
}

func NewGoogleOAuthProvider(cfg config.OAuthProviderConfig, client *http.Client) *GoogleOAuthProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &GoogleOAuthProvider{cfg: cfg, client: client}
}

func (p *GoogleOAuthProvider) Name() domain.OAuthProviderKind {
	return domain.OAuthProviderGoogle
}

func (p *GoogleOAuthProvider) AuthURL(_ context.Context, state string, session domain.OAuthSession) (string, error) {
	nonce := state
	scope := "openid email profile"
	if len(p.cfg.Scopes) > 0 {
		scope = strings.Join(p.cfg.Scopes, " ")
	}

	q := url.Values{}
	q.Set("client_id", p.cfg.ClientID)
	q.Set("redirect_uri", session.RedirectTo)
	q.Set("response_type", "code")
	q.Set("scope", scope)
	q.Set("state", state)
	q.Set("nonce", nonce)

	return p.cfg.AuthURL + "?" + q.Encode(), nil
}

func (p *GoogleOAuthProvider) Exchange(ctx context.Context, code string, session domain.OAuthSession) (domain.OAuthProviderProfile, error) {
	type tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}

	form := url.Values{}
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", session.RedirectTo)

	var token tokenResp
	if err := postFormAndDecodeJSON(ctx, p.client, p.cfg.TokenURL, form, &token); err != nil {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: token exchange failed", sherrors.ErrUnauthorized)
	}

	claims, err := parseGoogleIDTokenClaims(token.IDToken)
	if err != nil {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: invalid id_token", sherrors.ErrUnauthorized)
	}
	if err := validateGoogleClaims(claims, p.cfg.ClientID, session.State); err != nil {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: id_token claims validation failed", sherrors.ErrUnauthorized)
	}

	return domain.OAuthProviderProfile{
		Provider:       domain.OAuthProviderGoogle,
		ProviderUserID: claims.Sub,
		Email:          claims.Email,
		EmailVerified:  claims.EmailVerified,
	}, nil
}

type googleIDTokenClaims struct {
	Iss           string  `json:"iss"`
	Aud           string  `json:"aud"`
	Exp           int64   `json:"exp"`
	Nonce         string  `json:"nonce"`
	Sub           string  `json:"sub"`
	Email         *string `json:"email"`
	EmailVerified bool    `json:"email_verified"`
}

func parseGoogleIDTokenClaims(token string) (googleIDTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return googleIDTokenClaims{}, errors.New("malformed jwt")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return googleIDTokenClaims{}, fmt.Errorf("decode payload: %w", err)
	}

	var claims googleIDTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return googleIDTokenClaims{}, fmt.Errorf("unmarshal claims: %w", err)
	}

	return claims, nil
}

func validateGoogleClaims(claims googleIDTokenClaims, clientID, expectedNonce string) error {
	if claims.Iss != "https://accounts.google.com" && claims.Iss != "accounts.google.com" {
		return errors.New("invalid issuer")
	}
	if claims.Aud != clientID || clientID == "" {
		return errors.New("invalid audience")
	}
	if claims.Exp <= time.Now().Unix() {
		return errors.New("token expired")
	}
	if expectedNonce == "" || claims.Nonce != expectedNonce {
		return errors.New("invalid nonce")
	}
	if strings.TrimSpace(claims.Sub) == "" {
		return errors.New("missing sub")
	}
	return nil
}

func buildUnsignedJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	b, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(b)
	return header + "." + payload + "."
}
