package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestOAuthCallback_Execute(t *testing.T) {
	ctx := context.Background()

	newUC := func(
		states *MockOAuthStateStore,
		provider *MockOAuthProvider,
		identities *MockOAuthIdentityRepository,
		users *MockUserRepository,
		tokens *MockTokenService,
	) *usecase.OAuthCallback {
		provider.On("Name").Return(domain.OAuthProviderGoogle).Once()
		return usecase.NewOAuthCallback(states, []usecase.OAuthProvider{provider}, identities, users, tokens)
	}

	t.Run("callback existing identity success", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)

		uc := newUC(states, provider, identities, users, tokens)
		userID := uuid.New()
		u := &domain.User{ID: userID, Email: ptr("u@example.com")}
		pair := fakePair()

		states.On("Consume", ctx, "s1").Return(domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}, nil).Once()
		provider.On("Exchange", ctx, "code-1").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-1",
			Email:          ptr("u@example.com"),
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-1").Return(&domain.OAuthIdentity{
			ID:     uuid.New(),
			UserID: userID,
		}, nil).Once()
		users.On("GetByID", ctx, userID).Return(u, nil).Once()
		tokens.On("IssuePair", ctx, userID).Return(pair, nil).Once()

		got, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s1", Code: "code-1"})
		require.NoError(t, err)
		assert.Equal(t, userID, got.User.ID)
		assert.Equal(t, pair.Access, got.TokenPair.Access)
	})

	t.Run("callback verified email links existing user", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)

		uc := newUC(states, provider, identities, users, tokens)
		userID := uuid.New()
		email := "existing@example.com"
		u := &domain.User{ID: userID, Email: &email}

		states.On("Consume", ctx, "s2").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-2").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-2",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-2").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(u, nil).Once()
		identities.On("Create", ctx, mock.MatchedBy(func(i *domain.OAuthIdentity) bool {
			return i.UserID == userID && i.ProviderUserID == "sub-2"
		})).Return(nil).Once()
		tokens.On("IssuePair", ctx, userID).Return(fakePair(), nil).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s2", Code: "code-2"})
		require.NoError(t, err)
	})

	t.Run("callback new oauth user", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)

		uc := newUC(states, provider, identities, users, tokens)
		email := "new@example.com"
		createdUser := &domain.User{ID: uuid.New(), Email: &email}

		states.On("Consume", ctx, "s3").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-3").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-3",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-3").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(nil, sherrors.ErrNotFound).Once()
		users.On("CreateOAuth", ctx, mock.AnythingOfType("*domain.User")).Run(func(args mock.Arguments) {
			argUser := args.Get(1).(*domain.User)
			createdUser.ID = argUser.ID
		}).Return(nil).Once()
		identities.On("Create", ctx, mock.MatchedBy(func(i *domain.OAuthIdentity) bool {
			return i.UserID == createdUser.ID && i.ProviderUserID == "sub-3"
		})).Return(nil).Once()
		tokens.On("IssuePair", ctx, mock.AnythingOfType("uuid.UUID")).Return(fakePair(), nil).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s3", Code: "code-3"})
		require.NoError(t, err)
	})

	t.Run("callback missing email conflict", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)

		uc := newUC(states, provider, identities, users, tokens)

		states.On("Consume", ctx, "s4").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-4").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-4",
			Email:          nil,
			EmailVerified:  false,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-4").Return(nil, sherrors.ErrNotFound).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s4", Code: "code-4"})
		assert.ErrorIs(t, err, sherrors.ErrConflict)
	})

	t.Run("callback reused state unauthorized", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)

		uc := newUC(states, provider, identities, users, tokens)
		states.On("Consume", ctx, "reused").Return(domain.OAuthSession{}, sherrors.ErrUnauthorized).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "reused", Code: "code"})
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("callback redirect mismatch unauthorized", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)

		uc := newUC(states, provider, identities, users, tokens)
		states.On("Consume", ctx, "s5").Return(domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "",
		}, nil).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s5", Code: "code"})
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("callback provider exchange unauthorized", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)

		uc := newUC(states, provider, identities, users, tokens)
		states.On("Consume", ctx, "s6").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "bad-code").Return(domain.OAuthProviderProfile{}, sherrors.ErrUnauthorized).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s6", Code: "bad-code"})
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("callback malformed input invalid input", func(t *testing.T) {
		uc := newUC(new(MockOAuthStateStore), new(MockOAuthProvider), new(MockOAuthIdentityRepository), new(MockUserRepository), new(MockTokenService))
		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: "", State: "", Code: ""})
		assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
	})

	t.Run("callback provider repo error passthrough", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)

		uc := newUC(states, provider, identities, users, tokens)
		states.On("Consume", ctx, "s7").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-7").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-7",
			Email:          ptr("x@example.com"),
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-7").Return(nil, errors.New("db down")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s7", Code: "code-7"})
		require.Error(t, err)
		assert.Equal(t, "db down", err.Error())
	})

	t.Run("callback race on identity create fallback path", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)

		uc := newUC(states, provider, identities, users, tokens)
		email := "race@example.com"
		userID := uuid.New()
		localUser := &domain.User{ID: userID, Email: &email}
		linkedIdentity := &domain.OAuthIdentity{ID: uuid.New(), UserID: userID, Provider: domain.OAuthProviderGoogle, ProviderUserID: "sub-race"}

		states.On("Consume", ctx, "s8").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-8").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-race",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-race").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(localUser, nil).Once()
		identities.On("Create", ctx, mock.AnythingOfType("*domain.OAuthIdentity")).Return(sherrors.ErrAlreadyExists).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-race").Return(linkedIdentity, nil).Once()
		users.On("GetByID", ctx, userID).Return(localUser, nil).Once()
		tokens.On("IssuePair", ctx, userID).Return(fakePair(), nil).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s8", Code: "code-8"})
		require.NoError(t, err)
	})
}
