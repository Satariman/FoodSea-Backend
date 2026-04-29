//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerResp matches the identity register/login response shape.
type registerResp struct {
	Data struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		User    struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"data"`
}

type meResp struct {
	Data struct {
		ID             string `json:"id"`
		OnboardingDone bool   `json:"onboarding_done"`
	} `json:"data"`
}

func TestHTTPSmoke(t *testing.T) {
	var (
		email    = "smoke@foodsea.test"
		password = "SuperSecret1!"
		access   string
		refresh  string
	)

	t.Run("health", func(t *testing.T) {
		resp, err := httpClient.Get(testBaseURL + "/health")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("register", func(t *testing.T) {
		resp, err := postJSON(testBaseURL+"/api/v1/auth/register", map[string]string{
			"email":    email,
			"password": password,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body registerResp
		require.NoError(t, decodeJSON(resp, &body))
		require.NotEmpty(t, body.Data.Access)
		require.NotEmpty(t, body.Data.Refresh)
		access = body.Data.Access
		refresh = body.Data.Refresh
	})

	t.Run("login", func(t *testing.T) {
		resp, err := postJSON(testBaseURL+"/api/v1/auth/login", map[string]string{
			"email":    email,
			"password": password,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body registerResp
		require.NoError(t, decodeJSON(resp, &body))
		require.NotEmpty(t, body.Data.Access)
		access = body.Data.Access
		refresh = body.Data.Refresh
	})

	t.Run("me", func(t *testing.T) {
		resp, err := getAuth(testBaseURL+"/api/v1/users/me", access)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body meResp
		require.NoError(t, decodeJSON(resp, &body))
		assert.NotEmpty(t, body.Data.ID)
	})

	t.Run("onboarding", func(t *testing.T) {
		resp, err := postJSONAuth(testBaseURL+"/api/v1/users/me/onboarding", access, map[string]any{
			"dietary_preferences": []string{},
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("me_onboarding_done", func(t *testing.T) {
		resp, err := getAuth(testBaseURL+"/api/v1/users/me", access)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body meResp
		require.NoError(t, decodeJSON(resp, &body))
		assert.True(t, body.Data.OnboardingDone)
	})

	t.Run("categories", func(t *testing.T) {
		resp, err := httpClient.Get(testBaseURL + "/api/v1/categories")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("products_list", func(t *testing.T) {
		resp, err := httpClient.Get(testBaseURL + "/api/v1/products?page=1&page_size=5")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("products_detail", func(t *testing.T) {
		resp, err := httpClient.Get(testBaseURL + "/api/v1/products/" + seededProductID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("barcode", func(t *testing.T) {
		resp, err := httpClient.Get(testBaseURL + "/api/v1/barcode/" + seededProductBarcode)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("search", func(t *testing.T) {
		resp, err := httpClient.Get(testBaseURL + "/api/v1/search?q=%D0%BC%D0%BE%D0%BB%D0%BE%D0%BA%D0%BE")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("stores", func(t *testing.T) {
		resp, err := httpClient.Get(testBaseURL + "/api/v1/stores")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("offers", func(t *testing.T) {
		resp, err := httpClient.Get(testBaseURL + "/api/v1/products/" + seededProductID + "/offers")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("cart_add_item", func(t *testing.T) {
		resp, err := postJSONAuth(testBaseURL+"/api/v1/cart/items", access, map[string]any{
			"product_id": seededProductID,
			"quantity":   2,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("cart_get", func(t *testing.T) {
		resp, err := getAuth(testBaseURL+"/api/v1/cart", access)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("cart_update_item", func(t *testing.T) {
		resp, err := putJSONAuth(
			testBaseURL+"/api/v1/cart/items/"+seededProductID,
			access,
			map[string]any{"quantity": 5},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("cart_remove_item", func(t *testing.T) {
		resp, err := deleteAuth(testBaseURL+"/api/v1/cart/items/"+seededProductID, access)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("cart_clear", func(t *testing.T) {
		// Re-add item first
		resp, err := postJSONAuth(testBaseURL+"/api/v1/cart/items", access, map[string]any{
			"product_id": seededProductID,
			"quantity":   1,
		})
		require.NoError(t, err)
		resp.Body.Close()

		resp, err = deleteAuth(testBaseURL+"/api/v1/cart", access)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("logout", func(t *testing.T) {
		resp, err := postJSONAuth(testBaseURL+"/api/v1/auth/logout", access, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("refresh_after_logout_rejected", func(t *testing.T) {
		resp, err := postJSON(testBaseURL+"/api/v1/auth/refresh", map[string]string{
			"refresh_token": refresh,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	})

}
