package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/search/domain"
	"github.com/foodsea/core/internal/modules/search/repository"
	"github.com/foodsea/core/internal/platform/cache"
)

// cacheDouble is a testify/mock implementation of cache.Cache.
type cacheDouble struct{ mock.Mock }

var _ cache.Cache = (*cacheDouble)(nil)

func (c *cacheDouble) Get(ctx context.Context, key string, dst any) (bool, error) {
	args := c.Called(ctx, key, dst)
	return args.Bool(0), args.Error(1)
}

func (c *cacheDouble) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	args := c.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (c *cacheDouble) Delete(ctx context.Context, keys ...string) error {
	args := c.Called(ctx, keys)
	return args.Error(0)
}

func TestSearchCache_Get_Miss(t *testing.T) {
	c := new(cacheDouble)
	c.On("Get", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(false, nil)

	sc := repository.NewSearchCache(c)
	result, err := sc.Get(context.Background(), "abc123")

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestSearchCache_Get_Error(t *testing.T) {
	c := new(cacheDouble)
	c.On("Get", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(false, errors.New("redis down"))

	sc := repository.NewSearchCache(c)
	_, err := sc.Get(context.Background(), "abc123")

	require.Error(t, err)
}

func TestSearchCache_Set_Success(t *testing.T) {
	c := new(cacheDouble)
	c.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, 5*time.Minute).Return(nil)

	sc := repository.NewSearchCache(c)
	err := sc.Set(context.Background(), "abc123", &domain.SearchResult{Total: 1})

	require.NoError(t, err)
	c.AssertExpectations(t)
}

func TestSearchCache_Set_Error(t *testing.T) {
	c := new(cacheDouble)
	c.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, 5*time.Minute).Return(errors.New("redis down"))

	sc := repository.NewSearchCache(c)
	err := sc.Set(context.Background(), "abc123", &domain.SearchResult{})

	require.Error(t, err)
}

func TestSearchCache_Key_HasSearchPrefix(t *testing.T) {
	c := new(cacheDouble)
	var capturedKey string
	c.On("Get", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Run(func(args mock.Arguments) { capturedKey = args.String(1) }).
		Return(false, nil)

	sc := repository.NewSearchCache(c)
	_, _ = sc.Get(context.Background(), "myhash")

	assert.True(t, len(capturedKey) > 0)
	assert.Contains(t, capturedKey, "search:")
	assert.Contains(t, capturedKey, "myhash")
}
