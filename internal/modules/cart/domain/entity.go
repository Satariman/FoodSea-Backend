package domain

// CartItem представляет позицию в корзине
type CartItem struct {
	ID        int64
	ClientID  string
	ProductID int64
	Quantity  int
}

// Cart представляет корзину пользователя
type Cart struct {
	ClientID string
	Items    []*CartItem
}

