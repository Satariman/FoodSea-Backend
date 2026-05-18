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
		return usecase.NewOAuthCallback(states, []domain.OAuthProvider{provider}, identities, users, tokens)
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
		provider.On("Exchange", ctx, "code-1", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
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

		got, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s1", Code: "code-1", RedirectURI: "/cb"})
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
		provider.On("Exchange", ctx, "code-2", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
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

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s2", Code: "code-2", RedirectURI: "/cb"})
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
		provider.On("Exchange", ctx, "code-3", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
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

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s3", Code: "code-3", RedirectURI: "/cb"})
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
		provider.On("Exchange", ctx, "code-4", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-4",
			Email:          nil,
			EmailVerified:  false,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-4").Return(nil, sherrors.ErrNotFound).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s4", Code: "code-4", RedirectURI: "/cb"})
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

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "reused", Code: "code", RedirectURI: "/cb"})
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

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s5", Code: "code", RedirectURI: "/cb"})
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
		provider.On("Exchange", ctx, "bad-code", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{}, sherrors.ErrUnauthorized).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s6", Code: "bad-code", RedirectURI: "/cb"})
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("callback malformed input invalid input", func(t *testing.T) {
		uc := newUC(new(MockOAuthStateStore), new(MockOAuthProvider), new(MockOAuthIdentityRepository), new(MockUserRepository), new(MockTokenService))
		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: "", State: "", Code: "", RedirectURI: "/cb"})
		assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
	})

	t.Run("callback unsupported provider invalid input", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
			Provider:    domain.OAuthProviderVK,
			State:       "state",
			Code:        "code",
			RedirectURI: "/cb",
		})
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
		provider.On("Exchange", ctx, "code-7", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-7",
			Email:          ptr("x@example.com"),
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-7").Return(nil, errors.New("db down")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s7", Code: "code-7", RedirectURI: "/cb"})
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
		provider.On("Exchange", ctx, "code-8", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
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

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s8", Code: "code-8", RedirectURI: "/cb"})
		require.NoError(t, err)
	})

	t.Run("callback state store unexpected error passthrough", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		states.On("Consume", ctx, "s9").Return(domain.OAuthSession{}, errors.New("redis down")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s9", Code: "code", RedirectURI: "/cb"})
		require.Error(t, err)
		assert.Equal(t, "redis down", err.Error())
	})

	t.Run("callback state provider mismatch unauthorized", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		states.On("Consume", ctx, "s10").Return(domain.OAuthSession{
			Provider:   domain.OAuthProviderYandex,
			RedirectTo: "/cb",
		}, nil).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s10", Code: "code", RedirectURI: "/cb"})
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("callback mode mismatch unauthorized", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		states.On("Consume", ctx, "s10m").Return(domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
			Mode:       domain.OAuthFlowModeNative,
		}, nil).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
			Provider:    domain.OAuthProviderGoogle,
			State:       "s10m",
			Code:        "code",
			RedirectURI: "/cb",
			Mode:        domain.OAuthFlowModeLegacy,
		})
		assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	})

	t.Run("callback provider exchange unexpected error passthrough", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		states.On("Consume", ctx, "s11").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-11", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{}, errors.New("provider timeout")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s11", Code: "code-11", RedirectURI: "/cb"})
		require.Error(t, err)
		assert.Equal(t, "provider timeout", err.Error())
	})

	t.Run("callback existing identity but user lookup fails", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		userID := uuid.New()
		states.On("Consume", ctx, "s12").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-12", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-12",
			Email:          ptr("u12@example.com"),
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-12").Return(&domain.OAuthIdentity{
			UserID: userID,
		}, nil).Once()
		users.On("GetByID", ctx, userID).Return(nil, errors.New("user lookup failed")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s12", Code: "code-12", RedirectURI: "/cb"})
		require.Error(t, err)
		assert.Equal(t, "user lookup failed", err.Error())
	})

	t.Run("callback user search by email unexpected error", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		email := "user13@example.com"
		states.On("Consume", ctx, "s13").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-13", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-13",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-13").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(nil, errors.New("db down")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s13", Code: "code-13", RedirectURI: "/cb"})
		require.Error(t, err)
		assert.Equal(t, "db down", err.Error())
	})

	t.Run("callback create oauth user fails", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		email := "user14@example.com"
		states.On("Consume", ctx, "s14").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-14", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-14",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-14").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(nil, sherrors.ErrNotFound).Once()
		users.On("CreateOAuth", ctx, mock.AnythingOfType("*domain.User")).Return(errors.New("insert failed")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s14", Code: "code-14", RedirectURI: "/cb"})
		require.Error(t, err)
		assert.Equal(t, "insert failed", err.Error())
	})

	t.Run("callback create identity unexpected error", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		email := "user15@example.com"
		localUser := &domain.User{ID: uuid.New(), Email: &email}
		states.On("Consume", ctx, "s15").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-15", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-15",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-15").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(localUser, nil).Once()
		identities.On("Create", ctx, mock.AnythingOfType("*domain.OAuthIdentity")).Return(errors.New("identity create failed")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s15", Code: "code-15", RedirectURI: "/cb"})
		require.Error(t, err)
		assert.Equal(t, "identity create failed", err.Error())
	})

	t.Run("callback race fallback get linked identity fails", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		email := "race-get-fail@example.com"
		localUser := &domain.User{ID: uuid.New(), Email: &email}
		states.On("Consume", ctx, "s16").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-16", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-16",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-16").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(localUser, nil).Once()
		identities.On("Create", ctx, mock.AnythingOfType("*domain.OAuthIdentity")).Return(sherrors.ErrAlreadyExists).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-16").Return(nil, errors.New("identity lookup failed")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s16", Code: "code-16", RedirectURI: "/cb"})
		require.Error(t, err)
		assert.Equal(t, "identity lookup failed", err.Error())
	})

	t.Run("callback race fallback user lookup fails", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		email := "race-user-fail@example.com"
		localUser := &domain.User{ID: uuid.New(), Email: &email}
		linkedIdentity := &domain.OAuthIdentity{ID: uuid.New(), UserID: uuid.New(), Provider: domain.OAuthProviderGoogle, ProviderUserID: "sub-17"}
		states.On("Consume", ctx, "s17").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-17", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-17",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-17").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(localUser, nil).Once()
		identities.On("Create", ctx, mock.AnythingOfType("*domain.OAuthIdentity")).Return(sherrors.ErrAlreadyExists).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-17").Return(linkedIdentity, nil).Once()
		users.On("GetByID", ctx, linkedIdentity.UserID).Return(nil, errors.New("linked user fetch failed")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s17", Code: "code-17", RedirectURI: "/cb"})
		require.Error(t, err)
		assert.Equal(t, "linked user fetch failed", err.Error())
	})

	t.Run("callback token issue fails", func(t *testing.T) {
		states := new(MockOAuthStateStore)
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(states, provider, identities, users, tokens)

		userID := uuid.New()
		u := &domain.User{ID: userID, Email: ptr("u18@example.com")}
		states.On("Consume", ctx, "s18").Return(domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectTo: "/cb"}, nil).Once()
		provider.On("Exchange", ctx, "code-18", domain.OAuthSession{
			Provider:   domain.OAuthProviderGoogle,
			RedirectTo: "/cb",
		}).Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "sub-18",
			Email:          ptr("u18@example.com"),
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "sub-18").Return(&domain.OAuthIdentity{
			UserID: userID,
		}, nil).Once()
		users.On("GetByID", ctx, userID).Return(u, nil).Once()
		tokens.On("IssuePair", ctx, userID).Return(domain.TokenPair{}, errors.New("token service down")).Once()

		_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle, State: "s18", Code: "code-18", RedirectURI: "/cb"})
		require.Error(t, err)
		assert.Equal(t, "token service down", err.Error())
	})
}

