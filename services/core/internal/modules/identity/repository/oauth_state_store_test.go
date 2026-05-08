package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestOAuthStateStore_Consume_EmptyState(t *testing.T) {
	store := NewOAuthStateStore(nil, time.Minute)

	_, err := store.Consume(context.Background(), "")
	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
}
