package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/foodsea/ordering/internal/shared/domain"
)

// Order is the aggregate root for the orders module.
type Order struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	OptimizationResultID *uuid.UUID
	TotalKopecks         int64
	DeliveryKopecks      int64
	Status               domain.OrderStatus
	Items                []OrderItem
	History              []StatusChange
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// OrderItem carries immutable snapshot data captured at order creation time.
type OrderItem struct {
	ID           uuid.UUID
	ProductID    uuid.UUID
	ProductName  string
	StoreID      uuid.UUID
	StoreName    string
	Quantity     int16
	PriceKopecks int64
}

// StatusChange records a single FSM transition in the order history.
type StatusChange struct {
	Status    domain.OrderStatus
	Comment   *string
	ChangedAt time.Time
}
