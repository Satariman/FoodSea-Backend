package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/catalog/usecase"
	shared "github.com/foodsea/core/internal/shared/domain"
)

func TestListProducts_ByCategoryID(t *testing.T) {
	repo := &MockProductRepository{}
	catRepo := &MockCategoryRepository{}

	catID := uuid.New()
	filter := domain.ProductFilter{
		CategoryID: &catID,
		Pagination: shared.NewPagination(1, 20),
	}

	products := []domain.Product{fakeProduct()}
	repo.On("List", mock.Anything, mock.MatchedBy(func(f domain.ProductFilter) bool {
		return f.CategoryID != nil && *f.CategoryID == catID
	})).Return(products, 1, nil)

	uc := usecase.NewListProducts(repo, catRepo, silentLogger())
	result, total, err := uc.Execute(context.Background(), filter)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, total)
}

func TestListProducts_SubcategoryWithoutCategory_ResolvesParent(t *testing.T) {
	repo := &MockProductRepository{}
	catRepo := &MockCategoryRepository{}

	parentID := uuid.New()
	subID := uuid.New()
	sub := &domain.Category{ID: subID, Name: "Sub", ParentID: &parentID}

	catRepo.On("GetByID", mock.Anything, subID).Return(sub, nil)

	filter := domain.ProductFilter{
		SubcategoryID: &subID,
		Pagination:    shared.NewPagination(1, 20),
	}

	products := []domain.Product{fakeProduct()}
	repo.On("List", mock.Anything, mock.MatchedBy(func(f domain.ProductFilter) bool {
		return f.CategoryID != nil && *f.CategoryID == parentID
	})).Return(products, 1, nil)

	uc := usecase.NewListProducts(repo, catRepo, silentLogger())
	result, _, err := uc.Execute(context.Background(), filter)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	catRepo.AssertCalled(t, "GetByID", mock.Anything, subID)
}

func TestListProducts_PageSizeClamped(t *testing.T) {
	repo := &MockProductRepository{}
	catRepo := &MockCategoryRepository{}

	filter := domain.ProductFilter{
		Pagination: shared.NewPagination(1, 200), // over 100
	}

	repo.On("List", mock.Anything, mock.MatchedBy(func(f domain.ProductFilter) bool {
		return f.Pagination.PageSize <= 100
	})).Return([]domain.Product{}, 0, nil)

	uc := usecase.NewListProducts(repo, catRepo, silentLogger())
	_, _, err := uc.Execute(context.Background(), filter)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestListProducts_InStockOnly(t *testing.T) {
	repo := &MockProductRepository{}
	catRepo := &MockCategoryRepository{}

	filter := domain.ProductFilter{
		InStockOnly: true,
		Pagination:  shared.NewPagination(1, 20),
	}

	repo.On("List", mock.Anything, mock.MatchedBy(func(f domain.ProductFilter) bool {
		return f.InStockOnly
	})).Return([]domain.Product{}, 0, nil)

	uc := usecase.NewListProducts(repo, catRepo, silentLogger())
	_, _, err := uc.Execute(context.Background(), filter)

	require.NoError(t, err)
}

func TestListProducts_Pagination(t *testing.T) {
	repo := &MockProductRepository{}
	catRepo := &MockCategoryRepository{}

	pag := shared.NewPagination(2, 20)
	filter := domain.ProductFilter{Pagination: pag}

	assert.Equal(t, 20, pag.Offset())

	repo.On("List", mock.Anything, mock.Anything).Return([]domain.Product{}, 0, nil)

	uc := usecase.NewListProducts(repo, catRepo, silentLogger())
	_, _, err := uc.Execute(context.Background(), filter)

	require.NoError(t, err)
}
