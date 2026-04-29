package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/catalog/usecase"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestListCategories_CacheHit(t *testing.T) {
	catRepo := &MockCategoryRepository{}
	c := &MockProductCache{}

	tree := []domain.Category{fakeCategory("Root")}
	c.On("GetCategoriesTree", mock.Anything).Return(tree, nil)

	uc := usecase.NewListCategories(catRepo, c, silentLogger())
	result, err := uc.Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, tree, result)
	catRepo.AssertNotCalled(t, "ListAll")
}

func TestListCategories_CacheMiss_StoresResult(t *testing.T) {
	catRepo := &MockCategoryRepository{}
	c := &MockProductCache{}

	ctx := context.Background()
	c.On("GetCategoriesTree", ctx).Return(nil, nil)

	flat := []domain.Category{
		{ID: uuid.New(), Name: "Root1", SortOrder: 1},
		{ID: uuid.New(), Name: "Root2", SortOrder: 2},
	}
	catRepo.On("ListAll", ctx).Return(flat, nil)
	c.On("SetCategoriesTree", ctx, mock.Anything).Return(nil)

	uc := usecase.NewListCategories(catRepo, c, silentLogger())
	result, err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	c.AssertCalled(t, "SetCategoriesTree", ctx, mock.Anything)
}

func TestListCategories_BuildsTreeWithSorting(t *testing.T) {
	catRepo := &MockCategoryRepository{}
	c := &MockProductCache{}

	ctx := context.Background()
	c.On("GetCategoriesTree", ctx).Return(nil, nil)

	rootID := uuid.New()
	root := domain.Category{ID: rootID, Name: "Root", SortOrder: 0}
	child1 := domain.Category{ID: uuid.New(), Name: "B-Child", ParentID: &rootID, SortOrder: 2}
	child2 := domain.Category{ID: uuid.New(), Name: "A-Child", ParentID: &rootID, SortOrder: 1}

	catRepo.On("ListAll", ctx).Return([]domain.Category{root, child1, child2}, nil)
	c.On("SetCategoriesTree", ctx, mock.Anything).Return(nil)

	uc := usecase.NewListCategories(catRepo, c, silentLogger())
	result, err := uc.Execute(ctx)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Root", result[0].Name)
	require.Len(t, result[0].Children, 2)
	// sorted by SortOrder: A-Child (sort=1) before B-Child (sort=2)
	assert.Equal(t, "A-Child", result[0].Children[0].Name)
	assert.Equal(t, "B-Child", result[0].Children[1].Name)
}

func TestListCategories_CacheGetError_FallsBackToDB(t *testing.T) {
	catRepo := &MockCategoryRepository{}
	c := &MockProductCache{}

	ctx := context.Background()
	c.On("GetCategoriesTree", ctx).Return(nil, errors.New("redis down"))

	flat := []domain.Category{fakeCategory("Root")}
	catRepo.On("ListAll", ctx).Return(flat, nil)
	c.On("SetCategoriesTree", ctx, mock.Anything).Return(nil)

	uc := usecase.NewListCategories(catRepo, c, silentLogger())
	result, err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	catRepo.AssertCalled(t, "ListAll", ctx)
}

func TestListCategories_CacheSetError_ReturnsResult(t *testing.T) {
	catRepo := &MockCategoryRepository{}
	c := &MockProductCache{}

	ctx := context.Background()
	c.On("GetCategoriesTree", ctx).Return(nil, nil)

	flat := []domain.Category{fakeCategory("Root")}
	catRepo.On("ListAll", ctx).Return(flat, nil)
	c.On("SetCategoriesTree", ctx, mock.Anything).Return(errors.New("redis down"))

	uc := usecase.NewListCategories(catRepo, c, silentLogger())
	result, err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}
