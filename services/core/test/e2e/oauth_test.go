//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

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

		t.Run("apple_native_callback_happy_path", func(t *testing.T) {
		email := "apple-new@foodsea.test"
		token := signAppleIdentityToken(
			"apple-sub-new",
			testAppleClientID,
			testAppleIssuer,
			&email,
			time.Now().Add(5*time.Minute),
			time.Now(),
		)

		resp, err := postJSON(testBaseURL+"/api/v1/auth/oauth/native/apple/callback", map[string]string{
			"identity_token": token,
			"email":          email,
			"full_name":      "Apple User",
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var auth registerResp
		require.NoError(t, decodeJSON(resp, &auth))
			require.NotEmpty(t, auth.Data.Access)
			require.NotEmpty(t, auth.Data.Refresh)
		})

		t.Run("apple_native_callback_repeat_same_sub_returns_same_user", func(t *testing.T) {
			email := "apple-repeat@foodsea.test"
			sub := "apple-sub-repeat"
			issuedAt := time.Now()

			firstToken := signAppleIdentityToken(
				sub,
				testAppleClientID,
				testAppleIssuer,
				&email,
				issuedAt.Add(5*time.Minute),
				issuedAt,
			)

			firstResp, err := postJSON(testBaseURL+"/api/v1/auth/oauth/native/apple/callback", map[string]string{
				"identity_token": firstToken,
				"email":          email,
				"full_name":      "Apple Repeat User",
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, firstResp.StatusCode)

			var firstAuth registerResp
			require.NoError(t, decodeJSON(firstResp, &firstAuth))
			require.NotEmpty(t, firstAuth.Data.User.ID)
			require.NotEmpty(t, firstAuth.Data.Access)
			require.NotEmpty(t, firstAuth.Data.Refresh)

			secondIssuedAt := issuedAt.Add(30 * time.Second)
			secondToken := signAppleIdentityToken(
				sub,
				testAppleClientID,
				testAppleIssuer,
				nil,
				secondIssuedAt.Add(5*time.Minute),
				secondIssuedAt,
			)

			secondResp, err := postJSON(testBaseURL+"/api/v1/auth/oauth/native/apple/callback", map[string]string{
				"identity_token": secondToken,
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, secondResp.StatusCode)

			var secondAuth registerResp
			require.NoError(t, decodeJSON(secondResp, &secondAuth))
			require.NotEmpty(t, secondAuth.Data.User.ID)
			require.NotEmpty(t, secondAuth.Data.Access)
			require.NotEmpty(t, secondAuth.Data.Refresh)
			assert.Equal(t, firstAuth.Data.User.ID, secondAuth.Data.User.ID)

			meRespRaw, err := getAuth(testBaseURL+"/api/v1/users/me", secondAuth.Data.Access)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, meRespRaw.StatusCode)

			var me meResp
			require.NoError(t, decodeJSON(meRespRaw, &me))
			assert.Equal(t, firstAuth.Data.User.ID, me.Data.ID)
		})

		t.Run("apple_native_callback_malformed_token_unauthorized", func(t *testing.T) {
		resp, err := postJSON(testBaseURL+"/api/v1/auth/oauth/native/apple/callback", map[string]string{
			"identity_token": "not-a-jwt",
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("apple_native_callback_wrong_audience_unauthorized", func(t *testing.T) {
		token := signAppleIdentityToken(
			"apple-sub-bad-aud",
			"wrong-client",
			testAppleIssuer,
			nil,
			time.Now().Add(5*time.Minute),
			time.Now(),
		)
		resp, err := postJSON(testBaseURL+"/api/v1/auth/oauth/native/apple/callback", map[string]string{
			"identity_token": token,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("apple_native_callback_email_conflict", func(t *testing.T) {
		email := "apple-conflict@foodsea.test"
		password := "SuperSecret1!"

		resp, err := postJSON(testBaseURL+"/api/v1/auth/register", map[string]string{
			"email":    email,
			"password": password,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		token := signAppleIdentityToken(
			"apple-sub-conflict",
			testAppleClientID,
			testAppleIssuer,
			&email,
			time.Now().Add(5*time.Minute),
			time.Now(),
		)
		resp, err = postJSON(testBaseURL+"/api/v1/auth/oauth/native/apple/callback", map[string]string{
			"identity_token": token,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		resp.Body.Close()
	})
}
