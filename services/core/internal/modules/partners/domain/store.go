package domain

import "github.com/google/uuid"

type Store struct {
	ID       uuid.UUID
	Name     string
	Slug     string
	LogoURL  *string
	IsActive bool
}
