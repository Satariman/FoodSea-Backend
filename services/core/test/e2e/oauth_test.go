//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type oauthStartResp struct {
	Data struct {
		AuthURL string `json:"auth_url"`
		State   string `json:"state"`
	} `json:"data"`
}

func TestOAuthE2E(t *testing.T) {
	t.Run("google_start_callback_refresh_and_replay_protection", func(t *testing.T) {
		startURL := testBaseURL + "/api/v1/auth/oauth/google/start?redirect_uri=" + url.QueryEscape(testGoogleRedirectURI)
		resp, err := httpClient.Get(startURL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var start oauthStartResp
		require.NoError(t, decodeJSON(resp, &start))
		require.NotEmpty(t, start.Data.State)
		require.NotEmpty(t, start.Data.AuthURL)

		code := fmt.Sprintf("google-new:%s", start.Data.State)
		resp, err = postJSON(testBaseURL+"/api/v1/auth/oauth/google/callback", map[string]string{
			"code":         code,
			"state":        start.Data.State,
			"redirect_uri": testGoogleRedirectURI,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var auth registerResp
		require.NoError(t, decodeJSON(resp, &auth))
		require.NotEmpty(t, auth.Data.Access)
		require.NotEmpty(t, auth.Data.Refresh)

		resp, err = postJSON(testBaseURL+"/api/v1/auth/refresh", map[string]string{"refresh_token": auth.Data.Refresh})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Replay of the same state must be rejected.
		resp, err = postJSON(testBaseURL+"/api/v1/auth/oauth/google/callback", map[string]string{
			"code":         code,
			"state":        start.Data.State,
			"redirect_uri": testGoogleRedirectURI,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("yandex_start_callback", func(t *testing.T) {
		startURL := testBaseURL + "/api/v1/auth/oauth/yandex/start?redirect_uri=" + url.QueryEscape(testYandexRedirectURI)
		resp, err := httpClient.Get(startURL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var start oauthStartResp
		require.NoError(t, decodeJSON(resp, &start))
		require.NotEmpty(t, start.Data.State)

		resp, err = postJSON(testBaseURL+"/api/v1/auth/oauth/yandex/callback", map[string]string{
			"code":         "yandex-code-new-user",
			"state":        start.Data.State,
			"redirect_uri": testYandexRedirectURI,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("verified_oauth_email_links_existing_password_user", func(t *testing.T) {
		email := "linked-oauth@foodsea.test"
		password := "SuperSecret1!"

		resp, err := postJSON(testBaseURL+"/api/v1/auth/register", map[string]string{
			"email":    email,
			"password": password,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		startURL := testBaseURL + "/api/v1/auth/oauth/google/start?redirect_uri=" + url.QueryEscape(testGoogleRedirectURI)
		resp, err = httpClient.Get(startURL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var start oauthStartResp
		require.NoError(t, decodeJSON(resp, &start))

		code := fmt.Sprintf("google-link:%s", start.Data.State)
		resp, err = postJSON(testBaseURL+"/api/v1/auth/oauth/google/callback", map[string]string{
			"code":         code,
			"state":        start.Data.State,
			"redirect_uri": testGoogleRedirectURI,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Existing password login must still work after OAuth linking.
		resp, err = postJSON(testBaseURL+"/api/v1/auth/login", map[string]string{
			"email":    email,
			"password": password,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})
}
