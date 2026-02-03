package domain

import "context"

// AnalogRepository определяет интерфейс для работы с аналогами
type AnalogRepository interface {
	FindAnalogs(ctx context.Context, productID int64, limit int) ([]*Analog, error)
}

// MLGateway определяет интерфейс для взаимодействия с ML-модулем
type MLGateway interface {
	FindSimilarProducts(ctx context.Context, productID int64, limit int) ([]*Analog, error)
}

