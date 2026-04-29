package errors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sherrors "github.com/foodsea/optimization/internal/shared/errors"
)

func TestSentinelErrors(t *testing.T) {
	errs := []error{
		sherrors.ErrNotFound,
		sherrors.ErrAlreadyExists,
		sherrors.ErrInvalidInput,
		sherrors.ErrUnauthorized,
		sherrors.ErrConflict,
	}
	for _, e := range errs {
		require.Error(t, e)
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := &sherrors.ValidationError{Field: "email", Message: "must be valid"}
	assert.Equal(t, "email: must be valid", ve.Error())
}

func TestValidationError_IsDistinct(t *testing.T) {
	ve := &sherrors.ValidationError{Field: "f", Message: "m"}
	assert.False(t, ve == nil)
	assert.NotErrorIs(t, ve, sherrors.ErrInvalidInput)
}
