package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type User struct {
	ID             uuid.UUID
	Phone          *string
	Email          *string
	OnboardingDone bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Credentials struct {
	Phone    *string
	Email    *string
	Password string
}

func (c Credentials) Validate() error {
	hasPhone := c.Phone != nil && *c.Phone != ""
	hasEmail := c.Email != nil && *c.Email != ""

	if hasPhone && hasEmail {
		return fmt.Errorf("%w: provide either phone or email, not both", sherrors.ErrInvalidInput)
	}
	if !hasPhone && !hasEmail {
		return fmt.Errorf("%w: phone or email is required", sherrors.ErrInvalidInput)
	}
	if len(c.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", sherrors.ErrInvalidInput)
	}
	return nil
}