func TestOAuthCallback_ExecuteToken(t *testing.T) {
	ctx := context.Background()

	newUC := func(
		provider *MockOAuthProvider,
		identities *MockOAuthIdentityRepository,
		users *MockUserRepository,
		tokens *MockTokenService,
	) *usecase.OAuthCallback {
		provider.On("Name").Return(domain.OAuthProviderYandex).Once()
		return usecase.NewOAuthCallback(new(MockOAuthStateStore), []domain.OAuthProvider{provider}, identities, users, tokens)
	}

	t.Run("token callback success", func(t *testing.T) {
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(provider, identities, users, tokens)

		userID := uuid.New()
		email := "sdk@example.com"
		u := &domain.User{ID: userID, Email: &email}
		pair := fakePair()

		provider.On("ProfileFromToken", ctx, "sdk-token").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderYandex,
			ProviderUserID: "yandex-id-1",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderYandex, "yandex-id-1").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(u, nil).Once()
		identities.On("Create", ctx, mock.AnythingOfType("*domain.OAuthIdentity")).Return(nil).Once()
		tokens.On("IssuePair", ctx, userID).Return(pair, nil).Once()

		result, err := uc.ExecuteToken(ctx, domain.OAuthTokenCallbackRequest{
			Provider:    domain.OAuthProviderYandex,
			AccessToken: "sdk-token",
		})
		require.NoError(t, err)
		assert.Equal(t, pair.Access, result.TokenPair.Access)
		assert.Equal(t, userID, result.User.ID)
	})

	t.Run("token callback malformed input", func(t *testing.T) {
		provider := new(MockOAuthProvider)
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := newUC(provider, identities, users, tokens)

		_, err := uc.ExecuteToken(ctx, domain.OAuthTokenCallbackRequest{})
		assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
	})

	t.Run("apple token callback existing identity success", func(t *testing.T) {
		provider := new(MockOAuthProvider)
		provider.On("Name").Return(domain.OAuthProviderApple).Once()
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := usecase.NewOAuthCallback(new(MockOAuthStateStore), []domain.OAuthProvider{provider}, identities, users, tokens)

		userID := uuid.New()
		u := &domain.User{ID: userID, Email: ptr("apple@foodsea.test")}
		pair := fakePair()
		provider.On("ProfileFromToken", ctx, "apple-token").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderApple,
			ProviderUserID: "apple-sub-1",
			Email:          ptr("apple@foodsea.test"),
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderApple, "apple-sub-1").Return(&domain.OAuthIdentity{
			UserID: userID,
		}, nil).Once()
		users.On("GetByID", ctx, userID).Return(u, nil).Once()
		tokens.On("IssuePair", ctx, userID).Return(pair, nil).Once()

		_, err := uc.ExecuteToken(ctx, domain.OAuthTokenCallbackRequest{
			Provider:    domain.OAuthProviderApple,
			AccessToken: "apple-token",
			Email:       ptr("ignored@foodsea.test"),
		})
		require.NoError(t, err)
	})

	t.Run("apple token callback email conflict returns conflict", func(t *testing.T) {
		provider := new(MockOAuthProvider)
		provider.On("Name").Return(domain.OAuthProviderApple).Once()
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := usecase.NewOAuthCallback(new(MockOAuthStateStore), []domain.OAuthProvider{provider}, identities, users, tokens)

		email := "existing@foodsea.test"
		existingUser := &domain.User{ID: uuid.New(), Email: &email}
		provider.On("ProfileFromToken", ctx, "apple-token").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderApple,
			ProviderUserID: "apple-sub-2",
			Email:          &email,
			EmailVerified:  true,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderApple, "apple-sub-2").Return(nil, sherrors.ErrNotFound).Once()
		users.On("GetByEmail", ctx, email).Return(existingUser, nil).Once()

		_, err := uc.ExecuteToken(ctx, domain.OAuthTokenCallbackRequest{
			Provider:    domain.OAuthProviderApple,
			AccessToken: "apple-token",
		})
		assert.ErrorIs(t, err, sherrors.ErrConflict)
	})

	t.Run("apple token callback creates user when email absent", func(t *testing.T) {
		provider := new(MockOAuthProvider)
		provider.On("Name").Return(domain.OAuthProviderApple).Once()
		identities := new(MockOAuthIdentityRepository)
		users := new(MockUserRepository)
		tokens := new(MockTokenService)
		uc := usecase.NewOAuthCallback(new(MockOAuthStateStore), []domain.OAuthProvider{provider}, identities, users, tokens)

		pair := fakePair()
		provider.On("ProfileFromToken", ctx, "apple-token").Return(domain.OAuthProviderProfile{
			Provider:       domain.OAuthProviderApple,
			ProviderUserID: "apple-sub-3",
			Email:          nil,
			EmailVerified:  false,
		}, nil).Once()
		identities.On("GetByProviderUserID", ctx, domain.OAuthProviderApple, "apple-sub-3").Return(nil, sherrors.ErrNotFound).Once()
		users.On("CreateOAuth", ctx, mock.AnythingOfType("*domain.User")).Return(nil).Once()
		identities.On("Create", ctx, mock.AnythingOfType("*domain.OAuthIdentity")).Return(nil).Once()
		tokens.On("IssuePair", ctx, mock.AnythingOfType("uuid.UUID")).Return(pair, nil).Once()

		_, err := uc.ExecuteToken(ctx, domain.OAuthTokenCallbackRequest{
			Provider:    domain.OAuthProviderApple,
			AccessToken: "apple-token",
		})
		require.NoError(t, err)
	})
}
