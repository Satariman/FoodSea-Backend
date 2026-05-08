//go:build integration

package repository_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/repository"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestOAuthIdentityRepo_Integration(t *testing.T) {
	client := startPostgres(t)
	ctx := context.Background()
	userRepo := repository.NewUserRepo(client)
	oauthRepo := repository.NewOAuthIdentityRepo(client)

	t.Run("create and get by provider user id", func(t *testing.T) {
		email := "oauth_repo_" + uuid.NewString()[:8] + "@example.com"
		u := &domain.User{ID: uuid.New(), Email: &email}
		require.NoError(t, userRepo.CreateOAuth(ctx, u))

		identity := &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u.ID,
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub_" + uuid.NewString(),
			Email:          &email,
		}
		require.NoError(t, oauthRepo.Create(ctx, identity))

		found, err := oauthRepo.GetByProviderUserID(ctx, domain.OAuthProviderGoogle, identity.ProviderUserID)
		require.NoError(t, err)
		assert.Equal(t, identity.ID, found.ID)
		assert.Equal(t, identity.UserID, found.UserID)
		assert.Equal(t, identity.Provider, found.Provider)
		assert.Equal(t, identity.ProviderUserID, found.ProviderUserID)
		require.NotNil(t, found.Email)
		assert.Equal(t, email, *found.Email)
	})

	t.Run("missing identity returns not found", func(t *testing.T) {
		_, err := oauthRepo.GetByProviderUserID(ctx, domain.OAuthProviderGoogle, "missing_"+uuid.NewString())
		assert.ErrorIs(t, err, sherrors.ErrNotFound)
	})

	t.Run("duplicate provider subject returns already exists", func(t *testing.T) {
		email1 := "dup_sub_1_" + uuid.NewString()[:8] + "@example.com"
		u1 := &domain.User{ID: uuid.New(), Email: &email1}
		require.NoError(t, userRepo.CreateOAuth(ctx, u1))

		email2 := "dup_sub_2_" + uuid.NewString()[:8] + "@example.com"
		u2 := &domain.User{ID: uuid.New(), Email: &email2}
		require.NoError(t, userRepo.CreateOAuth(ctx, u2))

		subject := "dup_sub_" + uuid.NewString()
		require.NoError(t, oauthRepo.Create(ctx, &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u1.ID,
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: subject,
		}))

		err := oauthRepo.Create(ctx, &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u2.ID,
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: subject,
		})
		assert.ErrorIs(t, err, sherrors.ErrAlreadyExists)
	})

	t.Run("duplicate same provider per user returns already exists", func(t *testing.T) {
		email := "dup_user_provider_" + uuid.NewString()[:8] + "@example.com"
		u := &domain.User{ID: uuid.New(), Email: &email}
		require.NoError(t, userRepo.CreateOAuth(ctx, u))

		require.NoError(t, oauthRepo.Create(ctx, &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u.ID,
			Provider:       domain.OAuthProviderApple,
			ProviderUserID: "apple_sub_1_" + uuid.NewString(),
		}))

		err := oauthRepo.Create(ctx, &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u.ID,
			Provider:       domain.OAuthProviderApple,
			ProviderUserID: "apple_sub_2_" + uuid.NewString(),
		})
		assert.ErrorIs(t, err, sherrors.ErrAlreadyExists)
	})

	t.Run("non-existent user id does not map to already exists", func(t *testing.T) {
		err := oauthRepo.Create(ctx, &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         uuid.New(),
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "fk_missing_user_" + uuid.NewString(),
		})
		require.Error(t, err)
		assert.NotErrorIs(t, err, sherrors.ErrAlreadyExists)
	})

	t.Run("parallel create same subject one wins", func(t *testing.T) {
		email1 := "race_sub_1_" + uuid.NewString()[:8] + "@example.com"
		u1 := &domain.User{ID: uuid.New(), Email: &email1}
		require.NoError(t, userRepo.CreateOAuth(ctx, u1))

		email2 := "race_sub_2_" + uuid.NewString()[:8] + "@example.com"
		u2 := &domain.User{ID: uuid.New(), Email: &email2}
		require.NoError(t, userRepo.CreateOAuth(ctx, u2))

		subject := "race_sub_" + uuid.NewString()
		identities := []*domain.OAuthIdentity{
			{
				ID:             uuid.New(),
				UserID:         u1.ID,
				Provider:       domain.OAuthProviderVK,
				ProviderUserID: subject,
			},
			{
				ID:             uuid.New(),
				UserID:         u2.ID,
				Provider:       domain.OAuthProviderVK,
				ProviderUserID: subject,
			},
		}

		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			results []error
		)
		for _, identity := range identities {
			wg.Add(1)
			go func(identity *domain.OAuthIdentity) {
				defer wg.Done()
				err := oauthRepo.Create(ctx, identity)
				mu.Lock()
				results = append(results, err)
				mu.Unlock()
			}(identity)
		}
		wg.Wait()

		var nils, alreadyExists int
		for _, err := range results {
			if err == nil {
				nils++
				continue
			}
			if assert.ErrorIs(t, err, sherrors.ErrAlreadyExists) {
				alreadyExists++
			}
		}

		assert.Equal(t, 1, nils, "exactly one create should succeed")
		assert.Equal(t, 1, alreadyExists, "exactly one create should fail with already exists")
	})
}
