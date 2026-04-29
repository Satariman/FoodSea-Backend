package domain

import (
	"context"

	"github.com/google/uuid"
)

// CartParticipant abstracts core-service cart operations used by the saga.
type CartParticipant interface {
	ClearCart(ctx context.Context, userID uuid.UUID) error
	RestoreCart(ctx context.Context, userID uuid.UUID, items []OrderItemSnapshot) error
}

// OptimizationResult is the saga-domain view of an optimization run.
type OptimizationResult struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	TotalKopecks    int64
	DeliveryKopecks int64
	Status          string
	Items           []OrderItemSnapshot
}

// OptimizationParticipant abstracts optimization-service operations used by the saga.
type OptimizationParticipant interface {
	LockResult(ctx context.Context, resultID uuid.UUID) error
	UnlockResult(ctx context.Context, resultID uuid.UUID) error
	GetResult(ctx context.Context, resultID uuid.UUID) (*OptimizationResult, error)
}

// CreatePendingInput carries the data needed to create a pending order in the orders module.
type CreatePendingInput struct {
	UserID               uuid.UUID
	OptimizationResultID *uuid.UUID
	Items                []OrderItemSnapshot
	TotalKopecks         int64
	DeliveryKopecks      int64
}

// Order is a minimal order reference returned by OrdersParticipant.CreatePending.
type Order struct {
	ID uuid.UUID
}

// OrdersParticipant abstracts the orders module facade used by the saga.
type OrdersParticipant interface {
	CreatePending(ctx context.Context, input CreatePendingInput) (*Order, error)
	Confirm(ctx context.Context, orderID uuid.UUID) error
	Cancel(ctx context.Context, orderID uuid.UUID, reason string) error
}

// SagaAuditPublisher writes command/reply events to Kafka for audit and monitoring.
type SagaAuditPublisher interface {
	PublishCommand(ctx context.Context, sagaID uuid.UUID, step int8, cmdType string, payload any) error
	PublishReply(ctx context.Context, sagaID uuid.UUID, step int8, status string, payload any) error
}
