package domain

// Offer представляет предложение магазина для товара
type Offer struct {
	ID           int64
	ProductID    int64
	PartnerID    int64
	Price        float64
	IsAvailable  bool
	HasDiscount  bool
	DiscountRate float64
	DeliveryCost float64
	FreeDeliveryThreshold float64
}

// Partner представляет магазин-партнер
type Partner struct {
	ID   int64
	Name string
	URL  string
}

