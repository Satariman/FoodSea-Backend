package domain

// Partner представляет магазин-партнер
type Partner struct {
	ID                      int64
	Name                    string
	URL                     string
	DeliveryCost            float64
	FreeDeliveryThreshold   float64
	IsActive                bool
}

// PartnerAdapter определяет интерфейс для адаптера интеграции с партнером
type PartnerAdapter interface {
	CreateOrder(ctx context.Context, orderData interface{}) (string, error)
	GetOrderStatus(ctx context.Context, orderID string) (string, error)
	UpdateProducts(ctx context.Context) error
}

