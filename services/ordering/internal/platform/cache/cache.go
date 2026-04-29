package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache is the read-through/write-through cache interface.
type Cache interface {
	Get(ctx context.Context, key string, dst any) (bool, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// RedisCache implements Cache backed by Redis. Errors are non-fatal — callers
// should log at WARN and fall through to the DB (Cache-Aside pattern).
type RedisCache struct {
	client *redis.Client
}

// NewRedis parses the Redis URL and returns a RedisCache.
func NewRedis(redisURL string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}
	return &RedisCache{client: redis.NewClient(opts)}, nil
}

// Client exposes the underlying redis.Client for health checks.
func (c *RedisCache) Client() *redis.Client { return c.client }

// Close shuts down the Redis connection pool.
func (c *RedisCache) Close() error { return c.client.Close() }

func (c *RedisCache) Get(ctx context.Context, key string, dst any) (bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache get %q: %w", key, err)
	}
	if err = json.Unmarshal(data, dst); err != nil {
		return false, fmt.Errorf("cache unmarshal %q: %w", key, err)
	}
	return true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal %q: %w", key, err)
	}
	if err = c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache set %q: %w", key, err)
	}
	return nil
}

func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache delete: %w", err)
	}
	return nil
}

// Key prefixes for ordering-service.
const (
	KeyOrder    = "ordering:order:"
	KeySaga     = "ordering:saga:"
)
