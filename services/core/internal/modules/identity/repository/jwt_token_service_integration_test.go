//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/core/internal/modules/identity/repository"
	sherrors "github.com/foodsea/core/internal/shared/errors"

	"github.com/google/uuid"
)

func startRedis(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestJWTTokenService_Integration(t *testing.T) {
	redisClient := startRedis(t)
	ctx := context.Background()

	newSvc := func() *repository.JWTTokenService {
		return repository.NewJWTTokenService("test-secret", 15*time.Minute, 30*24*time.Hour, redisClient)
	}

	t.Run("issue and validate access token", func(t *testing.T) {
		svc := newSvc()
		userID := uuid.New()

		pair, err := svc.IssuePair(ctx, userID)
		require.NoError(t, err)
		assert.NotEmpty(t, pair.Access)
		assert.NotEmpty(t, pair.Refresh)

		claims, err := svc.ValidateAccess(pair.Access)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
	})

	t.Run("rotate refresh — old token invalidated", func(t *testing.T) {
		svc := newSvc()
		userID := uuid.New()

		pair1, err := svc.IssuePair(ctx, userID)
		require.NoError(t, err)

		pair2, err := svc.RotateRefresh(ctx, pair1.Refresh)
		require.NoError(t, err)
		assert.NotEqual(t, pair1.Refresh, pair2.Refresh)
		assert.NotEqual(t, pair1.Access, pair2.Access)

		// old refresh must now be invalid
		_, err = svc.RotateRefresh(ctx, pair1.Refresh)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)

		// new refresh must still be valid
		pair3, err := svc.RotateRefresh(ctx, pair2.Refresh)
		require.NoError(t, err)
		assert.NotEmpty(t, pair3.Access)
	})

	t.Run("rotate with unknown token → 401", func(t *testing.T) {
		svc := newSvc()
		_, err := svc.RotateRefresh(ctx, "nonexistent-token")
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("rotate with empty token → 401", func(t *testing.T) {
		svc := newSvc()
		_, err := svc.RotateRefresh(ctx, "")
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("revoke invalidates all sessions", func(t *testing.T) {
		svc := newSvc()
		userID := uuid.New()

		pair1, err := svc.IssuePair(ctx, userID)
		require.NoError(t, err)
		pair2, err := svc.IssuePair(ctx, userID)
		require.NoError(t, err)

		require.NoError(t, svc.Revoke(ctx, userID))

		_, err = svc.RotateRefresh(ctx, pair1.Refresh)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
		_, err = svc.RotateRefresh(ctx, pair2.Refresh)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("validate expired access token → unauthorized", func(t *testing.T) {
		svc := repository.NewJWTTokenService("test-secret", -time.Second, 30*24*time.Hour, redisClient)
		userID := uuid.New()

		pair, err := svc.IssuePair(ctx, userID)
		require.NoError(t, err)

		_, err = svc.ValidateAccess(pair.Access)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("validate token with wrong secret → unauthorized", func(t *testing.T) {
		svc := newSvc()
		userID := uuid.New()
		pair, err := svc.IssuePair(ctx, userID)
		require.NoError(t, err)

		wrongSvc := repository.NewJWTTokenService("wrong-secret", 15*time.Minute, 30*24*time.Hour, redisClient)
		_, err = wrongSvc.ValidateAccess(pair.Access)
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})
}
