package domain

import "context"

// ProductRepository определяет интерфейс для работы с товарами
type ProductRepository interface {
	GetByID(ctx context.Context, id int64) (*Product, error)
	GetByBarcode(ctx context.Context, barcode string) (*Product, error)
	List(ctx context.Context, limit, offset int) ([]*Product, error)
	Search(ctx context.Context, query string, limit, offset int) ([]*Product, error)
}

// CategoryRepository определяет интерфейс для работы с категориями
type CategoryRepository interface {
	GetAll(ctx context.Context) ([]*Category, error)
	GetByID(ctx context.Context, id int64) (*Category, error)
}

// BrandRepository определяет интерфейс для работы с брендами
type BrandRepository interface {
	GetAll(ctx context.Context) ([]*Brand, error)
	GetByID(ctx context.Context, id int64) (*Brand, error)
}

