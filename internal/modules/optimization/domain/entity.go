package domain

// OptimizationResult представляет результат оптимизации корзины
type OptimizationResult struct {
	ID              int64
	ClientID       string
	TotalCost      float64
	OriginalCost   float64
	Savings        float64
	Distribution   []*StoreDistribution
	Substitutions  []*Substitution
}

// StoreDistribution представляет распределение товаров по магазину
type StoreDistribution struct {
	PartnerID    int64
	PartnerName  string
	Items        []*DistributionItem
	Subtotal     float64
	DeliveryCost float64
	Total        float64
}

// DistributionItem представляет товар в распределении
type DistributionItem struct {
	ProductID int64
	ProductName string
	Quantity    int
	Price       float64
	Total       float64
}

// Substitution представляет предложение по замене товара
type Substitution struct {
	OriginalProductID   int64
	OriginalProductName string
	SuggestedProductID  int64
	SuggestedProductName string
	PriceDifference     float64
	Savings             float64
	SimilarityScore     float64
}

