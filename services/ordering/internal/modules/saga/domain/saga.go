package domain

import (
	"time"

	"github.com/google/uuid"
)

type SagaStatus string

const (
	SagaStatusPending      SagaStatus = "pending"
	SagaStatusCompleted    SagaStatus = "completed"
	SagaStatusCompensating SagaStatus = "compensating"
	SagaStatusFailed       SagaStatus = "failed"
)

// StartInput is the entry point for launching a new saga.
type StartInput struct {
	UserID               uuid.UUID
	OptimizationResultID uuid.UUID
}

// OrderItemSnapshot is an immutable snapshot of one cart line captured from optimization result.
type OrderItemSnapshot struct {
	ProductID    uuid.UUID `json:"product_id"`
	ProductName  string    `json:"product_name"`
	StoreID      uuid.UUID `json:"store_id"`
	StoreName    string    `json:"store_name"`
	Quantity     int16     `json:"quantity"`
	PriceKopecks int64     `json:"price_kopecks"`
}

// SagaPayload carries mutable saga data persisted in JSONB after each step.
type SagaPayload struct {
	OptimizationResultID uuid.UUID           `json:"optimization_result_id"`
	OrderID              *uuid.UUID          `json:"order_id,omitempty"`
	Items                []OrderItemSnapshot `json:"items"`
	TotalKopecks         int64               `json:"total_kopecks"`
	DeliveryKopecks      int64               `json:"delivery_kopecks"`
	FailureReason        string              `json:"failure_reason,omitempty"`
}

// SagaState is the aggregate tracking a distributed saga lifecycle.
type SagaState struct {
	ID          uuid.UUID
	OrderID     uuid.UUID // DB column; zero until step 2 completes
	UserID      uuid.UUID
	CurrentStep int8
	Status      SagaStatus
	Payload     SagaPayload
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
