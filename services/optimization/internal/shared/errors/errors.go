package errors

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrValidation   = errors.New("validation error")
	ErrInternal     = errors.New("internal error")
	ErrUnauthorized = errors.New("unauthorized")

	// Backward-compatible aliases used by shared platform helpers.
	ErrAlreadyExists = ErrConflict
	ErrInvalidInput  = ErrValidation
)

// ValidationError carries field-level validation details.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}
