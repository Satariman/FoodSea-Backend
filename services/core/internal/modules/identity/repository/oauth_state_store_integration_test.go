//go:build integration

package repository_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	t.Run("consume malformed payload", func(t *testing.T) {
		state := "malformed-state"
		key := oauthStateKeyForTest(state)
		require.NoError(t, redisClient.Set(ctx, key, "{", time.Minute).Err())

		_, err := store.Consume(ctx, state)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshaling oauth session")
	})

	t.Run("consume expired payload unauthorized", func(t *testing.T) {
		state := "expired-state"
		key := oauthStateKeyForTest(state)
		payload, err := json.Marshal(domain.OAuthSession{
			State:      state,
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/api/v1/auth/oauth/callback",
			CreatedAt:  time.Now().Add(-2 * time.Hour),
			ExpiresAt:  time.Now().Add(-time.Minute),
		})
		require.NoError(t, err)
		require.NoError(t, redisClient.Set(ctx, key, payload, time.Minute).Err())

		_, err = store.Consume(ctx, state)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("create with closed redis client returns error", func(t *testing.T) {
		closedClient := startRedis(t)
		closedStore := repository.NewOAuthStateStore(closedClient, time.Minute)
		require.NoError(t, closedClient.Close())

		_, err := closedStore.Create(ctx, domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/api/v1/auth/oauth/callback",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "storing oauth state")
	})

	t.Run("consume with closed redis client returns error", func(t *testing.T) {
		closedClient := startRedis(t)
		closedStore := repository.NewOAuthStateStore(closedClient, time.Minute)
		require.NoError(t, closedClient.Close())

		_, err := closedStore.Consume(ctx, "state-after-close")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "consuming oauth state")
	})
}

func oauthStateKeyForTest(state string) string {
	hash := sha256.Sum256([]byte(state))
	return "oauth:state:" + hex.EncodeToString(hash[:])
}
