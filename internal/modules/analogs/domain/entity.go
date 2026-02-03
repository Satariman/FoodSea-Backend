package domain

// Analog представляет аналог товара
type Analog struct {
	ProductID      int64
	ProductName    string
	SimilarityScore float64
	Price          float64
	IsCheaper      bool
	PriceDifference float64
}

