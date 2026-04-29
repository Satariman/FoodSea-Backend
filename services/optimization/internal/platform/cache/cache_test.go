package cache_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/optimization/internal/platform/cache"
)

func TestCacheKeys(t *testing.T) {
	assert.Equal(t, "optimization:result:id:", cache.KeyResultByID)
	assert.Equal(t, "optimization:result:hash:", cache.KeyResultByHash)
	assert.Equal(t, "optimization:analogs:", cache.KeyAnalogs)
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

func TestRedisCache_NewRedis_InvalidURL(t *testing.T) {
	_, err := cache.NewRedis("not-a-valid-url")
	assert.Error(t, err)
}

func TestRedisCache_DeleteEmpty(t *testing.T) {
	rc := &mockCache{}
	err := rc.Delete(context.Background())
	assert.NoError(t, err)
}

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
