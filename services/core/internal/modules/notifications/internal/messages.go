package internal

import "fmt"

// OrderStatus mirrors ordering lifecycle statuses used in notifications payloads.
type OrderStatus string

const (
	StatusCreated    OrderStatus = "created"
	StatusConfirmed  OrderStatus = "confirmed"
	StatusInDelivery OrderStatus = "in_delivery"
	StatusDelivered  OrderStatus = "delivered"
	StatusCancelled  OrderStatus = "cancelled"
)

var statusMessagesRU = map[OrderStatus]string{
	StatusCreated:    "Мы приняли ваш заказ",
	StatusConfirmed:  "Магазин подтвердил, скоро начнём собирать",
	StatusInDelivery: "Курьер уже в пути",
	StatusDelivered:  "Заказ доставлен",
	StatusCancelled:  "Заказ отменён",
}

// MessageForStatusRU returns a localized push alert message for order status.
func MessageForStatusRU(status OrderStatus) (string, error) {
	message, ok := statusMessagesRU[status]
	if !ok {
		return "", fmt.Errorf("notifications: unknown order status %q", status)
	}
	return message, nil
}
