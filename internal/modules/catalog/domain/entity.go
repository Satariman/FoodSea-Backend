package domain

// Product представляет товар в каталоге
type Product struct {
	ID          int64
	Name        string
	Description string
	CategoryID  int64
	BrandID     int64
	Barcode     string
	ImageURL    string
}

// Category представляет категорию товаров
type Category struct {
	ID   int64
	Name string
}

// Brand представляет бренд/производителя
type Brand struct {
	ID   int64
	Name string
}

