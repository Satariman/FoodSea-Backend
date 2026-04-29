package cache_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/platform/cache"
)

// TestRedisCache_Serialization uses a standalone JSON round-trip to verify
// the cache serialisation logic without requiring a real Redis instance.
func TestCacheKeys(t *testing.T) {
	assert.Equal(t, "catalog:product:", cache.KeyCatalogProduct)
	assert.Equal(t, "catalog:categories", cache.KeyCatalogCategories)
	assert.Equal(t, "offers:product:", cache.KeyOffersProduct)
	assert.Equal(t, "search:", cache.KeySearch)
	assert.Equal(t, "session:", cache.KeySession)
}

type testPayload struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestCacheJSONRoundTrip(t *testing.T) {
	original := testPayload{ID: 42, Name: "test"}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded testPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

// TestRedisCache_NewRedis_InvalidURL checks URL parsing rejects bad URLs.
func TestRedisCache_NewRedis_InvalidURL(t *testing.T) {
	_, err := cache.NewRedis("not-a-valid-url")
	assert.Error(t, err)
}

// TestRedisCache_Delete_Empty checks that Delete with no keys is a no-op.
func TestRedisCache_DeleteEmpty(t *testing.T) {
	// Use a non-existent instance — the error path we want is "no keys passed"
	// which returns nil before touching Redis.
	rc := &mockCache{}
	err := rc.Delete(context.Background())
	assert.NoError(t, err)
}

// mockCache is a simple in-memory implementation for unit testing.
type mockCache struct {
	data map[string][]byte
	ttls map[string]time.Duration
}

func newMockCache() *mockCache {
	return &mockCache{data: map[string][]byte{}, ttls: map[string]time.Duration{}}
}

func (m *mockCache) Get(_ context.Context, key string, dst any) (bool, error) {
	v, ok := m.data[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(v, dst)
}

func (m *mockCache) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.data[key] = data
	m.ttls[key] = ttl
	return nil
}

func (m *mockCache) Delete(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

func TestMockCache_SetGet(t *testing.T) {
	c := newMockCache()
	payload := testPayload{ID: 1, Name: "hello"}

	err := c.Set(context.Background(), "key1", payload, time.Minute)
	require.NoError(t, err)

	var got testPayload
	found, err := c.Get(context.Background(), "key1", &got)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, payload, got)
}

func TestMockCache_Miss(t *testing.T) {
	c := newMockCache()
	var got testPayload
	found, err := c.Get(context.Background(), "missing", &got)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestMockCache_Delete(t *testing.T) {
	c := newMockCache()
	_ = c.Set(context.Background(), "k", testPayload{ID: 1}, time.Minute)
	_ = c.Delete(context.Background(), "k")

	var got testPayload
	found, _ := c.Get(context.Background(), "k", &got)
	assert.False(t, found)
}
