package domain

// BarcodeSearchResult представляет результат поиска по штрихкоду
type BarcodeSearchResult struct {
	Product *Product
	Found   bool
}

// Product представляет товар (используется из модуля catalog)
type Product struct {
	ID          int64
	Name        string
	Description string
	Barcode     string
}

