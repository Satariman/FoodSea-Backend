package domain

import "github.com/google/uuid"

// Category represents a product category (root or subcategory).
type Category struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	ParentID  *uuid.UUID
	SortOrder int
	Children  []Category // populated by ListCategories tree build
}
