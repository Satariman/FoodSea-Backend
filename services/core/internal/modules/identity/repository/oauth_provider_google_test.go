package repository

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/platform/config"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestGoogleOAuthProvider_NameAndDefaultClient(t *testing.T) {
	t.Parallel()

	p := NewGoogleOAuthProvider(config.OAuthProviderConfig{}, nil)
	assert.Equal(t, domain.OAuthProviderGoogle, p.Name())
	require.NotNil(t, p.client)
}

func TestGoogleOAuthProvider_AuthURL(t *testing.T) {
	p := NewGoogleOAuthProvider(config.OAuthProviderConfig{
		ClientID: "google-client",
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		Scopes:   []string{"openid", "email", "profile"},
	}, http.DefaultClient)

	got, err := p.AuthURL(context.Background(), "state-1", domain.OAuthSession{RedirectTo: "https://app/callback"})
	require.NoError(t, err)

	u, err := url.Parse(got)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "google-client", q.Get("client_id"))
	assert.Equal(t, "https://app/callback", q.Get("redirect_uri"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "openid email profile", q.Get("scope"))
	assert.Equal(t, "state-1", q.Get("state"))
	assert.Equal(t, "state-1", q.Get("nonce"))
}

func TestGoogleOAuthProvider_Exchange(t *testing.T) {
	type tc struct {
		name         string
		tokenStatus  int
		claims       map[string]any
		wantErr      error
		wantEmail    *string
		wantVerified bool
	}

	nowExp := time.Now().Add(time.Hour).Unix()
	email := "user@example.com"
	tests := []tc{
		{
			name:        "exchange success",
			tokenStatus: http.StatusOK,
			claims: map[string]any{
				"iss":            "https://accounts.google.com",
				"aud":            "google-client",
				"exp":            nowExp,
				"nonce":          "state-1",
				"sub":            "google-sub-1",
				"email":          email,
				"email_verified": true,
			},
			wantEmail:    &email,
			wantVerified: true,
		},
		{
			name:        "token endpoint non-200 unauthorized",
			tokenStatus: http.StatusUnauthorized,
			wantErr:     sherrors.ErrUnauthorized,
		},
		{
			name:        "malformed id token unauthorized",
			tokenStatus: http.StatusOK,
			claims:      nil,
			wantErr:     sherrors.ErrUnauthorized,
		},
		{
			name:        "wrong nonce unauthorized",
			tokenStatus: http.StatusOK,
			claims: map[string]any{
				"iss":            "https://accounts.google.com",
				"aud":            "google-client",
				"exp":            nowExp,
				"nonce":          "wrong",
				"sub":            "google-sub-1",
				"email":          email,
				"email_verified": true,
			},
			wantErr: sherrors.ErrUnauthorized,
		},
		{
			name:        "wrong audience unauthorized",
			tokenStatus: http.StatusOK,
			claims: map[string]any{
				"iss":            "https://accounts.google.com",
				"aud":            "other-client",
				"exp":            nowExp,
				"nonce":          "state-1",
				"sub":            "google-sub-1",
				"email":          email,
				"email_verified": true,
			},
			wantErr: sherrors.ErrUnauthorized,
		},
		{
			name:        "unverified email profile behavior",
			tokenStatus: http.StatusOK,
			claims: map[string]any{
				"iss":            "https://accounts.google.com",
				"aud":            "google-client",
				"exp":            nowExp,
				"nonce":          "state-1",
				"sub":            "google-sub-2",
				"email":          "raw@example.com",
				"email_verified": false,
			},
			wantVerified: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/token" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if tt.tokenStatus != http.StatusOK {
					w.WriteHeader(tt.tokenStatus)
					_, _ = w.Write([]byte(`{"error":"bad"}`))
					return
				}
				idToken := "malformed"
				if tt.claims != nil {
					idToken = buildUnsignedJWT(tt.claims)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token": "access",
					"id_token":     idToken,
				})
			}))
			defer srv.Close()

			p := NewGoogleOAuthProvider(config.OAuthProviderConfig{
				ClientID:     "google-client",
				ClientSecret: "secret",
				TokenURL:     srv.URL + "/token",
			}, srv.Client())

			profile, err := p.Exchange(context.Background(), "code", domain.OAuthSession{
				State:      "state-1",
				RedirectTo: "https://app/callback",
			})
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.OAuthProviderGoogle, profile.Provider)
			assert.NotEmpty(t, profile.ProviderUserID)
			assert.Equal(t, tt.wantVerified, profile.EmailVerified)
			if tt.wantEmail == nil {
				if profile.Email != nil {
					assert.NotEmpty(t, *profile.Email)
				}
			} else {
				require.NotNil(t, profile.Email)
				assert.Equal(t, *tt.wantEmail, *profile.Email)
			}
		})
	}
}

