package domain

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	StatusCreated    OrderStatus = "created"
	StatusConfirmed  OrderStatus = "confirmed"
	StatusInDelivery OrderStatus = "in_delivery"
	StatusDelivered  OrderStatus = "delivered"
	StatusCancelled  OrderStatus = "cancelled"
)

// validTransitions defines the FSM: which statuses a given status can transition to.
var validTransitions = map[OrderStatus][]OrderStatus{
	StatusCreated:    {StatusConfirmed, StatusCancelled},
	StatusConfirmed:  {StatusInDelivery, StatusCancelled},
	StatusInDelivery: {StatusDelivered},
	StatusDelivered:  {},
	StatusCancelled:  {},
}

// CanTransitionTo returns true if the FSM allows transitioning from s to next.
func (s OrderStatus) CanTransitionTo(next OrderStatus) bool {
	if s == next {
		return false
	}
	for _, allowed := range validTransitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// IsTerminal returns true for delivered and cancelled statuses.
func (s OrderStatus) IsTerminal() bool {
	return s == StatusDelivered || s == StatusCancelled
}

// String implements the Stringer interface.
func (s OrderStatus) String() string { return string(s) }
