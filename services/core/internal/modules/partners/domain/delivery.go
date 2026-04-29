package domain

import "github.com/google/uuid"

type DeliveryCondition struct {
	StoreID             uuid.UUID
	MinOrderKopecks     int64
	DeliveryCostKopecks int64
	FreeFromKopecks     *int64
	EstimatedMinutes    *int32
}