func TestParseGoogleIDTokenClaims(t *testing.T) {
	t.Parallel()

	t.Run("malformed jwt", func(t *testing.T) {
		_, err := parseGoogleIDTokenClaims("not-a-jwt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed jwt")
	})

	t.Run("invalid payload base64", func(t *testing.T) {
		_, err := parseGoogleIDTokenClaims("a.!.c")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode payload")
	})

	t.Run("invalid payload json", func(t *testing.T) {
		token := "a." + "eA" + ".c"
		_, err := parseGoogleIDTokenClaims(token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal claims")
	})

	t.Run("success", func(t *testing.T) {
		email := "ok@example.com"
		token := buildUnsignedJWT(map[string]any{
			"iss":            "https://accounts.google.com",
			"aud":            "client-1",
			"exp":            time.Now().Add(time.Hour).Unix(),
			"nonce":          "state-1",
			"sub":            "sub-1",
			"email":          email,
			"email_verified": true,
		})
		claims, err := parseGoogleIDTokenClaims(token)
		require.NoError(t, err)
		assert.Equal(t, "sub-1", claims.Sub)
		require.NotNil(t, claims.Email)
		assert.Equal(t, email, *claims.Email)
	})
}

func TestValidateGoogleClaims(t *testing.T) {
	t.Parallel()

	base := googleIDTokenClaims{
		Iss:   "https://accounts.google.com",
		Aud:   "client-1",
		Exp:   time.Now().Add(time.Hour).Unix(),
		Nonce: "state-1",
		Sub:   "sub-1",
	}

	tests := []struct {
		name    string
		claims  googleIDTokenClaims
		client  string
		nonce   string
		wantErr error
	}{
		{name: "success", claims: base, client: "client-1", nonce: "state-1"},
		{name: "success alt issuer", claims: func() googleIDTokenClaims { c := base; c.Iss = "accounts.google.com"; return c }(), client: "client-1", nonce: "state-1"},
		{name: "invalid issuer", claims: func() googleIDTokenClaims { c := base; c.Iss = "issuer"; return c }(), client: "client-1", nonce: "state-1", wantErr: errors.New("invalid issuer")},
		{name: "invalid audience", claims: base, client: "other", nonce: "state-1", wantErr: errors.New("invalid audience")},
		{name: "empty client", claims: base, client: "", nonce: "state-1", wantErr: errors.New("invalid audience")},
		{name: "expired token", claims: func() googleIDTokenClaims { c := base; c.Exp = time.Now().Add(-time.Minute).Unix(); return c }(), client: "client-1", nonce: "state-1", wantErr: errors.New("token expired")},
		{name: "empty expected nonce", claims: base, client: "client-1", nonce: "", wantErr: errors.New("invalid nonce")},
		{name: "nonce mismatch", claims: base, client: "client-1", nonce: "other", wantErr: errors.New("invalid nonce")},
		{name: "missing sub", claims: func() googleIDTokenClaims { c := base; c.Sub = " "; return c }(), client: "client-1", nonce: "state-1", wantErr: errors.New("missing sub")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := validateGoogleClaims(tt.claims, tt.client, tt.nonce)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr.Error(), err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
