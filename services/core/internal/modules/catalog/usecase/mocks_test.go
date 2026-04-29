package usecase_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/foodsea/core/internal/modules/catalog/domain"
)

// MockCategoryRepository mocks domain.CategoryRepository.
type MockCategoryRepository struct{ mock.Mock }

func (m *MockCategoryRepository) ListAll(ctx context.Context) ([]domain.Category, error) {
	args := m.Called(ctx)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].([]domain.Category)
	return v, nil
}

func (m *MockCategoryRepository) GetBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	args := m.Called(ctx, slug)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.Category)
	return v, nil
}

func (m *MockCategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	args := m.Called(ctx, id)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.Category)
	return v, nil
}

// MockBrandRepository mocks domain.BrandRepository.
type MockBrandRepository struct{ mock.Mock }

func (m *MockBrandRepository) ListAll(ctx context.Context) ([]domain.Brand, error) {
	args := m.Called(ctx)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].([]domain.Brand)
	return v, nil
}

func (m *MockBrandRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Brand, error) {
	args := m.Called(ctx, id)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.Brand)
	return v, nil
}

// MockProductRepository mocks domain.ProductRepository.
type MockProductRepository struct{ mock.Mock }

func (m *MockProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	args := m.Called(ctx, id)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.Product)
	return v, nil
}

func (m *MockProductRepository) GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*domain.ProductDetail, error) {
	args := m.Called(ctx, id)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.ProductDetail)
	return v, nil
}

func (m *MockProductRepository) GetByBarcode(ctx context.Context, barcode string) (*domain.ProductDetail, error) {
	args := m.Called(ctx, barcode)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.ProductDetail)
	return v, nil
}

func (m *MockProductRepository) ListAllForML(ctx context.Context) ([]domain.ProductMLData, error) {
	args := m.Called(ctx)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].([]domain.ProductMLData)
	return v, nil
}

func (m *MockProductRepository) List(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int, error) {
	args := m.Called(ctx, filter)
	var err error
	if len(args) > 2 && args[2] != nil {
		err, _ = args[2].(error)
	}
	if err != nil {
		return nil, args.Int(1), err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, args.Int(1), nil
	}
	v, _ := args[0].([]domain.Product)
	return v, args.Int(1), nil
}

// MockProductCache mocks domain.ProductCache.
type MockProductCache struct{ mock.Mock }

func (m *MockProductCache) GetProduct(ctx context.Context, id uuid.UUID) (*domain.ProductDetail, error) {
	args := m.Called(ctx, id)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.ProductDetail)
	return v, nil
}

func (m *MockProductCache) SetProduct(ctx context.Context, product *domain.ProductDetail) error {
	args := m.Called(ctx, product)
	return args.Error(0)
}

func (m *MockProductCache) GetCategoriesTree(ctx context.Context) ([]domain.Category, error) {
	args := m.Called(ctx)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].([]domain.Category)
	return v, nil
}

func (m *MockProductCache) SetCategoriesTree(ctx context.Context, tree []domain.Category) error {
	args := m.Called(ctx, tree)
	return args.Error(0)
}

func (m *MockProductCache) Invalidate(ctx context.Context, productID uuid.UUID) error {
	args := m.Called(ctx, productID)
	return args.Error(0)
}

// MockBestOfferProvider mocks domain.BestOfferProvider.
type MockBestOfferProvider struct{ mock.Mock }

func (m *MockBestOfferProvider) GetBestOffer(ctx context.Context, productID uuid.UUID) (*domain.BestOffer, error) {
	args := m.Called(ctx, productID)
	var err error
	if len(args) > 1 && args[1] != nil {
		err, _ = args[1].(error)
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	v, _ := args[0].(*domain.BestOffer)
	return v, nil
}

// helpers

func fakeCategory(name string) domain.Category {
	return domain.Category{ID: uuid.New(), Name: name, Slug: name}
}

func fakeProduct() domain.Product {
	return domain.Product{
		ID:         uuid.New(),
		Name:       "Test Product",
		CategoryID: uuid.New(),
		InStock:    true,
	}
}

func fakeProductDetail() *domain.ProductDetail {
	p := fakeProduct()
	cat := fakeCategory("Beverages")
	return &domain.ProductDetail{
		Product:  p,
		Category: cat,
	}
}
