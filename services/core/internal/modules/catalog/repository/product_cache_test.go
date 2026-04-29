package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/catalog/repository"
)

// mockCache mocks platform/cache.Cache.
type mockCache struct{ mock.Mock }

func (m *mockCache) Get(ctx context.Context, key string, dst any) (bool, error) {
	args := m.Called(ctx, key, dst)
	return args.Bool(0), args.Error(1)
}

func (m *mockCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *mockCache) Delete(ctx context.Context, keys ...string) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

func TestProductCache_GetProduct_Miss(t *testing.T) {
	c := &mockCache{}
	id := uuid.New()
	key := "catalog:product:" + id.String()

	c.On("Get", mock.Anything, key, mock.Anything).Return(false, nil)

	pc := repository.NewProductCache(c)
	result, err := pc.GetProduct(context.Background(), id)

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestProductCache_GetProduct_Error(t *testing.T) {
	c := &mockCache{}
	id := uuid.New()
	key := "catalog:product:" + id.String()

	c.On("Get", mock.Anything, key, mock.Anything).Return(false, errors.New("redis down"))

	pc := repository.NewProductCache(c)
	_, err := pc.GetProduct(context.Background(), id)
	assert.Error(t, err)
}

func TestProductCache_SetProduct(t *testing.T) {
	c := &mockCache{}
	detail := &domain.ProductDetail{Product: domain.Product{ID: uuid.New()}}
	key := "catalog:product:" + detail.ID.String()

	c.On("Set", mock.Anything, key, detail, 15*time.Minute).Return(nil)

	pc := repository.NewProductCache(c)
	err := pc.SetProduct(context.Background(), detail)
	require.NoError(t, err)
}

func TestProductCache_GetCategoriesTree_Miss(t *testing.T) {
	c := &mockCache{}
	c.On("Get", mock.Anything, "catalog:categories", mock.Anything).Return(false, nil)

	pc := repository.NewProductCache(c)
	result, err := pc.GetCategoriesTree(context.Background())

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestProductCache_SetCategoriesTree(t *testing.T) {
	c := &mockCache{}
	tree := []domain.Category{{ID: uuid.New(), Name: "Root"}}
	c.On("Set", mock.Anything, "catalog:categories", tree, 30*time.Minute).Return(nil)

	pc := repository.NewProductCache(c)
	err := pc.SetCategoriesTree(context.Background(), tree)
	require.NoError(t, err)
}

func TestProductCache_Invalidate(t *testing.T) {
	c := &mockCache{}
	id := uuid.New()
	c.On("Delete", mock.Anything, []string{"catalog:product:" + id.String()}).Return(nil)

	pc := repository.NewProductCache(c)
	err := pc.Invalidate(context.Background(), id)
	require.NoError(t, err)
}
