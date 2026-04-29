package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type (
	ProductID = uuid.UUID
	StoreID   = uuid.UUID
)

// CartItem is a user cart item consumed by optimizer.
type CartItem struct {
	ProductID   ProductID
	ProductName string
	Quantity    int
}

// DeliveryCondition defines store delivery rules.
type DeliveryCondition struct {
	MinOrderKopecks     int64
	DeliveryCostKopecks int64
	FreeFromKopecks     *int64
}

// Analog is a semantic substitute candidate for a product.
type Analog struct {
	ProductID   ProductID
	ProductName string
	Score       float64
}

// Input is the full optimization snapshot.
type Input struct {
	UserID     uuid.UUID
	Items      []CartItem
	Stores     []StoreID
	StoreNames map[StoreID]string
	Prices     map[ProductID]map[StoreID]int64
	Delivery   map[StoreID]DeliveryCondition
	Analogs    map[ProductID][]Analog
}

// Assignment is the selected store for one product.
type Assignment struct {
	ProductID   ProductID
	ProductName string
	StoreID     StoreID
	StoreName   string
	Price       int64
	Quantity    int
}

// Substitution is one optional analog replacement suggestion.
type Substitution struct {
	OriginalID           ProductID
	OriginalProductName  string
	AnalogID             ProductID
	AnalogProductName    string
	OriginalStoreID      StoreID
	NewStoreID           StoreID
	NewStoreName         string
	OldPriceKopecks      int64
	NewPriceKopecks      int64
	PriceDeltaKopecks    int64
	DeliveryDeltaKopecks int64
	TotalSavingKopecks   int64
	Score                float64
	IsCrossStore         bool
}

// Result is an in-memory optimization output from algorithm.
type Result struct {
	Assignments     []Assignment
	Substitutions   []Substitution
	TotalKopecks    int64
	DeliveryKopecks int64
	SavingsKopecks  int64
	IsApproximate   bool
}

// OptimizationResult is a persisted optimization snapshot.
type OptimizationResult struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	CartHash        string
	TotalKopecks    int64
	DeliveryKopecks int64
	SavingsKopecks  int64
	Status          string
	IsApproximate   bool
	Items           []Assignment
	Substitutions   []Substitution
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

var (
	ErrEmptyCart          = errors.New("cart is empty")
	ErrNoOffers           = errors.New("no offers available for cart items")
	ErrDeliveryIncomplete = errors.New("delivery conditions are incomplete for optimization stores")
	ErrNoFeasibleSolution = errors.New("no feasible solution found")
	ErrResultNotFound     = errors.New("optimization result not found")
	ErrResultLocked       = errors.New("optimization result is already locked")
	ErrResultNotActive    = errors.New("optimization result is not active")
	ErrResultNotLocked    = errors.New("optimization result is not locked")
)
