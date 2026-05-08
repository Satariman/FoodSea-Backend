package repository

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPostFormAndDecodeJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	form := url.Values{"code": {"abc"}}

	t.Run("invalid endpoint", func(t *testing.T) {
		var dst map[string]any
		err := postFormAndDecodeJSON(ctx, http.DefaultClient, "://bad-url", form, &dst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "creating post request")
	})

	t.Run("transport error", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			}),
		}
		var dst map[string]any
		err := postFormAndDecodeJSON(ctx, client, "http://example.com/token", form, &dst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "performing post request")
	})

	t.Run("non-2xx response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad_request"}`))
		}))
		defer srv.Close()

		var dst map[string]any
		err := postFormAndDecodeJSON(ctx, srv.Client(), srv.URL, form, &dst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-2xx response")
	})

	t.Run("invalid json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "{")
		}))
		defer srv.Close()

		var dst map[string]any
		err := postFormAndDecodeJSON(ctx, srv.Client(), srv.URL, form, &dst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decoding json response")
	})

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Equal(t, "code=abc", string(body))
			_, _ = io.WriteString(w, `{"ok":true}`)
		}))
		defer srv.Close()

		var dst map[string]any
		err := postFormAndDecodeJSON(ctx, srv.Client(), srv.URL, form, &dst)
		require.NoError(t, err)
		assert.Equal(t, true, dst["ok"])
	})
}

func TestGetJSONWithHeaders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("invalid endpoint", func(t *testing.T) {
		var dst map[string]any
		err := getJSONWithHeaders(ctx, http.DefaultClient, "://bad-url", nil, &dst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "creating get request")
	})

	t.Run("transport error", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			}),
		}
		var dst map[string]any
		err := getJSONWithHeaders(ctx, client, "http://example.com/userinfo", nil, &dst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "performing get request")
	})

	t.Run("non-2xx response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `forbidden`)
		}))
		defer srv.Close()

		var dst map[string]any
		err := getJSONWithHeaders(ctx, srv.Client(), srv.URL, nil, &dst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-2xx response")
		assert.Contains(t, err.Error(), "status=403")
	})

	t.Run("invalid json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "{")
		}))
		defer srv.Close()

		var dst map[string]any
		err := getJSONWithHeaders(ctx, srv.Client(), srv.URL, nil, &dst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decoding json response")
	})

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.Equal(t, "Bearer abc", r.Header.Get("Authorization"))
			assert.Equal(t, "foodsea", r.Header.Get("X-Client"))
			_, _ = io.WriteString(w, `{"id":"u-1"}`)
		}))
		defer srv.Close()

		var dst map[string]any
		err := getJSONWithHeaders(ctx, srv.Client(), srv.URL, map[string]string{
			"Authorization": "Bearer abc",
			"X-Client":      "foodsea",
		}, &dst)
		require.NoError(t, err)
		assert.Equal(t, "u-1", dst["id"])
	})

	t.Run("status body truncated to 512 bytes", func(t *testing.T) {
		payload := strings.Repeat("x", 1024)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, payload)
		}))
		defer srv.Close()

		var dst map[string]any
		err := getJSONWithHeaders(ctx, srv.Client(), srv.URL, nil, &dst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status=502")
		assert.LessOrEqual(t, len(err.Error()), 700)
	})
}
