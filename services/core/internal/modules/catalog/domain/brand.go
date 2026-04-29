package domain

import "github.com/google/uuid"

// Brand represents a product brand.
type Brand struct {
	ID   uuid.UUID
	Name string
}
