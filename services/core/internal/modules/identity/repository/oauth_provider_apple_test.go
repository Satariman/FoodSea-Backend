package repository

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/platform/config"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestAppleOAuthProvider_NameAndUnsupportedMethods(t *testing.T) {
	t.Parallel()

	p := NewAppleOAuthProvider(config.OAuthAppleConfig{}, nil)
	assert.Equal(t, domain.OAuthProviderApple, p.Name())
	require.NotNil(t, p.client)

	_, err := p.AuthURL(context.Background(), "state", domain.OAuthSession{})
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)

	_, err = p.Exchange(context.Background(), "code", domain.OAuthSession{})
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
}

func TestAppleOAuthProvider_ProfileFromToken(t *testing.T) {
	t.Parallel()

	now := time.Now()
	mainKey := mustECDSAKey(t)
	otherKey := mustECDSAKey(t)
	mainKid := "kid-main"
	otherKid := "kid-other"

	newProvider := func(url string) *AppleOAuthProvider {
		p := NewAppleOAuthProvider(config.OAuthAppleConfig{
			Enabled:      true,
			ClientID:     "me.foodSea",
			JWKSURL:      url,
			JWKSCacheTTL: time.Hour,
			Issuer:       "https://appleid.apple.com",
		}, nil)
		p.now = func() time.Time { return now }
		return p
	}

	t.Run("valid token success", func(t *testing.T) {
		jwks := jwksPayload(mainKid, mainKey)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(jwks)
		}))
		defer srv.Close()

		p := newProvider(srv.URL)
		token := mustSignAppleToken(t, mainKey, mainKid, appleClaims{
			Iss:           "https://appleid.apple.com",
			Aud:           "me.foodSea",
			Sub:           "apple-sub-1",
			Email:         ptrString("apple@foodsea.test"),
			EmailVerified: true,
			Iat:           now.Unix(),
			Exp:           now.Add(5 * time.Minute).Unix(),
		})

		profile, err := p.ProfileFromToken(context.Background(), token)
		require.NoError(t, err)
		assert.Equal(t, domain.OAuthProviderApple, profile.Provider)
		assert.Equal(t, "apple-sub-1", profile.ProviderUserID)
		require.NotNil(t, profile.Email)
		assert.Equal(t, "apple@foodsea.test", *profile.Email)
		assert.True(t, profile.EmailVerified)
	})

	t.Run("invalid signature unauthorized", func(t *testing.T) {
		jwks := jwksPayload(mainKid, mainKey)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(jwks)
		}))
		defer srv.Close()

		p := newProvider(srv.URL)
		token := mustSignAppleToken(t, otherKey, mainKid, appleClaims{
			Iss: "https://appleid.apple.com", Aud: "me.foodSea", Sub: "apple-sub-1",
			Iat: now.Unix(), Exp: now.Add(5 * time.Minute).Unix(),
		})

		_, err := p.ProfileFromToken(context.Background(), token)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("wrong audience unauthorized", func(t *testing.T) {
		jwks := jwksPayload(mainKid, mainKey)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(jwks)
		}))
		defer srv.Close()

		p := newProvider(srv.URL)
		token := mustSignAppleToken(t, mainKey, mainKid, appleClaims{
			Iss: "https://appleid.apple.com", Aud: "wrong-client", Sub: "apple-sub-1",
			Iat: now.Unix(), Exp: now.Add(5 * time.Minute).Unix(),
		})

		_, err := p.ProfileFromToken(context.Background(), token)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("wrong issuer unauthorized", func(t *testing.T) {
		jwks := jwksPayload(mainKid, mainKey)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(jwks)
		}))
		defer srv.Close()

		p := newProvider(srv.URL)
		token := mustSignAppleToken(t, mainKey, mainKid, appleClaims{
			Iss: "https://issuer.example", Aud: "me.foodSea", Sub: "apple-sub-1",
			Iat: now.Unix(), Exp: now.Add(5 * time.Minute).Unix(),
		})

		_, err := p.ProfileFromToken(context.Background(), token)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("expired token unauthorized", func(t *testing.T) {
		jwks := jwksPayload(mainKid, mainKey)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(jwks)
		}))
		defer srv.Close()

		p := newProvider(srv.URL)
		token := mustSignAppleToken(t, mainKey, mainKid, appleClaims{
			Iss: "https://appleid.apple.com", Aud: "me.foodSea", Sub: "apple-sub-1",
			Iat: now.Add(-10 * time.Minute).Unix(), Exp: now.Add(-1 * time.Minute).Unix(),
		})

		_, err := p.ProfileFromToken(context.Background(), token)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("kid miss triggers refresh", func(t *testing.T) {
		var requests int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			call := atomic.AddInt32(&requests, 1)
			switch call {
			case 1:
				_ = json.NewEncoder(w).Encode(jwksPayload(mainKid, mainKey))
			default:
				_ = json.NewEncoder(w).Encode(jwksPayload(otherKid, otherKey))
			}
		}))
		defer srv.Close()

		p := newProvider(srv.URL)
		token := mustSignAppleToken(t, otherKey, otherKid, appleClaims{
			Iss: "https://appleid.apple.com", Aud: "me.foodSea", Sub: "apple-sub-refresh",
			Iat: now.Unix(), Exp: now.Add(5 * time.Minute).Unix(),
		})

		profile, err := p.ProfileFromToken(context.Background(), token)
		require.NoError(t, err)
		assert.Equal(t, "apple-sub-refresh", profile.ProviderUserID)
		assert.EqualValues(t, 2, atomic.LoadInt32(&requests))
	})

	t.Run("jwks fetch failure unauthorized", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unavailable", http.StatusBadGateway)
		}))
		defer srv.Close()

		p := newProvider(srv.URL)
		token := mustSignAppleToken(t, mainKey, mainKid, appleClaims{
			Iss: "https://appleid.apple.com", Aud: "me.foodSea", Sub: "apple-sub-1",
			Iat: now.Unix(), Exp: now.Add(5 * time.Minute).Unix(),
		})

		_, err := p.ProfileFromToken(context.Background(), token)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})
}

type appleClaims struct {
	Iss           string
	Aud           string
	Sub           string
	Email         *string
	EmailVerified any
	Iat           int64
	Exp           int64
}

func mustECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func jwksPayload(kid string, key *ecdsa.PrivateKey) map[string]any {
	x := base64.RawURLEncoding.EncodeToString(key.PublicKey.X.Bytes())
	y := base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.Bytes())
	return map[string]any{
		"keys": []map[string]any{
			{
				"kty": "EC",
				"kid": kid,
				"use": "sig",
				"alg": "ES256",
				"crv": "P-256",
				"x":   x,
				"y":   y,
			},
		},
	}
}

func mustSignAppleToken(t *testing.T, key *ecdsa.PrivateKey, kid string, claims appleClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss":            claims.Iss,
		"aud":            claims.Aud,
		"sub":            claims.Sub,
		"email":          claims.Email,
		"email_verified": claims.EmailVerified,
		"iat":            claims.Iat,
		"exp":            claims.Exp,
	})
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func ptrString(v string) *string {
	return &v
}
