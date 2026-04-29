package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/foodsea/optimization/internal/shared/domain"
)

func TestResultStatusValues(t *testing.T) {
	assert.Equal(t, "active", string(domain.StatusActive))
	assert.Equal(t, "locked", string(domain.StatusLocked))
	assert.Equal(t, "expired", string(domain.StatusExpired))
}
