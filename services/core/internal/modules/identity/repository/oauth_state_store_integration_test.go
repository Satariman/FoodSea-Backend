//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/repository"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestOAuthStateStore_Integration(t *testing.T) {
	redisClient := startRedis(t)
	ctx := context.Background()
	store := repository.NewOAuthStateStore(redisClient, time.Minute)

	session := domain.OAuthSession{
		Provider:   domain.OAuthProviderGoogle,
		RedirectTo: "/api/v1/auth/oauth/callback",
	}

	state, err := store.Create(ctx, session)
	require.NoError(t, err)
	require.NotEmpty(t, state)

	consumed, err := store.Consume(ctx, state)
	require.NoError(t, err)
	assert.Equal(t, domain.OAuthProviderGoogle, consumed.Provider)
	assert.Equal(t, "/api/v1/auth/oauth/callback", consumed.RedirectTo)
	assert.Equal(t, state, consumed.State)

	_, err = store.Consume(ctx, state)
	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
}
