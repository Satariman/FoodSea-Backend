package domain

import (
	"time"

	"github.com/google/uuid"
)

type Cart struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Items     []CartItem
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CartItem struct {
	ID          uuid.UUID
	ProductID   uuid.UUID
	ProductName string
	Quantity    int16
	AddedAt     time.Time
}
