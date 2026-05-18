package repository

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/platform/config"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

const (
	appleDefaultJWKSURL      = "https://appleid.apple.com/auth/keys"
	appleDefaultIssuer       = "https://appleid.apple.com"
	appleDefaultJWKSCacheTTL = time.Hour
	appleMaxTokenAge         = 10 * time.Minute
	appleClockSkew           = time.Minute
)

type AppleOAuthProvider struct {
	cfg    config.OAuthAppleConfig
	client *http.Client
	now    func() time.Time

	mu           sync.RWMutex
	cachedKeys   map[string]*ecdsa.PublicKey
	cacheExpires time.Time
}

func NewAppleOAuthProvider(cfg config.OAuthAppleConfig, client *http.Client) *AppleOAuthProvider {
	if client == nil {
		client = http.DefaultClient
	}
	if cfg.JWKSURL == "" {
		cfg.JWKSURL = appleDefaultJWKSURL
	}
	if cfg.Issuer == "" {
		cfg.Issuer = appleDefaultIssuer
	}
	if cfg.JWKSCacheTTL <= 0 {
		cfg.JWKSCacheTTL = appleDefaultJWKSCacheTTL
	}

	return &AppleOAuthProvider{
		cfg:        cfg,
		client:     client,
		now:        time.Now,
		cachedKeys: map[string]*ecdsa.PublicKey{},
	}
}

func (p *AppleOAuthProvider) Name() domain.OAuthProviderKind {
	return domain.OAuthProviderApple
}

func (p *AppleOAuthProvider) AuthURL(_ context.Context, _ string, _ domain.OAuthSession) (string, error) {
	return "", fmt.Errorf("%w: apple auth url flow is not supported", sherrors.ErrInvalidInput)
}

func (p *AppleOAuthProvider) Exchange(_ context.Context, _ string, _ domain.OAuthSession) (domain.OAuthProviderProfile, error) {
	return domain.OAuthProviderProfile{}, fmt.Errorf("%w: apple code exchange is not supported", sherrors.ErrInvalidInput)
}

func (p *AppleOAuthProvider) ProfileFromToken(ctx context.Context, accessToken string) (domain.OAuthProviderProfile, error) {
	tokenString := strings.TrimSpace(accessToken)
	if tokenString == "" {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: apple identity token is empty", sherrors.ErrInvalidInput)
	}

	kid, err := parseJWTKid(tokenString)
	if err != nil {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: malformed apple identity token", sherrors.ErrUnauthorized)
	}

	claims := appleIdentityTokenClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithAudience(p.cfg.ClientID),
		jwt.WithIssuer(p.cfg.Issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(appleClockSkew),
		jwt.WithTimeFunc(p.now),
	)

	_, err = parser.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		return p.publicKeyForKID(ctx, kid)
	})
	if err != nil {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: invalid apple identity token", sherrors.ErrUnauthorized)
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return domain.OAuthProviderProfile{}, fmt.Errorf("%w: apple identity token missing sub", sherrors.ErrUnauthorized)
	}

	if claims.IssuedAt != nil {
		now := p.now()
		if claims.IssuedAt.Time.After(now.Add(appleClockSkew)) {
			return domain.OAuthProviderProfile{}, fmt.Errorf("%w: apple identity token iat is in the future", sherrors.ErrUnauthorized)
		}
		issuedAgo := now.Sub(claims.IssuedAt.Time)
		if issuedAgo > appleMaxTokenAge+appleClockSkew {
			return domain.OAuthProviderProfile{}, fmt.Errorf("%w: apple identity token is too old", sherrors.ErrUnauthorized)
		}
	}

	email := normalizeJWTString(claims.Email)
	emailVerified := parseJWTBool(claims.EmailVerified)
	if email == nil {
		emailVerified = false
	}

	return domain.OAuthProviderProfile{
		Provider:       domain.OAuthProviderApple,
		ProviderUserID: claims.Subject,
		Email:          email,
		EmailVerified:  emailVerified,
	}, nil
}

type appleIdentityTokenClaims struct {
	jwt.RegisteredClaims
	Email         *string `json:"email"`
	EmailVerified any     `json:"email_verified"`
}

type jwtHeader struct {
	KID string `json:"kid"`
}

type appleJWKS struct {
	Keys []appleJWK `json:"keys"`
}

type appleJWK struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	ALG string `json:"alg"`
	CRV string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (p *AppleOAuthProvider) publicKeyForKID(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	if key, ok := p.readCachedKey(kid); ok {
		return key, nil
	}

	if err := p.refreshJWKS(ctx); err != nil {
		return nil, err
	}
	if key, ok := p.readCachedKey(kid); ok {
		return key, nil
	}

	if err := p.refreshJWKS(ctx); err != nil {
		return nil, err
	}
	if key, ok := p.readCachedKey(kid); ok {
		return key, nil
	}

	return nil, fmt.Errorf("kid %q not found", kid)
}

func (p *AppleOAuthProvider) readCachedKey(kid string) (*ecdsa.PublicKey, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.now().After(p.cacheExpires) {
		return nil, false
	}
	key, ok := p.cachedKeys[kid]
	return key, ok
}

func (p *AppleOAuthProvider) refreshJWKS(ctx context.Context) error {
	var payload appleJWKS
	if err := getJSONWithHeaders(ctx, p.client, p.cfg.JWKSURL, nil, &payload); err != nil {
		return err
	}

	keys := make(map[string]*ecdsa.PublicKey, len(payload.Keys))
	for _, jwk := range payload.Keys {
		pub, err := jwkToECPublicKey(jwk)
		if err != nil {
			continue
		}
		keys[jwk.KID] = pub
	}

	p.mu.Lock()
	p.cachedKeys = keys
	p.cacheExpires = p.now().Add(p.cfg.JWKSCacheTTL)
	p.mu.Unlock()
	return nil
}

func parseJWTKid(tokenString string) (string, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed jwt")
	}

	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("decode header: %w", err)
	}

	var header jwtHeader
	if err := json.Unmarshal(rawHeader, &header); err != nil {
		return "", fmt.Errorf("unmarshal header: %w", err)
	}

	if strings.TrimSpace(header.KID) == "" {
		return "", fmt.Errorf("missing kid")
	}
	return header.KID, nil
}

func jwkToECPublicKey(jwk appleJWK) (*ecdsa.PublicKey, error) {
	if jwk.KTY != "EC" || jwk.CRV != "P-256" || strings.TrimSpace(jwk.KID) == "" {
		return nil, fmt.Errorf("unsupported jwk")
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}

	pub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}
	if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
		return nil, fmt.Errorf("point is not on p256 curve")
	}
	return pub, nil
}

func normalizeJWTString(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func parseJWTBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}
