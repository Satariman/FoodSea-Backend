package domain

import "context"

// CartRepository определяет интерфейс для работы с корзиной
type CartRepository interface {
	GetByClientID(ctx context.Context, clientID string) (*Cart, error)
	AddItem(ctx context.Context, clientID string, productID int64, quantity int) error
	UpdateItemQuantity(ctx context.Context, clientID string, itemID int64, quantity int) error
	RemoveItem(ctx context.Context, clientID string, itemID int64) error
	Clear(ctx context.Context, clientID string) error
}

