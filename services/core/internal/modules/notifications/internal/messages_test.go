package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageForStatusRU(t *testing.T) {
	testCases := []struct {
		name    string
		status  OrderStatus
		message string
	}{
		{name: "created", status: StatusCreated, message: "Мы приняли ваш заказ"},
		{name: "confirmed", status: StatusConfirmed, message: "Магазин подтвердил, скоро начнём собирать"},
		{name: "in_delivery", status: StatusInDelivery, message: "Курьер уже в пути"},
		{name: "delivered", status: StatusDelivered, message: "Заказ доставлен"},
		{name: "cancelled", status: StatusCancelled, message: "Заказ отменён"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			message, err := MessageForStatusRU(tc.status)
			require.NoError(t, err)
			assert.Equal(t, tc.message, message)
		})
	}
}

func TestMessageForStatusRU_UnknownStatus(t *testing.T) {
	_, err := MessageForStatusRU(OrderStatus("unknown"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown order status")
}
