package repository

import (
	"context"
	"encoding/json"
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
		name          string
		tokenStatus   int
		claims        map[string]any
		wantErr       error
		wantEmail     *string
		wantVerified  bool
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
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token": "access",
					"id_token":     buildUnsignedJWT(tt.claims),
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
