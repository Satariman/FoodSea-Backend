package errors

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrConflict      = errors.New("conflict")
)

// ValidationError carries field-level validation details.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}
