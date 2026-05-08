package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/platform/config"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestYandexOAuthProvider_AuthURL(t *testing.T) {
	p := NewYandexOAuthProvider(config.OAuthProviderConfig{
		ClientID: "yandex-client",
		AuthURL:  "https://oauth.yandex.ru/authorize",
		Scopes:   []string{"login:email", "login:info"},
	}, http.DefaultClient)

	got, err := p.AuthURL(context.Background(), "state-y", domain.OAuthSession{RedirectTo: "https://app/cb"})
	require.NoError(t, err)

	u, err := url.Parse(got)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "yandex-client", q.Get("client_id"))
	assert.Equal(t, "https://app/cb", q.Get("redirect_uri"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "login:email login:info", q.Get("scope"))
	assert.Equal(t, "state-y", q.Get("state"))
}

func TestYandexOAuthProvider_NameAndDefaultClient(t *testing.T) {
	t.Parallel()

	p := NewYandexOAuthProvider(config.OAuthProviderConfig{}, nil)
	assert.Equal(t, domain.OAuthProviderYandex, p.Name())
	require.NotNil(t, p.client)
}

func TestYandexOAuthProvider_Exchange(t *testing.T) {
	type tc struct {
		name           string
		tokenStatus    int
		userInfoStatus int
		userInfoBody   map[string]any
		wantErr        error
		wantEmailNil   bool
		wantVerified   bool
	}

	tests := []tc{
		{
			name:           "exchange success",
			tokenStatus:    http.StatusOK,
			userInfoStatus: http.StatusOK,
			userInfoBody: map[string]any{
				"id":            "yandex-user-1",
				"default_email": "y@example.com",
			},
			wantVerified: true,
		},
		{
			name:        "token endpoint non-200 unauthorized",
			tokenStatus: http.StatusUnauthorized,
			wantErr:     sherrors.ErrUnauthorized,
		},
		{
			name:           "userinfo endpoint non-200 unauthorized",
			tokenStatus:    http.StatusOK,
			userInfoStatus: http.StatusForbidden,
			wantErr:        sherrors.ErrUnauthorized,
		},
		{
			name:           "userinfo without id unauthorized",
			tokenStatus:    http.StatusOK,
			userInfoStatus: http.StatusOK,
			userInfoBody: map[string]any{
				"default_email": "y@example.com",
			},
			wantErr: sherrors.ErrUnauthorized,
		},
		{
			name:           "userinfo without email -> nil false",
			tokenStatus:    http.StatusOK,
			userInfoStatus: http.StatusOK,
			userInfoBody: map[string]any{
				"id": "yandex-user-2",
			},
			wantEmailNil: true,
			wantVerified: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					if tt.tokenStatus != http.StatusOK {
						w.WriteHeader(tt.tokenStatus)
						_, _ = w.Write([]byte(`{"error":"bad"}`))
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-1"})
				case "/userinfo":
					if got := r.Header.Get("Authorization"); got != "OAuth access-1" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					if tt.userInfoStatus != http.StatusOK {
						w.WriteHeader(tt.userInfoStatus)
						return
					}
					_ = json.NewEncoder(w).Encode(tt.userInfoBody)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			p := NewYandexOAuthProvider(config.OAuthProviderConfig{
				ClientID:     "yandex-client",
				ClientSecret: "secret",
				TokenURL:     srv.URL + "/token",
				UserInfoURL:  srv.URL + "/userinfo",
			}, srv.Client())

			profile, err := p.Exchange(context.Background(), "code", domain.OAuthSession{
				RedirectTo: "https://app/cb",
			})
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.OAuthProviderYandex, profile.Provider)
			assert.NotEmpty(t, profile.ProviderUserID)
			assert.Equal(t, tt.wantVerified, profile.EmailVerified)
			if tt.wantEmailNil {
				assert.Nil(t, profile.Email)
			}
		})
	}
}
