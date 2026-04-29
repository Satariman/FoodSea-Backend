//go:build integration

package repository_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/repository"
	"github.com/foodsea/core/internal/platform/config"
	"github.com/foodsea/core/internal/platform/database"
	sherrors "github.com/foodsea/core/internal/shared/errors"

	_ "github.com/foodsea/core/ent/runtime"
)

func startPostgres(t *testing.T) *ent.Client {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("identity_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	dbCfg := config.DatabaseConfig{
		URL:          connStr,
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	client, _, err := database.Open(ctx, dbCfg, slog.Default())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Schema.Create(ctx))
	return client
}

func TestUserRepo_Integration(t *testing.T) {
	client := startPostgres(t)
	ctx := context.Background()
	repo := repository.NewUserRepo(client)

	t.Run("create and get by email", func(t *testing.T) {
		email := "user_" + uuid.NewString()[:8] + "@example.com"
		u := &domain.User{
			ID:    uuid.New(),
			Email: &email,
		}
		require.NoError(t, repo.Create(ctx, u, "hash123"))

		found, err := repo.GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, u.ID, found.ID)
		assert.Equal(t, email, *found.Email)
		assert.False(t, found.OnboardingDone)
	})

	t.Run("create and get by phone", func(t *testing.T) {
		phone := "+7900" + uuid.NewString()[:7]
		u := &domain.User{
			ID:    uuid.New(),
			Phone: &phone,
		}
		require.NoError(t, repo.Create(ctx, u, "hash456"))

		found, err := repo.GetByPhone(ctx, phone)
		require.NoError(t, err)
		assert.Equal(t, u.ID, found.ID)
	})

	t.Run("get by id", func(t *testing.T) {
		email := "byid_" + uuid.NewString()[:8] + "@example.com"
		u := &domain.User{ID: uuid.New(), Email: &email}
		require.NoError(t, repo.Create(ctx, u, "hash"))

		found, err := repo.GetByID(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, u.ID, found.ID)
	})

	t.Run("get by id not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, sherrors.ErrNotFound)
	})

	t.Run("get password hash", func(t *testing.T) {
		email := "pw_" + uuid.NewString()[:8] + "@example.com"
		u := &domain.User{ID: uuid.New(), Email: &email}
		require.NoError(t, repo.Create(ctx, u, "secrethash"))

		hash, err := repo.GetPasswordHash(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, "secrethash", hash)
	})

	t.Run("set onboarding done — idempotent", func(t *testing.T) {
		email := "onb_" + uuid.NewString()[:8] + "@example.com"
		u := &domain.User{ID: uuid.New(), Email: &email}
		require.NoError(t, repo.Create(ctx, u, "hash"))

		require.NoError(t, repo.SetOnboardingDone(ctx, u.ID))
		require.NoError(t, repo.SetOnboardingDone(ctx, u.ID)) // idempotent

		found, err := repo.GetByID(ctx, u.ID)
		require.NoError(t, err)
		assert.True(t, found.OnboardingDone)
	})

	t.Run("duplicate email returns already exists", func(t *testing.T) {
		email := "dup_" + uuid.NewString()[:8] + "@example.com"
		u1 := &domain.User{ID: uuid.New(), Email: &email}
		u2 := &domain.User{ID: uuid.New(), Email: &email}

		require.NoError(t, repo.Create(ctx, u1, "hash"))
		err := repo.Create(ctx, u2, "hash")
		assert.ErrorIs(t, err, sherrors.ErrAlreadyExists)
	})

	t.Run("parallel register same email — one wins", func(t *testing.T) {
		email := "race_" + uuid.NewString()[:8] + "@example.com"
		var (
			mu      sync.Mutex
			results []error
			wg      sync.WaitGroup
		)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				u := &domain.User{ID: uuid.New(), Email: &email}
				err := repo.Create(ctx, u, "hash")
				mu.Lock()
				results = append(results, err)
				mu.Unlock()
			}()
		}
		wg.Wait()

		var errs, nils int
		for _, err := range results {
			if err == nil {
				nils++
			} else {
				errs++
			}
		}
		assert.Equal(t, 1, nils, "exactly one registration should succeed")
		assert.Equal(t, 1, errs, "exactly one registration should fail")
	})
}
