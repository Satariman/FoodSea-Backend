package usecase

import (
	"context"
	"foodsea-backend/internal/modules/catalog/domain"
)

type CatalogUseCase struct {
	productRepo  domain.ProductRepository
	categoryRepo domain.CategoryRepository
	brandRepo    domain.BrandRepository
}

func NewCatalogUseCase(
	productRepo domain.ProductRepository,
	categoryRepo domain.CategoryRepository,
	brandRepo domain.BrandRepository,
) *CatalogUseCase {
	return &CatalogUseCase{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		brandRepo:    brandRepo,
	}
}

func (uc *CatalogUseCase) GetProductByID(ctx context.Context, id int64) (*domain.Product, error) {
	return uc.productRepo.GetByID(ctx, id)
}

func (uc *CatalogUseCase) GetProductByBarcode(ctx context.Context, barcode string) (*domain.Product, error) {
	return uc.productRepo.GetByBarcode(ctx, barcode)
}

func (uc *CatalogUseCase) ListProducts(ctx context.Context, limit, offset int) ([]*domain.Product, error) {
	return uc.productRepo.List(ctx, limit, offset)
}

func (uc *CatalogUseCase) SearchProducts(ctx context.Context, query string, limit, offset int) ([]*domain.Product, error) {
	return uc.productRepo.Search(ctx, query, limit, offset)
}

func (uc *CatalogUseCase) GetAllCategories(ctx context.Context) ([]*domain.Category, error) {
	return uc.categoryRepo.GetAll(ctx)
}

func (uc *CatalogUseCase) GetAllBrands(ctx context.Context) ([]*domain.Brand, error) {
	return uc.brandRepo.GetAll(ctx)
}

