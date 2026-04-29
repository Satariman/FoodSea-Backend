package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from   OrderStatus
		to     OrderStatus
		expect bool
	}{
		{StatusCreated, StatusConfirmed, true},
		{StatusCreated, StatusCancelled, true},
		{StatusCreated, StatusDelivered, false},
		{StatusCreated, StatusInDelivery, false},
		{StatusConfirmed, StatusInDelivery, true},
		{StatusConfirmed, StatusCancelled, true},
		{StatusConfirmed, StatusDelivered, false},
		{StatusConfirmed, StatusCreated, false},
		{StatusInDelivery, StatusDelivered, true},
		{StatusInDelivery, StatusCancelled, false},
		{StatusInDelivery, StatusConfirmed, false},
		{StatusDelivered, StatusCancelled, false},
		{StatusDelivered, StatusCreated, false},
		{StatusDelivered, StatusConfirmed, false},
		{StatusDelivered, StatusInDelivery, false},
		{StatusCancelled, StatusCreated, false},
		{StatusCancelled, StatusConfirmed, false},
		{StatusCancelled, StatusInDelivery, false},
		{StatusCancelled, StatusDelivered, false},
		// transition to self is always false
		{StatusCreated, StatusCreated, false},
		{StatusConfirmed, StatusConfirmed, false},
		{StatusInDelivery, StatusInDelivery, false},
		{StatusDelivered, StatusDelivered, false},
		{StatusCancelled, StatusCancelled, false},
	}

	for _, tt := range tests {
		name := string(tt.from) + " -> " + string(tt.to)
		t.Run(name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestOrderStatus_IsTerminal(t *testing.T) {
	assert.True(t, StatusDelivered.IsTerminal())
	assert.True(t, StatusCancelled.IsTerminal())
	assert.False(t, StatusCreated.IsTerminal())
	assert.False(t, StatusConfirmed.IsTerminal())
	assert.False(t, StatusInDelivery.IsTerminal())
}

func TestOrderStatus_String(t *testing.T) {
	assert.Equal(t, "created", StatusCreated.String())
	assert.Equal(t, "in_delivery", StatusInDelivery.String())
}
