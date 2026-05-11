package repository

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestOAuthStateStore_Consume_EmptyState(t *testing.T) {
	store := NewOAuthStateStore(nil, time.Minute)

	_, err := store.Consume(context.Background(), "")
	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
}

func TestOAuthStateKey_Deterministic(t *testing.T) {
	t.Parallel()

	key1 := oauthStateKey("state-1")
	key2 := oauthStateKey("state-1")
	key3 := oauthStateKey("state-2")

	assert.Equal(t, key1, key2)
	assert.NotEqual(t, key1, key3)
	assert.Contains(t, key1, "oauth:state:")
}

func TestRandomURLToken(t *testing.T) {
	t.Parallel()

	token1, err := randomURLToken(32)
	require.NoError(t, err)
	token2, err := randomURLToken(32)
	require.NoError(t, err)

	assert.NotEqual(t, token1, token2)
	raw, err := base64.RawURLEncoding.DecodeString(token1)
	require.NoError(t, err)
	assert.Len(t, raw, 32)
}

func TestOAuthStateStore_Create_RandomError(t *testing.T) {
	origRand := oauthStateRandRead
	oauthStateRandRead = func([]byte) (int, error) {
		return 0, errors.New("entropy unavailable")
	}
	t.Cleanup(func() {
		oauthStateRandRead = origRand
	})

	store := NewOAuthStateStore(nil, time.Minute)
	_, err := store.Create(context.Background(), sampleOAuthSession())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generating oauth state")
}

func TestOAuthStateStore_Create_MarshalError(t *testing.T) {
	origMarshal := oauthStateMarshal
	oauthStateMarshal = func(any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	t.Cleanup(func() {
		oauthStateMarshal = origMarshal
	})

	store := NewOAuthStateStore(nil, time.Minute)
	_, err := store.Create(context.Background(), sampleOAuthSession())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshaling oauth session")
}

func sampleOAuthSession() domain.OAuthSession {
	return domain.OAuthSession{
		Provider:   domain.OAuthProviderGoogle,
		RedirectTo: "/cb",
	}
}
