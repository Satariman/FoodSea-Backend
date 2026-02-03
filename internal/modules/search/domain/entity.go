package domain

// SearchFilter представляет фильтры для поиска товаров
type SearchFilter struct {
	Query      string
	CategoryIDs []int64
	BrandIDs   []int64
	MinPrice   *float64
	MaxPrice   *float64
	Limit      int
	Offset     int
}

// SearchResult представляет результат поиска
type SearchResult struct {
	Products []*Product
	Total    int
}

// Product представляет товар (используется из модуля catalog)
type Product struct {
	ID          int64
	Name        string
	Description string
	CategoryID  int64
	BrandID     int64
	Price       float64
}

