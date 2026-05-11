# OAuth Registration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Google and Yandex ID OAuth registration/login to `core-service` through backend-owned authorization-code flow, with FoodSea JWT/refresh sessions and 100% coverage for new OAuth code.

**Architecture:** Keep OAuth inside `services/core/internal/modules/identity`. Add provider adapters, Redis-backed OAuth state storage, a separate `oauth_identities` table, and two use cases: start and callback. The callback resolves an existing provider identity, auto-links by verified email, or creates an OAuth-only local user.

**Tech Stack:** Go 1.25, Gin, Ent, Atlas migrations, Redis, `github.com/golang-jwt/jwt/v5`, `github.com/stretchr/testify`, testcontainers, `httptest.Server`.

---

## File Structure

Create:

- `services/core/ent/schema/oauth_identity.go` - Ent schema for provider account links.
- `services/core/internal/modules/identity/domain/oauth.go` - OAuth value objects and interfaces.
- `services/core/internal/modules/identity/usecase/oauth_start.go` - start authorization flow.
- `services/core/internal/modules/identity/usecase/oauth_callback.go` - complete authorization flow and issue FoodSea tokens.
- `services/core/internal/modules/identity/repository/oauth_identity_repo.go` - Ent repository for OAuth identity links and OAuth user creation.
- `services/core/internal/modules/identity/repository/oauth_state_store.go` - Redis state store.
- `services/core/internal/modules/identity/repository/oauth_provider_google.go` - Google OIDC adapter.
- `services/core/internal/modules/identity/repository/oauth_provider_yandex.go` - Yandex ID adapter.
- `services/core/internal/modules/identity/repository/oauth_provider_http.go` - small shared HTTP helpers for provider adapters.
- `services/core/internal/modules/identity/usecase/oauth_start_test.go`
- `services/core/internal/modules/identity/usecase/oauth_callback_test.go`
- `services/core/internal/modules/identity/repository/oauth_state_store_test.go`
- `services/core/internal/modules/identity/repository/oauth_provider_google_test.go`
- `services/core/internal/modules/identity/repository/oauth_provider_yandex_test.go`
- `services/core/internal/modules/identity/repository/oauth_identity_repo_integration_test.go`
- `services/core/test/e2e/oauth_test.go`
- one new Atlas migration under `services/core/migrations/`.

Modify:

- `services/core/ent/schema/user.go` - make `password_hash` optional/nillable and add inverse OAuth edge.
- generated Ent files under `services/core/ent/` after `go generate ./ent`.
- `services/core/internal/platform/config/config.go` - OAuth config.
- `services/core/internal/platform/config/config_test.go` - OAuth config coverage.
- `services/core/internal/modules/identity/domain/repository.go` - repository interfaces.
- `services/core/internal/modules/identity/domain/user.go` - represent optional password hash behavior through repository return type.
- `services/core/internal/modules/identity/usecase/login.go` - reject password login for OAuth-only users.
- `services/core/internal/modules/identity/usecase/mocks_test.go` - mock expanded repository interface.
- `services/core/internal/modules/identity/repository/user_repo.go` - nillable password hash handling.
- `services/core/internal/modules/identity/repository/user_repo_integration_test.go` - OAuth-only user regression.
- `services/core/internal/modules/identity/handler/dto.go` - OAuth request/response DTOs.
- `services/core/internal/modules/identity/handler/auth_handler.go` - start/callback handlers and swagger annotations.
- `services/core/internal/modules/identity/handler/auth_handler_test.go` - handler coverage.
- `services/core/internal/modules/identity/module.go` - DI and route registration.
- `services/core/cmd/api/main.go` - pass OAuth config into identity module.
- `services/core/internal/modules/identity/architecture-notes.md` - document OAuth decisions.
- `services/core/cmd/api/architecture-notes.md` - document new public routes.
- `services/core/docs/swagger/*` and `docs/api/core-swagger.*` - regenerate swagger.

---

## Task 1: OAuth Config

**Files:**
- Modify: `services/core/internal/platform/config/config.go`
- Modify: `services/core/internal/platform/config/config_test.go`

- [ ] **Step 1: Add failing config tests**

Append these tests to `services/core/internal/platform/config/config_test.go`:

```go
func TestLoad_OAuthDefaults(t *testing.T) {
	unsetenv(t, "OAUTH_STATE_TTL")
	unsetenv(t, "OAUTH_ALLOWED_REDIRECT_URIS")
	unsetenv(t, "GOOGLE_OAUTH_CLIENT_ID")
	unsetenv(t, "GOOGLE_OAUTH_CLIENT_SECRET")
	unsetenv(t, "YANDEX_OAUTH_CLIENT_ID")
	unsetenv(t, "YANDEX_OAUTH_CLIENT_SECRET")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 10*time.Minute, cfg.OAuth.StateTTL)
	assert.Empty(t, cfg.OAuth.AllowedRedirectURIs)
	assert.False(t, cfg.OAuth.Google.Enabled)
	assert.False(t, cfg.OAuth.Yandex.Enabled)
}

func TestLoad_OAuthCustomValues(t *testing.T) {
	setenv(t, "OAUTH_STATE_TTL", "7m")
	setenv(t, "OAUTH_ALLOWED_REDIRECT_URIS", "foodsea://oauth/google/callback, foodsea://oauth/yandex/callback")
	setenv(t, "GOOGLE_OAUTH_CLIENT_ID", "google-client")
	setenv(t, "GOOGLE_OAUTH_CLIENT_SECRET", "google-secret")
	setenv(t, "YANDEX_OAUTH_CLIENT_ID", "yandex-client")
	setenv(t, "YANDEX_OAUTH_CLIENT_SECRET", "yandex-secret")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, 7*time.Minute, cfg.OAuth.StateTTL)
	assert.Equal(t, []string{
		"foodsea://oauth/google/callback",
		"foodsea://oauth/yandex/callback",
	}, cfg.OAuth.AllowedRedirectURIs)
	assert.True(t, cfg.OAuth.Google.Enabled)
	assert.Equal(t, "google-client", cfg.OAuth.Google.ClientID)
	assert.Equal(t, "google-secret", cfg.OAuth.Google.ClientSecret)
	assert.True(t, cfg.OAuth.Yandex.Enabled)
	assert.Equal(t, "yandex-client", cfg.OAuth.Yandex.ClientID)
	assert.Equal(t, "yandex-secret", cfg.OAuth.Yandex.ClientSecret)
}

func TestLoad_ProdOAuthPartialCredentials(t *testing.T) {
	setenv(t, "ENV", "production")
	setenv(t, "JWT_SECRET", "supersecret")
	setenv(t, "GOOGLE_OAUTH_CLIENT_ID", "google-client")
	unsetenv(t, "GOOGLE_OAUTH_CLIENT_SECRET")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GOOGLE_OAUTH_CLIENT_SECRET")
}

func TestLoad_ProdOAuthWithoutRedirectURIs(t *testing.T) {
	setenv(t, "ENV", "production")
	setenv(t, "JWT_SECRET", "supersecret")
	setenv(t, "GOOGLE_OAUTH_CLIENT_ID", "google-client")
	setenv(t, "GOOGLE_OAUTH_CLIENT_SECRET", "google-secret")
	unsetenv(t, "OAUTH_ALLOWED_REDIRECT_URIS")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OAUTH_ALLOWED_REDIRECT_URIS")
}
```

- [ ] **Step 2: Run config tests and confirm they fail**

Run:

```bash
cd services/core && go test ./internal/platform/config -run 'OAuth|ProdOAuth' -count=1
```

Expected: compile failure because `cfg.OAuth` does not exist.

- [ ] **Step 3: Add OAuth config types and loading**

In `services/core/internal/platform/config/config.go`, add `OAuth OAuthConfig` to `Config` and add:

```go
type OAuthConfig struct {
	StateTTL            time.Duration
	AllowedRedirectURIs []string
	Google              OAuthProviderConfig
	Yandex              OAuthProviderConfig
}

type OAuthProviderConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	JWKSURL      string
}
```

In `Load`, after JWT validation, load:

```go
oauthStateTTL := getEnvDuration("OAUTH_STATE_TTL", 10*time.Minute)
if oauthStateTTL < time.Minute {
	return nil, fmt.Errorf("OAUTH_STATE_TTL must be >= 1m, got %s", oauthStateTTL)
}

oauthAllowedRedirectURIs := getEnvStrings("OAUTH_ALLOWED_REDIRECT_URIS", nil)
googleOAuth := oauthProviderConfig(
	"GOOGLE",
	"https://accounts.google.com/o/oauth2/v2/auth",
	"https://oauth2.googleapis.com/token",
	"",
	"https://www.googleapis.com/oauth2/v3/certs",
)
yandexOAuth := oauthProviderConfig(
	"YANDEX",
	"https://oauth.yandex.ru/authorize",
	"https://oauth.yandex.ru/token",
	"https://login.yandex.ru/info",
	"",
)

if env == "production" {
	if err := validateOAuthProvider("GOOGLE", googleOAuth); err != nil {
		return nil, err
	}
	if err := validateOAuthProvider("YANDEX", yandexOAuth); err != nil {
		return nil, err
	}
	if (googleOAuth.Enabled || yandexOAuth.Enabled) && len(oauthAllowedRedirectURIs) == 0 {
		return nil, fmt.Errorf("OAUTH_ALLOWED_REDIRECT_URIS must be set when OAuth is enabled in production")
	}
}
```

Set the field in `cfg`:

```go
OAuth: OAuthConfig{
	StateTTL:            oauthStateTTL,
	AllowedRedirectURIs: oauthAllowedRedirectURIs,
	Google:              googleOAuth,
	Yandex:              yandexOAuth,
},
```

Add helpers near the bottom:

```go
func oauthProviderConfig(prefix, defaultAuthURL, defaultTokenURL, defaultUserInfoURL, defaultJWKSURL string) OAuthProviderConfig {
	clientID := getEnv(prefix+"_OAUTH_CLIENT_ID", "")
	clientSecret := getEnv(prefix+"_OAUTH_CLIENT_SECRET", "")
	return OAuthProviderConfig{
		Enabled:      clientID != "" && clientSecret != "",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      getEnv(prefix+"_OAUTH_AUTH_URL", defaultAuthURL),
		TokenURL:     getEnv(prefix+"_OAUTH_TOKEN_URL", defaultTokenURL),
		UserInfoURL:  getEnv(prefix+"_OAUTH_USERINFO_URL", defaultUserInfoURL),
		JWKSURL:      getEnv(prefix+"_OAUTH_JWKS_URL", defaultJWKSURL),
	}
}

func validateOAuthProvider(prefix string, cfg OAuthProviderConfig) error {
	clientID := os.Getenv(prefix + "_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv(prefix + "_OAUTH_CLIENT_SECRET")
	if clientID != "" && clientSecret == "" {
		return fmt.Errorf("%s_OAUTH_CLIENT_SECRET must be set when %s_OAUTH_CLIENT_ID is set", prefix, prefix)
	}
	if clientSecret != "" && clientID == "" {
		return fmt.Errorf("%s_OAUTH_CLIENT_ID must be set when %s_OAUTH_CLIENT_SECRET is set", prefix, prefix)
	}
	if cfg.Enabled && cfg.AuthURL == "" {
		return fmt.Errorf("%s_OAUTH_AUTH_URL must be set", prefix)
	}
	if cfg.Enabled && cfg.TokenURL == "" {
		return fmt.Errorf("%s_OAUTH_TOKEN_URL must be set", prefix)
	}
	return nil
}
```

- [ ] **Step 4: Run config tests and commit**

Run:

```bash
cd services/core && go test ./internal/platform/config -count=1
```

Expected: PASS.

Commit:

```bash
git add services/core/internal/platform/config/config.go services/core/internal/platform/config/config_test.go
git commit -m "feat(core): add oauth configuration"
```

---

## Task 2: Domain Contracts

**Files:**
- Create: `services/core/internal/modules/identity/domain/oauth.go`
- Modify: `services/core/internal/modules/identity/domain/repository.go`
- Modify: `services/core/internal/modules/identity/domain/user.go`
- Modify: `services/core/internal/modules/identity/usecase/mocks_test.go`

- [ ] **Step 1: Add OAuth domain contracts**

Create `services/core/internal/modules/identity/domain/oauth.go`:

```go
package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type OAuthProviderName string

const (
	OAuthProviderGoogle OAuthProviderName = "google"
	OAuthProviderYandex OAuthProviderName = "yandex"
)

func ParseOAuthProviderName(value string) (OAuthProviderName, bool) {
	switch OAuthProviderName(value) {
	case OAuthProviderGoogle, OAuthProviderYandex:
		return OAuthProviderName(value), true
	default:
		return "", false
	}
}

type OAuthSession struct {
	Provider     OAuthProviderName `json:"provider"`
	RedirectURI  string            `json:"redirect_uri"`
	Nonce        string            `json:"nonce,omitempty"`
	CodeVerifier string            `json:"code_verifier,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at"`
}

type OAuthStartRequest struct {
	Provider    OAuthProviderName
	RedirectURI string
}

type OAuthStartResult struct {
	AuthURL string
	State   string
}

type OAuthCallbackRequest struct {
	Provider    OAuthProviderName
	Code        string
	State       string
	RedirectURI string
}

type OAuthProviderProfile struct {
	Provider       OAuthProviderName
	ProviderUserID string
	Email          *string
	EmailVerified  bool
}

type OAuthIdentity struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       OAuthProviderName
	ProviderUserID string
	Email          *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OAuthStateStore interface {
	Create(ctx context.Context, session OAuthSession) (string, error)
	Consume(ctx context.Context, state string) (OAuthSession, error)
}

type OAuthProvider interface {
	Name() OAuthProviderName
	AuthURL(ctx context.Context, state string, session OAuthSession) (string, error)
	Exchange(ctx context.Context, code string, session OAuthSession) (OAuthProviderProfile, error)
}

type OAuthIdentityRepository interface {
	GetByProviderUserID(ctx context.Context, provider OAuthProviderName, providerUserID string) (*OAuthIdentity, error)
	Create(ctx context.Context, identity *OAuthIdentity) error
}
```

- [ ] **Step 2: Modify user repository interface**

In `services/core/internal/modules/identity/domain/repository.go`, change the interface to include optional password hash and OAuth user creation:

```go
type UserRepository interface {
	Create(ctx context.Context, u *User, passwordHash string) error
	CreateOAuth(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByPhone(ctx context.Context, phone string) (*User, error)
	GetPasswordHash(ctx context.Context, id uuid.UUID) (*string, error)
	SetOnboardingDone(ctx context.Context, id uuid.UUID) error
}
```

- [ ] **Step 3: Update mocks to compile against new interface**

In `services/core/internal/modules/identity/usecase/mocks_test.go`, change `GetPasswordHash`:

```go
func (m *MockUserRepository) GetPasswordHash(ctx context.Context, id uuid.UUID) (*string, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	value := args.String(0)
	return &value, args.Error(1)
}
```

Add `CreateOAuth`:

```go
func (m *MockUserRepository) CreateOAuth(ctx context.Context, u *domain.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}
```

- [ ] **Step 4: Update login tests for pointer password hash**

In `services/core/internal/modules/identity/usecase/login_test.go`, replace expectations like:

```go
repo.On("GetPasswordHash", ctx, u.ID).Return("$hashed", nil)
```

with:

```go
repo.On("GetPasswordHash", ctx, u.ID).Return("$hashed", nil)
```

The mock implementation converts the returned string to `*string`. Add one new test:

```go
t.Run("oauth-only user without password hash → 401", func(t *testing.T) {
	repo := &MockUserRepository{}
	hasher := &MockPasswordHasher{}
	u := fakeUser()
	email := *u.Email

	repo.On("GetByEmail", ctx, email).Return(u, nil)
	repo.On("GetPasswordHash", ctx, u.ID).Return(nil, nil)

	uc := usecase.NewLogin(repo, hasher, &MockTokenService{})
	_, err := uc.Execute(ctx, domain.Credentials{Email: &email, Password: "password1"})

	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
	hasher.AssertNotCalled(t, "Verify", mock.Anything, mock.Anything)
})
```

- [ ] **Step 5: Run usecase tests and confirm failure**

Run:

```bash
cd services/core && go test ./internal/modules/identity/usecase -run Login -count=1
```

Expected: failure in `login.go` because it still treats password hash as `string`.

- [ ] **Step 6: Update login implementation**

In `services/core/internal/modules/identity/usecase/login.go`, replace:

```go
hash, err := l.users.GetPasswordHash(ctx, u.ID)
if err != nil {
	return LoginResult{}, err
}

if err := l.hasher.Verify(hash, creds.Password); err != nil {
	return LoginResult{}, sherrors.ErrUnauthorized
}
```

with:

```go
hash, err := l.users.GetPasswordHash(ctx, u.ID)
if err != nil {
	return LoginResult{}, err
}
if hash == nil || *hash == "" {
	return LoginResult{}, sherrors.ErrUnauthorized
}

if err := l.hasher.Verify(*hash, creds.Password); err != nil {
	return LoginResult{}, sherrors.ErrUnauthorized
}
```

- [ ] **Step 7: Run domain/usecase tests and commit**

Run:

```bash
cd services/core && go test ./internal/modules/identity/domain ./internal/modules/identity/usecase -count=1
```

Expected: PASS.

Commit:

```bash
git add services/core/internal/modules/identity/domain services/core/internal/modules/identity/usecase
git commit -m "feat(core): add oauth domain contracts"
```

---

## Task 3: Ent Schema and User Repository

**Files:**
- Create: `services/core/ent/schema/oauth_identity.go`
- Modify: `services/core/ent/schema/user.go`
- Modify: `services/core/internal/modules/identity/repository/user_repo.go`
- Modify: `services/core/internal/modules/identity/repository/user_repo_integration_test.go`
- Generated: `services/core/ent/**`

- [ ] **Step 1: Add repository integration tests**

Append to `TestUserRepo_Integration` in `services/core/internal/modules/identity/repository/user_repo_integration_test.go`:

```go
t.Run("create oauth-only user without password hash", func(t *testing.T) {
	email := "oauth_" + uuid.NewString()[:8] + "@example.com"
	u := &domain.User{ID: uuid.New(), Email: &email}

	require.NoError(t, repo.CreateOAuth(ctx, u))

	found, err := repo.GetByEmail(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, u.ID, found.ID)

	hash, err := repo.GetPasswordHash(ctx, u.ID)
	require.NoError(t, err)
	assert.Nil(t, hash)
})
```

- [ ] **Step 2: Run repository integration test and confirm failure**

Run:

```bash
cd services/core && go test -tags integration ./internal/modules/identity/repository -run 'TestUserRepo_Integration/create oauth-only user' -count=1
```

Expected: compile failure because `CreateOAuth` is not implemented.

- [ ] **Step 3: Add OAuth identity Ent schema**

Create `services/core/ent/schema/oauth_identity.go`:

```go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/foodsea/core/ent/schema/mixin"
)

type OAuthIdentity struct {
	ent.Schema
}

func (OAuthIdentity) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Timestamps{}}
}

func (OAuthIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("provider").NotEmpty(),
		field.String("provider_user_id").NotEmpty(),
		field.String("email").Optional().Nillable(),
		field.UUID("user_id", uuid.UUID{}),
	}
}

func (OAuthIdentity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("oauth_identities").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (OAuthIdentity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider", "provider_user_id").Unique(),
		index.Fields("provider", "user_id").Unique(),
	}
}
```

- [ ] **Step 4: Update user schema**

In `services/core/ent/schema/user.go`, change:

```go
field.String("password_hash").NotEmpty(),
```

to:

```go
field.String("password_hash").Optional().Nillable(),
```

Add the OAuth edge:

```go
edge.To("oauth_identities", OAuthIdentity.Type),
```

inside `Edges()`.

- [ ] **Step 5: Generate Ent code**

Run:

```bash
cd services/core && go generate ./ent
```

Expected: generated Ent files include `oauthidentity` package and `OAuthIdentity` client APIs.

- [ ] **Step 6: Update user repository**

In `services/core/internal/modules/identity/repository/user_repo.go`, implement:

```go
func (r *UserRepo) CreateOAuth(ctx context.Context, u *domain.User) error {
	created, err := r.client.User.Create().
		SetID(u.ID).
		SetOnboardingDone(u.OnboardingDone).
		SetNillableEmail(u.Email).
		SetNillablePhone(u.Phone).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return sherrors.ErrAlreadyExists
		}
		return fmt.Errorf("creating oauth user: %w", err)
	}

	u.CreatedAt = created.CreatedAt
	u.UpdatedAt = created.UpdatedAt
	return nil
}
```

Change `GetPasswordHash` signature and return:

```go
func (r *UserRepo) GetPasswordHash(ctx context.Context, id uuid.UUID) (*string, error) {
	u, err := r.client.User.Query().
		Where(user.ID(id)).
		Select(user.FieldPasswordHash).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting password hash: %w", err)
	}
	return u.PasswordHash, nil
}
```

- [ ] **Step 7: Run repository tests and commit**

Run:

```bash
cd services/core && go test -tags integration ./internal/modules/identity/repository -count=1
```

Expected: PASS.

Commit:

```bash
git add services/core/ent services/core/internal/modules/identity/repository/user_repo.go services/core/internal/modules/identity/repository/user_repo_integration_test.go
git commit -m "feat(core): add oauth identity schema"
```

---

## Task 4: OAuth Identity Repository

**Files:**
- Create: `services/core/internal/modules/identity/repository/oauth_identity_repo.go`
- Create: `services/core/internal/modules/identity/repository/oauth_identity_repo_integration_test.go`

- [ ] **Step 1: Write integration tests**

Create `services/core/internal/modules/identity/repository/oauth_identity_repo_integration_test.go`:

```go
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

	createUser := func(t *testing.T, email string) *domain.User {
		t.Helper()
		u := &domain.User{ID: uuid.New(), Email: &email}
		require.NoError(t, userRepo.CreateOAuth(ctx, u))
		return u
	}

	t.Run("create and get by provider user id", func(t *testing.T) {
		u := createUser(t, "oauth_identity_"+uuid.NewString()[:8]+"@example.com")
		email := *u.Email
		identity := &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u.ID,
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "google-sub-1",
			Email:          &email,
		}

		require.NoError(t, oauthRepo.Create(ctx, identity))

		found, err := oauthRepo.GetByProviderUserID(ctx, domain.OAuthProviderGoogle, "google-sub-1")
		require.NoError(t, err)
		assert.Equal(t, u.ID, found.UserID)
		assert.Equal(t, "google-sub-1", found.ProviderUserID)
		assert.Equal(t, email, *found.Email)
	})

	t.Run("missing identity returns not found", func(t *testing.T) {
		_, err := oauthRepo.GetByProviderUserID(ctx, domain.OAuthProviderYandex, "missing")
		assert.ErrorIs(t, err, sherrors.ErrNotFound)
	})

	t.Run("duplicate provider subject returns already exists", func(t *testing.T) {
		u1 := createUser(t, "dup1_"+uuid.NewString()[:8]+"@example.com")
		u2 := createUser(t, "dup2_"+uuid.NewString()[:8]+"@example.com")
		require.NoError(t, oauthRepo.Create(ctx, &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u1.ID,
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "same-subject",
		}))

		err := oauthRepo.Create(ctx, &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u2.ID,
			Provider:       domain.OAuthProviderGoogle,
			ProviderUserID: "same-subject",
		})
		assert.ErrorIs(t, err, sherrors.ErrAlreadyExists)
	})

	t.Run("duplicate provider per user returns already exists", func(t *testing.T) {
		u := createUser(t, "per_user_"+uuid.NewString()[:8]+"@example.com")
		require.NoError(t, oauthRepo.Create(ctx, &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u.ID,
			Provider:       domain.OAuthProviderYandex,
			ProviderUserID: "yandex-1",
		}))

		err := oauthRepo.Create(ctx, &domain.OAuthIdentity{
			ID:             uuid.New(),
			UserID:         u.ID,
			Provider:       domain.OAuthProviderYandex,
			ProviderUserID: "yandex-2",
		})
		assert.ErrorIs(t, err, sherrors.ErrAlreadyExists)
	})

	t.Run("parallel create same subject one wins", func(t *testing.T) {
		u := createUser(t, "race_oauth_"+uuid.NewString()[:8]+"@example.com")
		var wg sync.WaitGroup
		results := make(chan error, 2)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- oauthRepo.Create(ctx, &domain.OAuthIdentity{
					ID:             uuid.New(),
					UserID:         u.ID,
					Provider:       domain.OAuthProviderGoogle,
					ProviderUserID: "parallel-subject",
				})
			}()
		}
		wg.Wait()
		close(results)

		var okCount, errCount int
		for err := range results {
			if err == nil {
				okCount++
			} else {
				errCount++
				assert.ErrorIs(t, err, sherrors.ErrAlreadyExists)
			}
		}
		assert.Equal(t, 1, okCount)
		assert.Equal(t, 1, errCount)
	})
}
```

- [ ] **Step 2: Run tests and confirm failure**

Run:

```bash
cd services/core && go test -tags integration ./internal/modules/identity/repository -run OAuthIdentityRepo -count=1
```

Expected: compile failure because `NewOAuthIdentityRepo` is missing.

- [ ] **Step 3: Implement OAuth identity repository**

Create `services/core/internal/modules/identity/repository/oauth_identity_repo.go`:

```go
package repository

import (
	"context"
	"fmt"

	"github.com/foodsea/core/ent"
	"github.com/foodsea/core/ent/oauthidentity"
	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthIdentityRepo struct {
	client *ent.Client
}

func NewOAuthIdentityRepo(client *ent.Client) *OAuthIdentityRepo {
	return &OAuthIdentityRepo{client: client}
}

func (r *OAuthIdentityRepo) GetByProviderUserID(
	ctx context.Context,
	provider domain.OAuthProviderName,
	providerUserID string,
) (*domain.OAuthIdentity, error) {
	identity, err := r.client.OAuthIdentity.Query().
		Where(
			oauthidentity.Provider(string(provider)),
			oauthidentity.ProviderUserID(providerUserID),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sherrors.ErrNotFound
		}
		return nil, fmt.Errorf("getting oauth identity: %w", err)
	}
	return toDomainOAuthIdentity(identity), nil
}

func (r *OAuthIdentityRepo) Create(ctx context.Context, identity *domain.OAuthIdentity) error {
	created, err := r.client.OAuthIdentity.Create().
		SetID(identity.ID).
		SetUserID(identity.UserID).
		SetProvider(string(identity.Provider)).
		SetProviderUserID(identity.ProviderUserID).
		SetNillableEmail(identity.Email).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return sherrors.ErrAlreadyExists
		}
		return fmt.Errorf("creating oauth identity: %w", err)
	}
	identity.CreatedAt = created.CreatedAt
	identity.UpdatedAt = created.UpdatedAt
	return nil
}

func toDomainOAuthIdentity(identity *ent.OAuthIdentity) *domain.OAuthIdentity {
	return &domain.OAuthIdentity{
		ID:             identity.ID,
		UserID:         identity.UserID,
		Provider:       domain.OAuthProviderName(identity.Provider),
		ProviderUserID: identity.ProviderUserID,
		Email:          identity.Email,
		CreatedAt:      identity.CreatedAt,
		UpdatedAt:      identity.UpdatedAt,
	}
}
```

- [ ] **Step 4: Run repository tests and commit**

Run:

```bash
cd services/core && go test -tags integration ./internal/modules/identity/repository -count=1
```

Expected: PASS.

Commit:

```bash
git add services/core/internal/modules/identity/repository/oauth_identity_repo.go services/core/internal/modules/identity/repository/oauth_identity_repo_integration_test.go
git commit -m "feat(core): add oauth identity repository"
```

---

## Task 5: Redis OAuth State Store

**Files:**
- Create: `services/core/internal/modules/identity/repository/oauth_state_store.go`
- Create: `services/core/internal/modules/identity/repository/oauth_state_store_test.go`
- Create: `services/core/internal/modules/identity/repository/oauth_state_store_integration_test.go`

- [ ] **Step 1: Write state store tests**

Create `services/core/internal/modules/identity/repository/oauth_state_store_test.go`:

```go
package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/foodsea/core/internal/modules/identity/repository"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestOAuthStateStore_EmptyStateUnauthorized(t *testing.T) {
	store := repository.NewOAuthStateStore(redis.NewClient(&redis.Options{Addr: "localhost:0"}), 10*time.Minute)
	_, err := store.Consume(context.Background(), "")
	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
}
```

Create `services/core/internal/modules/identity/repository/oauth_state_store_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/repository"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestOAuthStateStore_Integration(t *testing.T) {
	ctx := context.Background()
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = redisContainer.Terminate(ctx) })

	host, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	port, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{Addr: host + ":" + port.Port()})
	t.Cleanup(func() { _ = redisClient.Close() })

	store := repository.NewOAuthStateStore(redisClient, 10*time.Minute)
	session := domain.OAuthSession{
		Provider:     domain.OAuthProviderGoogle,
		RedirectURI:  "foodsea://oauth/google/callback",
		Nonce:        "nonce",
		CodeVerifier: "verifier",
		ExpiresAt:    time.Now().Add(10 * time.Minute).UTC(),
	}

	state, err := store.Create(ctx, session)
	require.NoError(t, err)
	require.NotEmpty(t, state)

	loaded, err := store.Consume(ctx, state)
	require.NoError(t, err)
	assert.Equal(t, session.Provider, loaded.Provider)
	assert.Equal(t, session.RedirectURI, loaded.RedirectURI)
	assert.Equal(t, session.Nonce, loaded.Nonce)
	assert.Equal(t, session.CodeVerifier, loaded.CodeVerifier)

	_, err = store.Consume(ctx, state)
	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
}
```

- [ ] **Step 2: Run state store tests and confirm failure**

Run:

```bash
cd services/core && go test ./internal/modules/identity/repository -run OAuthStateStore -count=1
```

Expected: compile failure because `NewOAuthStateStore` does not exist.

- [ ] **Step 3: Implement state store**

Create `services/core/internal/modules/identity/repository/oauth_state_store.go`:

```go
package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthStateStore struct {
	redis *redis.Client
	ttl   time.Duration
}

func NewOAuthStateStore(redisClient *redis.Client, ttl time.Duration) *OAuthStateStore {
	return &OAuthStateStore{redis: redisClient, ttl: ttl}
}

func (s *OAuthStateStore) Create(ctx context.Context, session domain.OAuthSession) (string, error) {
	state, err := randomURLToken(32)
	if err != nil {
		return "", fmt.Errorf("generating oauth state: %w", err)
	}
	session.ExpiresAt = time.Now().Add(s.ttl).UTC()

	data, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("encoding oauth session: %w", err)
	}
	if err := s.redis.Set(ctx, oauthStateKey(state), data, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("storing oauth state: %w", err)
	}
	return state, nil
}

func (s *OAuthStateStore) Consume(ctx context.Context, state string) (domain.OAuthSession, error) {
	if state == "" {
		return domain.OAuthSession{}, sherrors.ErrUnauthorized
	}
	key := oauthStateKey(state)
	data, err := s.redis.GetDel(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return domain.OAuthSession{}, sherrors.ErrUnauthorized
		}
		return domain.OAuthSession{}, fmt.Errorf("loading oauth state: %w", err)
	}

	var session domain.OAuthSession
	if err := json.Unmarshal(data, &session); err != nil {
		return domain.OAuthSession{}, fmt.Errorf("decoding oauth session: %w", err)
	}
	if time.Now().After(session.ExpiresAt) {
		return domain.OAuthSession{}, sherrors.ErrUnauthorized
	}
	return session, nil
}

func oauthStateKey(state string) string {
	sum := sha256.Sum256([]byte(state))
	return "oauth:state:" + hex.EncodeToString(sum[:])
}

func randomURLToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
```

- [ ] **Step 4: Run state store tests and commit**

Run:

```bash
cd services/core && go test ./internal/modules/identity/repository -run OAuthStateStore -count=1
cd services/core && go test -tags integration ./internal/modules/identity/repository -run OAuthStateStore -count=1
```

Expected: PASS.

Commit:

```bash
git add services/core/internal/modules/identity/repository/oauth_state_store.go services/core/internal/modules/identity/repository/oauth_state_store_test.go services/core/internal/modules/identity/repository/oauth_state_store_integration_test.go
git commit -m "feat(core): add oauth state store"
```

---

## Task 6: OAuth Use Cases

**Files:**
- Create: `services/core/internal/modules/identity/usecase/oauth_start.go`
- Create: `services/core/internal/modules/identity/usecase/oauth_callback.go`
- Create: `services/core/internal/modules/identity/usecase/oauth_start_test.go`
- Create: `services/core/internal/modules/identity/usecase/oauth_callback_test.go`
- Modify: `services/core/internal/modules/identity/usecase/mocks_test.go`

- [ ] **Step 1: Add test mocks**

In `services/core/internal/modules/identity/usecase/mocks_test.go`, add mocks:

```go
type MockOAuthStateStore struct{ mock.Mock }

func (m *MockOAuthStateStore) Create(ctx context.Context, session domain.OAuthSession) (string, error) {
	args := m.Called(ctx, session)
	return args.String(0), args.Error(1)
}

func (m *MockOAuthStateStore) Consume(ctx context.Context, state string) (domain.OAuthSession, error) {
	args := m.Called(ctx, state)
	return args.Get(0).(domain.OAuthSession), args.Error(1)
}

type MockOAuthProvider struct{ mock.Mock }

func (m *MockOAuthProvider) Name() domain.OAuthProviderName {
	args := m.Called()
	return args.Get(0).(domain.OAuthProviderName)
}

func (m *MockOAuthProvider) AuthURL(ctx context.Context, state string, session domain.OAuthSession) (string, error) {
	args := m.Called(ctx, state, session)
	return args.String(0), args.Error(1)
}

func (m *MockOAuthProvider) Exchange(ctx context.Context, code string, session domain.OAuthSession) (domain.OAuthProviderProfile, error) {
	args := m.Called(ctx, code, session)
	return args.Get(0).(domain.OAuthProviderProfile), args.Error(1)
}

type MockOAuthIdentityRepository struct{ mock.Mock }

func (m *MockOAuthIdentityRepository) GetByProviderUserID(ctx context.Context, provider domain.OAuthProviderName, providerUserID string) (*domain.OAuthIdentity, error) {
	args := m.Called(ctx, provider, providerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.OAuthIdentity), args.Error(1)
}

func (m *MockOAuthIdentityRepository) Create(ctx context.Context, identity *domain.OAuthIdentity) error {
	args := m.Called(ctx, identity)
	return args.Error(0)
}
```

- [ ] **Step 2: Write OAuthStart tests**

Create `services/core/internal/modules/identity/usecase/oauth_start_test.go`:

```go
package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/foodsea/core/internal/modules/identity/domain"
	"github.com/foodsea/core/internal/modules/identity/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

func TestOAuthStart(t *testing.T) {
	ctx := context.Background()
	allowed := []string{"foodsea://oauth/google/callback"}

	t.Run("success", func(t *testing.T) {
		store := &MockOAuthStateStore{}
		provider := &MockOAuthProvider{}
		provider.On("Name").Return(domain.OAuthProviderGoogle)
		store.On("Create", ctx, mock.MatchedBy(func(s domain.OAuthSession) bool {
			return s.Provider == domain.OAuthProviderGoogle && s.RedirectURI == allowed[0]
		})).Return("state-1", nil)
		provider.On("AuthURL", ctx, "state-1", mock.AnythingOfType("domain.OAuthSession")).Return("https://google/auth", nil)

		uc := usecase.NewOAuthStart(store, []domain.OAuthProvider{provider}, allowed, 10*time.Minute)
		result, err := uc.Execute(ctx, domain.OAuthStartRequest{Provider: domain.OAuthProviderGoogle, RedirectURI: allowed[0]})

		require.NoError(t, err)
		assert.Equal(t, "https://google/auth", result.AuthURL)
		assert.Equal(t, "state-1", result.State)
	})

	t.Run("disallowed redirect uri", func(t *testing.T) {
		uc := usecase.NewOAuthStart(&MockOAuthStateStore{}, nil, allowed, 10*time.Minute)
		_, err := uc.Execute(ctx, domain.OAuthStartRequest{Provider: domain.OAuthProviderGoogle, RedirectURI: "bad://callback"})
		assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
	})

	t.Run("disabled provider", func(t *testing.T) {
		uc := usecase.NewOAuthStart(&MockOAuthStateStore{}, nil, allowed, 10*time.Minute)
		_, err := uc.Execute(ctx, domain.OAuthStartRequest{Provider: domain.OAuthProviderYandex, RedirectURI: allowed[0]})
		assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
	})
}
```

- [ ] **Step 3: Write OAuthCallback tests**

Create `services/core/internal/modules/identity/usecase/oauth_callback_test.go`. Add the package, imports, and the concrete existing-identity test below:

```go
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

func TestOAuthCallback_ExistingIdentity(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	session := domain.OAuthSession{
		Provider:    domain.OAuthProviderGoogle,
		RedirectURI: "foodsea://oauth/google/callback",
	}
	profile := domain.OAuthProviderProfile{
		Provider:       domain.OAuthProviderGoogle,
		ProviderUserID: "google-sub",
		EmailVerified:  true,
	}

	store := &MockOAuthStateStore{}
	provider := &MockOAuthProvider{}
	identities := &MockOAuthIdentityRepository{}
	users := &MockUserRepository{}
	tokens := &MockTokenService{}

	store.On("Consume", ctx, "state").Return(session, nil)
	provider.On("Name").Return(domain.OAuthProviderGoogle)
	provider.On("Exchange", ctx, "code", session).Return(profile, nil)
	identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "google-sub").Return(&domain.OAuthIdentity{
		UserID:         userID,
		Provider:       domain.OAuthProviderGoogle,
		ProviderUserID: "google-sub",
	}, nil)
	users.On("GetByID", ctx, userID).Return(&domain.User{ID: userID}, nil)
	tokens.On("IssuePair", ctx, userID).Return(fakePair(), nil)

	uc := usecase.NewOAuthCallback(store, []domain.OAuthProvider{provider}, identities, users, tokens)
	result, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
		Provider:    domain.OAuthProviderGoogle,
		Code:        "code",
		State:       "state",
		RedirectURI: "foodsea://oauth/google/callback",
	})

	require.NoError(t, err)
	assert.Equal(t, userID, result.User.ID)
	identities.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}
```

Add these additional concrete tests in the same file:

```go
func TestOAuthCallback_VerifiedEmailLinksExistingUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	email := "linked@example.com"
	session := domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectURI: "foodsea://oauth/google/callback"}
	profile := domain.OAuthProviderProfile{
		Provider:       domain.OAuthProviderGoogle,
		ProviderUserID: "google-linked",
		Email:          &email,
		EmailVerified:  true,
	}

	store := &MockOAuthStateStore{}
	provider := &MockOAuthProvider{}
	identities := &MockOAuthIdentityRepository{}
	users := &MockUserRepository{}
	tokens := &MockTokenService{}

	store.On("Consume", ctx, "state").Return(session, nil)
	provider.On("Name").Return(domain.OAuthProviderGoogle)
	provider.On("Exchange", ctx, "code", session).Return(profile, nil)
	identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "google-linked").Return(nil, sherrors.ErrNotFound)
	users.On("GetByEmail", ctx, email).Return(&domain.User{ID: userID, Email: &email}, nil)
	identities.On("Create", ctx, mock.MatchedBy(func(identity *domain.OAuthIdentity) bool {
		return identity.UserID == userID && identity.ProviderUserID == "google-linked"
	})).Return(nil)
	tokens.On("IssuePair", ctx, userID).Return(fakePair(), nil)

	uc := usecase.NewOAuthCallback(store, []domain.OAuthProvider{provider}, identities, users, tokens)
	result, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
		Provider: domain.OAuthProviderGoogle, Code: "code", State: "state", RedirectURI: "foodsea://oauth/google/callback",
	})

	require.NoError(t, err)
	assert.Equal(t, userID, result.User.ID)
}

func TestOAuthCallback_NewOAuthUser(t *testing.T) {
	ctx := context.Background()
	email := "new@example.com"
	session := domain.OAuthSession{Provider: domain.OAuthProviderYandex, RedirectURI: "foodsea://oauth/yandex/callback"}
	profile := domain.OAuthProviderProfile{
		Provider:       domain.OAuthProviderYandex,
		ProviderUserID: "yandex-new",
		Email:          &email,
		EmailVerified:  true,
	}

	store := &MockOAuthStateStore{}
	provider := &MockOAuthProvider{}
	identities := &MockOAuthIdentityRepository{}
	users := &MockUserRepository{}
	tokens := &MockTokenService{}

	store.On("Consume", ctx, "state").Return(session, nil)
	provider.On("Name").Return(domain.OAuthProviderYandex)
	provider.On("Exchange", ctx, "code", session).Return(profile, nil)
	identities.On("GetByProviderUserID", ctx, domain.OAuthProviderYandex, "yandex-new").Return(nil, sherrors.ErrNotFound)
	users.On("GetByEmail", ctx, email).Return(nil, sherrors.ErrNotFound)
	users.On("CreateOAuth", ctx, mock.MatchedBy(func(u *domain.User) bool {
		return u.Email != nil && *u.Email == email
	})).Return(nil).Run(func(args mock.Arguments) {
		u := args.Get(1).(*domain.User)
		tokens.On("IssuePair", ctx, u.ID).Return(fakePair(), nil)
	})
	identities.On("Create", ctx, mock.AnythingOfType("*domain.OAuthIdentity")).Return(nil)

	uc := usecase.NewOAuthCallback(store, []domain.OAuthProvider{provider}, identities, users, tokens)
	result, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
		Provider: domain.OAuthProviderYandex, Code: "code", State: "state", RedirectURI: "foodsea://oauth/yandex/callback",
	})

	require.NoError(t, err)
	assert.Equal(t, email, *result.User.Email)
}

func TestOAuthCallback_MissingEmailConflict(t *testing.T) {
	ctx := context.Background()
	session := domain.OAuthSession{Provider: domain.OAuthProviderYandex, RedirectURI: "foodsea://oauth/yandex/callback"}
	profile := domain.OAuthProviderProfile{Provider: domain.OAuthProviderYandex, ProviderUserID: "yandex-no-email"}

	store := &MockOAuthStateStore{}
	provider := &MockOAuthProvider{}
	identities := &MockOAuthIdentityRepository{}
	users := &MockUserRepository{}

	store.On("Consume", ctx, "state").Return(session, nil)
	provider.On("Name").Return(domain.OAuthProviderYandex)
	provider.On("Exchange", ctx, "code", session).Return(profile, nil)
	identities.On("GetByProviderUserID", ctx, domain.OAuthProviderYandex, "yandex-no-email").Return(nil, sherrors.ErrNotFound)

	uc := usecase.NewOAuthCallback(store, []domain.OAuthProvider{provider}, identities, users, &MockTokenService{})
	_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
		Provider: domain.OAuthProviderYandex, Code: "code", State: "state", RedirectURI: "foodsea://oauth/yandex/callback",
	})

	assert.ErrorIs(t, err, sherrors.ErrConflict)
}

func TestOAuthCallback_ReusedStateUnauthorized(t *testing.T) {
	ctx := context.Background()
	store := &MockOAuthStateStore{}
	store.On("Consume", ctx, "state").Return(domain.OAuthSession{}, sherrors.ErrUnauthorized)

	uc := usecase.NewOAuthCallback(store, nil, &MockOAuthIdentityRepository{}, &MockUserRepository{}, &MockTokenService{})
	_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
		Provider: domain.OAuthProviderGoogle, Code: "code", State: "state", RedirectURI: "foodsea://oauth/google/callback",
	})

	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
}

func TestOAuthCallback_RedirectMismatchUnauthorized(t *testing.T) {
	ctx := context.Background()
	store := &MockOAuthStateStore{}
	store.On("Consume", ctx, "state").Return(domain.OAuthSession{
		Provider: domain.OAuthProviderGoogle, RedirectURI: "foodsea://oauth/google/callback",
	}, nil)

	uc := usecase.NewOAuthCallback(store, nil, &MockOAuthIdentityRepository{}, &MockUserRepository{}, &MockTokenService{})
	_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
		Provider: domain.OAuthProviderGoogle, Code: "code", State: "state", RedirectURI: "foodsea://oauth/other/callback",
	})

	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
}

func TestOAuthCallback_ProviderExchangeUnauthorized(t *testing.T) {
	ctx := context.Background()
	session := domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectURI: "foodsea://oauth/google/callback"}
	store := &MockOAuthStateStore{}
	provider := &MockOAuthProvider{}
	store.On("Consume", ctx, "state").Return(session, nil)
	provider.On("Name").Return(domain.OAuthProviderGoogle)
	provider.On("Exchange", ctx, "code", session).Return(domain.OAuthProviderProfile{}, sherrors.ErrUnauthorized)

	uc := usecase.NewOAuthCallback(store, []domain.OAuthProvider{provider}, &MockOAuthIdentityRepository{}, &MockUserRepository{}, &MockTokenService{})
	_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
		Provider: domain.OAuthProviderGoogle, Code: "code", State: "state", RedirectURI: "foodsea://oauth/google/callback",
	})

	assert.ErrorIs(t, err, sherrors.ErrUnauthorized)
}

func TestOAuthCallback_MalformedInput(t *testing.T) {
	uc := usecase.NewOAuthCallback(&MockOAuthStateStore{}, nil, &MockOAuthIdentityRepository{}, &MockUserRepository{}, &MockTokenService{})
	_, err := uc.Execute(context.Background(), domain.OAuthCallbackRequest{Provider: domain.OAuthProviderGoogle})
	assert.ErrorIs(t, err, sherrors.ErrInvalidInput)
}

func TestOAuthCallback_ProviderRepositoryError(t *testing.T) {
	ctx := context.Background()
	session := domain.OAuthSession{Provider: domain.OAuthProviderGoogle, RedirectURI: "foodsea://oauth/google/callback"}
	profile := domain.OAuthProviderProfile{Provider: domain.OAuthProviderGoogle, ProviderUserID: "google-sub", EmailVerified: true}
	store := &MockOAuthStateStore{}
	provider := &MockOAuthProvider{}
	identities := &MockOAuthIdentityRepository{}
	dbErr := errors.New("db down")

	store.On("Consume", ctx, "state").Return(session, nil)
	provider.On("Name").Return(domain.OAuthProviderGoogle)
	provider.On("Exchange", ctx, "code", session).Return(profile, nil)
	identities.On("GetByProviderUserID", ctx, domain.OAuthProviderGoogle, "google-sub").Return(nil, dbErr)

	uc := usecase.NewOAuthCallback(store, []domain.OAuthProvider{provider}, identities, &MockUserRepository{}, &MockTokenService{})
	_, err := uc.Execute(ctx, domain.OAuthCallbackRequest{
		Provider: domain.OAuthProviderGoogle, Code: "code", State: "state", RedirectURI: "foodsea://oauth/google/callback",
	})

	assert.ErrorIs(t, err, dbErr)
}
```

- [ ] **Step 4: Run usecase tests and confirm failure**

Run:

```bash
cd services/core && go test ./internal/modules/identity/usecase -run OAuth -count=1
```

Expected: compile failure because OAuth use cases do not exist.

- [ ] **Step 5: Implement OAuthStart**

Create `services/core/internal/modules/identity/usecase/oauth_start.go`:

```go
package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthStart struct {
	states        domain.OAuthStateStore
	providers     map[domain.OAuthProviderName]domain.OAuthProvider
	redirectURIs  map[string]struct{}
	stateTTL      time.Duration
}

func NewOAuthStart(
	states domain.OAuthStateStore,
	providers []domain.OAuthProvider,
	allowedRedirectURIs []string,
	stateTTL time.Duration,
) *OAuthStart {
	providerMap := make(map[domain.OAuthProviderName]domain.OAuthProvider, len(providers))
	for _, provider := range providers {
		providerMap[provider.Name()] = provider
	}
	redirectMap := make(map[string]struct{}, len(allowedRedirectURIs))
	for _, uri := range allowedRedirectURIs {
		redirectMap[uri] = struct{}{}
	}
	return &OAuthStart{states: states, providers: providerMap, redirectURIs: redirectMap, stateTTL: stateTTL}
}

func (s *OAuthStart) Execute(ctx context.Context, req domain.OAuthStartRequest) (domain.OAuthStartResult, error) {
	provider, ok := s.providers[req.Provider]
	if !ok {
		return domain.OAuthStartResult{}, fmt.Errorf("%w: oauth provider disabled", sherrors.ErrInvalidInput)
	}
	if _, ok := s.redirectURIs[req.RedirectURI]; !ok {
		return domain.OAuthStartResult{}, fmt.Errorf("%w: redirect_uri is not allowed", sherrors.ErrInvalidInput)
	}

	session := domain.OAuthSession{
		Provider:    req.Provider,
		RedirectURI: req.RedirectURI,
		ExpiresAt:   time.Now().Add(s.stateTTL).UTC(),
	}
	state, err := s.states.Create(ctx, session)
	if err != nil {
		return domain.OAuthStartResult{}, err
	}
	authURL, err := provider.AuthURL(ctx, state, session)
	if err != nil {
		return domain.OAuthStartResult{}, err
	}
	return domain.OAuthStartResult{AuthURL: authURL, State: state}, nil
}
```

- [ ] **Step 6: Implement OAuthCallback**

Create `services/core/internal/modules/identity/usecase/oauth_callback.go`:

```go
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/identity/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type OAuthCallbackResult struct {
	User      *domain.User
	TokenPair domain.TokenPair
}

type OAuthCallback struct {
	states     domain.OAuthStateStore
	providers  map[domain.OAuthProviderName]domain.OAuthProvider
	identities domain.OAuthIdentityRepository
	users      domain.UserRepository
	tokens     domain.TokenService
}

func NewOAuthCallback(
	states domain.OAuthStateStore,
	providers []domain.OAuthProvider,
	identities domain.OAuthIdentityRepository,
	users domain.UserRepository,
	tokens domain.TokenService,
) *OAuthCallback {
	providerMap := make(map[domain.OAuthProviderName]domain.OAuthProvider, len(providers))
	for _, provider := range providers {
		providerMap[provider.Name()] = provider
	}
	return &OAuthCallback{states: states, providers: providerMap, identities: identities, users: users, tokens: tokens}
}

func (c *OAuthCallback) Execute(ctx context.Context, req domain.OAuthCallbackRequest) (OAuthCallbackResult, error) {
	if req.Code == "" || req.State == "" || req.RedirectURI == "" {
		return OAuthCallbackResult{}, fmt.Errorf("%w: code, state, and redirect_uri are required", sherrors.ErrInvalidInput)
	}
	session, err := c.states.Consume(ctx, req.State)
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	if session.Provider != req.Provider || session.RedirectURI != req.RedirectURI {
		return OAuthCallbackResult{}, sherrors.ErrUnauthorized
	}

	provider, ok := c.providers[req.Provider]
	if !ok {
		return OAuthCallbackResult{}, fmt.Errorf("%w: oauth provider disabled", sherrors.ErrInvalidInput)
	}
	profile, err := provider.Exchange(ctx, req.Code, session)
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	if profile.Provider != req.Provider || profile.ProviderUserID == "" {
		return OAuthCallbackResult{}, sherrors.ErrUnauthorized
	}

	user, err := c.resolveUser(ctx, profile)
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	pair, err := c.tokens.IssuePair(ctx, user.ID)
	if err != nil {
		return OAuthCallbackResult{}, fmt.Errorf("issuing tokens: %w", err)
	}
	return OAuthCallbackResult{User: user, TokenPair: pair}, nil
}

func (c *OAuthCallback) resolveUser(ctx context.Context, profile domain.OAuthProviderProfile) (*domain.User, error) {
	identity, err := c.identities.GetByProviderUserID(ctx, profile.Provider, profile.ProviderUserID)
	if err == nil {
		return c.users.GetByID(ctx, identity.UserID)
	}
	if !errors.Is(err, sherrors.ErrNotFound) {
		return nil, err
	}

	if profile.Email == nil || *profile.Email == "" || !profile.EmailVerified {
		return nil, fmt.Errorf("%w: verified email is required for first oauth login", sherrors.ErrConflict)
	}

	user, err := c.users.GetByEmail(ctx, *profile.Email)
	if err != nil {
		if !errors.Is(err, sherrors.ErrNotFound) {
			return nil, err
		}
		user = &domain.User{ID: uuid.New(), Email: profile.Email}
		if err := c.users.CreateOAuth(ctx, user); err != nil {
			return nil, err
		}
	}

	err = c.identities.Create(ctx, &domain.OAuthIdentity{
		ID:             uuid.New(),
		UserID:         user.ID,
		Provider:       profile.Provider,
		ProviderUserID: profile.ProviderUserID,
		Email:          profile.Email,
	})
	if err != nil {
		if errors.Is(err, sherrors.ErrAlreadyExists) {
			identity, readErr := c.identities.GetByProviderUserID(ctx, profile.Provider, profile.ProviderUserID)
			if readErr != nil {
				return nil, readErr
			}
			return c.users.GetByID(ctx, identity.UserID)
		}
		return nil, err
	}
	return user, nil
}
```

- [ ] **Step 7: Run OAuth usecase tests and commit**

Run:

```bash
cd services/core && go test ./internal/modules/identity/usecase -run OAuth -cover -count=1
```

Expected: PASS and 100% coverage for the new `oauth_start.go` and `oauth_callback.go` files. Add missing tests before committing if those files are below 100%.

Commit:

```bash
git add services/core/internal/modules/identity/usecase
git commit -m "feat(core): add oauth use cases"
```

---

## Task 7: Provider Adapters

**Files:**
- Create: `services/core/internal/modules/identity/repository/oauth_provider_http.go`
- Create: `services/core/internal/modules/identity/repository/oauth_provider_google.go`
- Create: `services/core/internal/modules/identity/repository/oauth_provider_yandex.go`
- Create: `services/core/internal/modules/identity/repository/oauth_provider_google_test.go`
- Create: `services/core/internal/modules/identity/repository/oauth_provider_yandex_test.go`

- [ ] **Step 1: Write provider tests**

For Google, use `httptest.Server` with `/token` and `/certs`. Generate an RSA key in the test, return a JWKS with one `kid`, sign an ID token using `github.com/golang-jwt/jwt/v5`, and assert:

```go
profile, err := provider.Exchange(ctx, "valid-code", domain.OAuthSession{
	Provider:    domain.OAuthProviderGoogle,
	RedirectURI: redirectURI,
	Nonce:       "nonce-1",
})
require.NoError(t, err)
assert.Equal(t, domain.OAuthProviderGoogle, profile.Provider)
assert.Equal(t, "google-sub-1", profile.ProviderUserID)
assert.Equal(t, "user@example.com", *profile.Email)
assert.True(t, profile.EmailVerified)
```

Add Google tests for:

- token endpoint non-200 returns `ErrUnauthorized`;
- ID token with wrong nonce returns `ErrUnauthorized`;
- ID token with wrong audience returns `ErrUnauthorized`;
- unverified email returns a profile with `EmailVerified=false`.

For Yandex, use `httptest.Server` with `/token` and `/info`. Assert:

```go
profile, err := provider.Exchange(ctx, "valid-code", domain.OAuthSession{
	Provider:    domain.OAuthProviderYandex,
	RedirectURI: redirectURI,
})
require.NoError(t, err)
assert.Equal(t, domain.OAuthProviderYandex, profile.Provider)
assert.Equal(t, "yandex-id-1", profile.ProviderUserID)
assert.Equal(t, "user@yandex.ru", *profile.Email)
assert.True(t, profile.EmailVerified)
```

Add Yandex tests for:

- token endpoint non-200 returns `ErrUnauthorized`;
- userinfo endpoint non-200 returns `ErrUnauthorized`;
- userinfo without id returns `ErrUnauthorized`;
- userinfo without email returns `Email=nil` and `EmailVerified=false`.

- [ ] **Step 2: Run provider tests and confirm failure**

Run:

```bash
cd services/core && go test ./internal/modules/identity/repository -run 'OAuthProvider' -count=1
```

Expected: compile failure because providers do not exist.

- [ ] **Step 3: Implement shared provider helpers**

Create `services/core/internal/modules/identity/repository/oauth_provider_http.go` with small helpers for form POST, JSON decode, and non-200 mapping:

```go
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func postFormJSON(ctx context.Context, client *http.Client, endpoint string, form url.Values, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, headers map[string]string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
```

- [ ] **Step 4: Implement Google provider**

Implement constructor:

```go
func NewGoogleOAuthProvider(cfg config.OAuthProviderConfig, client *http.Client) *GoogleOAuthProvider
```

`AuthURL` must build a URL with:

- `client_id`
- `redirect_uri`
- `response_type=code`
- `scope=openid email profile`
- `state`
- `nonce`
- `access_type=offline`

`Exchange` must post `grant_type=authorization_code`, `code`, `redirect_uri`, `client_id`, `client_secret`; then validate the returned `id_token` with JWKS and `jwt/v5`. Return `sherrors.ErrUnauthorized` on token exchange or claim validation failure.

- [ ] **Step 5: Implement Yandex provider**

Implement constructor:

```go
func NewYandexOAuthProvider(cfg config.OAuthProviderConfig, client *http.Client) *YandexOAuthProvider
```

`AuthURL` must build a URL with:

- `client_id`
- `redirect_uri`
- `response_type=code`
- `scope=login:email login:info`
- `state`

`Exchange` must post `grant_type=authorization_code`, `code`, `redirect_uri`, `client_id`, `client_secret`; then call userinfo with `Authorization: OAuth <access_token>`. Return `sherrors.ErrUnauthorized` on exchange/userinfo failure.

- [ ] **Step 6: Run provider tests and commit**

Run:

```bash
cd services/core && go test ./internal/modules/identity/repository -run 'OAuthProvider' -cover -count=1
```

Expected: PASS and 100% coverage for provider adapter files. Add missing tests before committing.

Commit:

```bash
git add services/core/internal/modules/identity/repository/oauth_provider_*.go services/core/internal/modules/identity/repository/oauth_provider_*_test.go
git commit -m "feat(core): add google and yandex oauth providers"
```

---

## Task 8: HTTP Handlers and Routes

**Files:**
- Modify: `services/core/internal/modules/identity/handler/dto.go`
- Modify: `services/core/internal/modules/identity/handler/auth_handler.go`
- Modify: `services/core/internal/modules/identity/handler/auth_handler_test.go`
- Modify: `services/core/internal/modules/identity/module.go`

- [ ] **Step 1: Add handler tests**

Extend `services/core/internal/modules/identity/handler/auth_handler_test.go` with mocks:

```go
type mockOAuthStart struct{ mock.Mock }

func (m *mockOAuthStart) Execute(ctx context.Context, req domain.OAuthStartRequest) (domain.OAuthStartResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(domain.OAuthStartResult), args.Error(1)
}

type mockOAuthCallback struct{ mock.Mock }

func (m *mockOAuthCallback) Execute(ctx context.Context, req domain.OAuthCallbackRequest) (usecase.OAuthCallbackResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(usecase.OAuthCallbackResult), args.Error(1)
}
```

Add routes to `setupAuthRouter`:

```go
r.GET("/auth/oauth/:provider/start", h.OAuthStart)
r.POST("/auth/oauth/:provider/callback", h.OAuthCallback)
```

Add tests:

```go
func TestAuthHandler_OAuthStart(t *testing.T) {
	start := &mockOAuthStart{}
	start.On("Execute", mock.Anything, domain.OAuthStartRequest{
		Provider:    domain.OAuthProviderGoogle,
		RedirectURI: "foodsea://oauth/google/callback",
	}).Return(domain.OAuthStartResult{AuthURL: "https://google/auth", State: "state"}, nil)

	h := NewAuthHandler(&mockRegister{}, &mockLogin{}, &mockRefresh{}, &mockLogout{}, start, &mockOAuthCallback{})
	req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/start?redirect_uri=foodsea://oauth/google/callback", nil)
	w := httptest.NewRecorder()
	setupAuthRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "https://google/auth")
}
```

Add tests for bad provider -> 400, disallowed redirect -> 400, callback success -> 200 AuthResponse, callback missing code -> 400, callback conflict -> 409, callback unauthorized -> 401.

- [ ] **Step 2: Run handler tests and confirm failure**

Run:

```bash
cd services/core && go test ./internal/modules/identity/handler -run OAuth -count=1
```

Expected: compile failure because handler methods do not exist.

- [ ] **Step 3: Add DTOs**

In `services/core/internal/modules/identity/handler/dto.go`, add:

```go
type OAuthStartResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

type OAuthCallbackRequest struct {
	Code        string `json:"code" binding:"required"`
	State       string `json:"state" binding:"required"`
	RedirectURI string `json:"redirect_uri" binding:"required"`
}
```

- [ ] **Step 4: Extend AuthHandler dependencies**

In `auth_handler.go`, add interfaces:

```go
type oauthStartUseCase interface {
	Execute(ctx context.Context, req domain.OAuthStartRequest) (domain.OAuthStartResult, error)
}

type oauthCallbackUseCase interface {
	Execute(ctx context.Context, req domain.OAuthCallbackRequest) (usecase.OAuthCallbackResult, error)
}
```

Add fields to `AuthHandler` and parameters to `NewAuthHandler`:

```go
oauthStart    oauthStartUseCase
oauthCallback oauthCallbackUseCase
```

- [ ] **Step 5: Implement handler methods**

Add methods:

```go
func (h *AuthHandler) OAuthStart(c *gin.Context) {
	provider, ok := domain.ParseOAuthProviderName(c.Param("provider"))
	if !ok {
		httputil.HandleError(c, sherrors.ErrInvalidInput)
		return
	}
	result, err := h.oauthStart.Execute(c.Request.Context(), domain.OAuthStartRequest{
		Provider:    provider,
		RedirectURI: c.Query("redirect_uri"),
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}
	httputil.OK(c, OAuthStartResponse{AuthURL: result.AuthURL, State: result.State})
}

func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	provider, ok := domain.ParseOAuthProviderName(c.Param("provider"))
	if !ok {
		httputil.HandleError(c, sherrors.ErrInvalidInput)
		return
	}

	var req OAuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}

	result, err := h.oauthCallback.Execute(c.Request.Context(), domain.OAuthCallbackRequest{
		Provider:    provider,
		Code:        req.Code,
		State:       req.State,
		RedirectURI: req.RedirectURI,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}
	httputil.OK(c, toAuthResponse(result.User, result.TokenPair))
}
```

Add swagger annotations above both methods.

- [ ] **Step 6: Register routes**

In `services/core/internal/modules/identity/module.go`, add:

```go
auth.GET("/oauth/:provider/start", m.authHandler.OAuthStart)
auth.POST("/oauth/:provider/callback", m.authHandler.OAuthCallback)
```

- [ ] **Step 7: Run handler tests and commit**

Run:

```bash
cd services/core && go test ./internal/modules/identity/handler -run OAuth -count=1
```

Expected: PASS.

Commit:

```bash
git add services/core/internal/modules/identity/handler services/core/internal/modules/identity/module.go
git commit -m "feat(core): add oauth http endpoints"
```

---

## Task 9: Dependency Injection

**Files:**
- Modify: `services/core/internal/modules/identity/module.go`
- Modify: `services/core/cmd/api/main.go`

- [ ] **Step 1: Extend identity deps**

In `services/core/internal/modules/identity/module.go`, add to `Deps`:

```go
OAuth config.OAuthConfig
HTTPClient *http.Client
```

Import `net/http`.

- [ ] **Step 2: Wire repositories and use cases**

In `NewModule`, create:

```go
httpClient := deps.HTTPClient
if httpClient == nil {
	httpClient = http.DefaultClient
}

providers := make([]domain.OAuthProvider, 0, 2)
if deps.OAuth.Google.Enabled {
	providers = append(providers, repository.NewGoogleOAuthProvider(deps.OAuth.Google, httpClient))
}
if deps.OAuth.Yandex.Enabled {
	providers = append(providers, repository.NewYandexOAuthProvider(deps.OAuth.Yandex, httpClient))
}

stateStore := repository.NewOAuthStateStore(deps.Redis, deps.OAuth.StateTTL)
oauthIdentityRepo := repository.NewOAuthIdentityRepo(deps.Ent)
oauthStartUC := usecase.NewOAuthStart(stateStore, providers, deps.OAuth.AllowedRedirectURIs, deps.OAuth.StateTTL)
oauthCallbackUC := usecase.NewOAuthCallback(stateStore, providers, oauthIdentityRepo, userRepo, tokenSvc)
```

Pass `oauthStartUC` and `oauthCallbackUC` to `handler.NewAuthHandler`.

- [ ] **Step 3: Update main**

In `services/core/cmd/api/main.go`, update identity module construction:

```go
identityModule := identity.NewModule(identity.Deps{
	Ent:   entClient,
	Redis: redisCache.Client(),
	Cache: redisCache,
	Log:   log,
	JWT:   cfg.JWT,
	OAuth: cfg.OAuth,
})
```

- [ ] **Step 4: Run core compile tests and commit**

Run:

```bash
cd services/core && go test ./internal/modules/identity/... ./cmd/api -run TestDoesNotExist -count=1
```

Expected: no compile errors; command exits PASS/no tests for packages without tests.

Commit:

```bash
git add services/core/internal/modules/identity/module.go services/core/cmd/api/main.go
git commit -m "feat(core): wire oauth dependencies"
```

---

## Task 10: E2E OAuth Flow

**Files:**
- Modify: `services/core/test/e2e/suite_test.go`
- Create: `services/core/test/e2e/oauth_test.go`

- [ ] **Step 1: Add OAuth test config hooks to suite**

In `services/core/test/e2e/suite_test.go`, create a fake provider before building `identity.NewModule`. The fake provider should expose token, JWKS, and userinfo endpoints and produce a Google-like signed ID token plus Yandex userinfo responses.

Add package variables:

```go
testGoogleRedirectURI = "foodsea://oauth/google/callback"
testYandexRedirectURI = "foodsea://oauth/yandex/callback"
```

Pass OAuth config to identity:

```go
OAuth: config.OAuthConfig{
	StateTTL: 10 * time.Minute,
	AllowedRedirectURIs: []string{testGoogleRedirectURI, testYandexRedirectURI},
	Google: config.OAuthProviderConfig{
		Enabled:      true,
		ClientID:     "google-client",
		ClientSecret: "google-secret",
		AuthURL:      fakeOAuth.URL + "/google/auth",
		TokenURL:     fakeOAuth.URL + "/google/token",
		JWKSURL:      fakeOAuth.URL + "/google/certs",
	},
	Yandex: config.OAuthProviderConfig{
		Enabled:      true,
		ClientID:     "yandex-client",
		ClientSecret: "yandex-secret",
		AuthURL:      fakeOAuth.URL + "/yandex/auth",
		TokenURL:     fakeOAuth.URL + "/yandex/token",
		UserInfoURL:  fakeOAuth.URL + "/yandex/info",
	},
},
```

- [ ] **Step 2: Write OAuth e2e tests**

Create `services/core/test/e2e/oauth_test.go`:

```go
//go:build e2e

package e2e

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type oauthStartResp struct {
	Data struct {
		AuthURL string `json:"auth_url"`
		State   string `json:"state"`
	} `json:"data"`
}

func TestOAuthE2E(t *testing.T) {
	t.Run("google start callback refresh and state replay", func(t *testing.T) {
		startURL := testBaseURL + "/api/v1/auth/oauth/google/start?redirect_uri=" + url.QueryEscape(testGoogleRedirectURI)
		resp, err := httpClient.Get(startURL)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var start oauthStartResp
		require.NoError(t, decodeJSON(resp, &start))
		require.NotEmpty(t, start.Data.AuthURL)
		require.NotEmpty(t, start.Data.State)

		resp, err = postJSON(testBaseURL+"/api/v1/auth/oauth/google/callback", map[string]string{
			"code":         "google-code-new-user",
			"state":        start.Data.State,
			"redirect_uri": testGoogleRedirectURI,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var auth registerResp
		require.NoError(t, decodeJSON(resp, &auth))
		require.NotEmpty(t, auth.Data.Access)
		require.NotEmpty(t, auth.Data.Refresh)

		resp, err = postJSON(testBaseURL+"/api/v1/auth/refresh", map[string]string{"refresh_token": auth.Data.Refresh})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp, err = postJSON(testBaseURL+"/api/v1/auth/oauth/google/callback", map[string]string{
			"code":         "google-code-new-user",
			"state":        start.Data.State,
			"redirect_uri": testGoogleRedirectURI,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("yandex start and callback", func(t *testing.T) {
		startURL := testBaseURL + "/api/v1/auth/oauth/yandex/start?redirect_uri=" + url.QueryEscape(testYandexRedirectURI)
		resp, err := httpClient.Get(startURL)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var start oauthStartResp
		require.NoError(t, decodeJSON(resp, &start))

		resp, err = postJSON(testBaseURL+"/api/v1/auth/oauth/yandex/callback", map[string]string{
			"code":         "yandex-code-new-user",
			"state":        start.Data.State,
			"redirect_uri": testYandexRedirectURI,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("verified oauth email links existing password user", func(t *testing.T) {
		email := "linked-oauth@foodsea.test"
		password := "SuperSecret1!"
		resp, err := postJSON(testBaseURL+"/api/v1/auth/register", map[string]string{
			"email":    email,
			"password": password,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		startURL := testBaseURL + "/api/v1/auth/oauth/google/start?redirect_uri=" + url.QueryEscape(testGoogleRedirectURI)
		resp, err = httpClient.Get(startURL)
		require.NoError(t, err)
		var start oauthStartResp
		require.NoError(t, decodeJSON(resp, &start))

		resp, err = postJSON(testBaseURL+"/api/v1/auth/oauth/google/callback", map[string]string{
			"code":         "google-code-linked-user",
			"state":        start.Data.State,
			"redirect_uri": testGoogleRedirectURI,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		resp, err = postJSON(testBaseURL+"/api/v1/auth/login", map[string]string{
			"email":    email,
			"password": password,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})
}
```

- [ ] **Step 3: Run e2e OAuth tests and confirm failure**

Run:

```bash
cd services/core && go test -tags e2e -count=1 -timeout 5m ./test/e2e/... -run OAuth
```

Expected: failure until fake provider support and wiring are complete.

- [ ] **Step 4: Implement fake provider support in suite**

Add helper types/functions in `suite_test.go` or a new e2e helper file:

- RSA key generation for Google ID tokens;
- JWKS endpoint returning the public key;
- token endpoint mapping code strings to profiles;
- Yandex userinfo endpoint returning `id`, `default_email`, and `emails`;
- no logs containing code/token/state values.

- [ ] **Step 5: Run e2e OAuth tests and commit**

Run:

```bash
cd services/core && go test -tags e2e -count=1 -timeout 5m ./test/e2e/... -run OAuth
```

Expected: PASS.

Commit:

```bash
git add services/core/test/e2e
git commit -m "test(core): add oauth e2e coverage"
```

---

## Task 11: Migration, Swagger, and Notes

**Files:**
- Create: `services/core/migrations/<timestamp>_oauth_identities.sql`
- Modify: `services/core/migrations/atlas.sum`
- Modify: `services/core/docs/swagger/*`
- Modify: `docs/api/core-swagger.*`
- Modify: `services/core/internal/modules/identity/architecture-notes.md`
- Modify: `services/core/cmd/api/architecture-notes.md`

- [ ] **Step 1: Generate migration**

Run the repo's Atlas workflow. If `atlas` is installed through `make tools`, run:

```bash
CORE_DB_URL=postgres://postgres:postgres@localhost:5433/core_db?sslmode=disable make atlas-diff-core
make atlas-hash
```

Expected migration content:

```sql
ALTER TABLE "users" ALTER COLUMN "password_hash" DROP NOT NULL;
CREATE TABLE "oauth_identities" (
  "id" uuid NOT NULL,
  "provider" character varying NOT NULL,
  "provider_user_id" character varying NOT NULL,
  "email" character varying NULL,
  "user_id" uuid NOT NULL,
  "created_at" timestamp with time zone NOT NULL,
  "updated_at" timestamp with time zone NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "oauth_identities_users_oauth_identities" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
CREATE UNIQUE INDEX "oauthidentity_provider_provider_user_id" ON "oauth_identities" ("provider", "provider_user_id");
CREATE UNIQUE INDEX "oauthidentity_provider_user_id" ON "oauth_identities" ("provider", "user_id");
```

If Atlas emits different constraint names, keep Atlas output and verify the semantics match the SQL above.

- [ ] **Step 2: Regenerate swagger**

Run:

```bash
make swagger
cp services/core/docs/swagger/swagger.yaml docs/api/core-swagger.yaml
cp services/core/docs/swagger/swagger.json docs/api/core-swagger.json
```

Expected: new `auth/oauth/{provider}/start` and `auth/oauth/{provider}/callback` operations appear in core swagger.

- [ ] **Step 3: Update architecture notes**

In `services/core/internal/modules/identity/architecture-notes.md`, add:

```md
## OAuth Google/Yandex ID

- OAuth is an entry point into the existing FoodSea session model; protected APIs still use FoodSea JWT access tokens and Redis-backed refresh tokens.
- `GET /auth/oauth/:provider/start` creates a short-lived Redis state and returns the provider authorization URL.
- `POST /auth/oauth/:provider/callback` consumes state once, exchanges the authorization code, resolves a local user, and issues FoodSea tokens.
- External accounts are stored in `oauth_identities` with unique `(provider, provider_user_id)` and `(provider, user_id)`.
- Verified provider email may auto-link to an existing password account. New OAuth-only users have `password_hash = NULL` and cannot use password login.
- Provider codes, access tokens, ID tokens, raw state, nonce, and PKCE verifier values must not be logged.
```

In `services/core/cmd/api/architecture-notes.md`, add the two public routes to the identity route list.

- [ ] **Step 4: Commit migration/docs**

Run:

```bash
git add services/core/migrations services/core/docs/swagger docs/api/core-swagger.yaml docs/api/core-swagger.json services/core/internal/modules/identity/architecture-notes.md services/core/cmd/api/architecture-notes.md
git commit -m "docs(core): document oauth auth flow"
```

---

## Task 12: Coverage Gate and Final Verification

**Files:**
- No planned source changes unless verification exposes uncovered OAuth lines.

- [ ] **Step 1: Run focused OAuth package coverage**

Run:

```bash
cd services/core && go test ./internal/modules/identity/... -coverprofile=identity.out -count=1
cd services/core && go tool cover -func=identity.out
```

Expected: all tests PASS. Every new file containing OAuth implementation has 100.0% statement coverage. If a new OAuth file is below 100.0%, add a focused test for the missing branch before continuing.

- [ ] **Step 2: Run integration tests**

Run:

```bash
cd services/core && go test -tags integration ./internal/modules/identity/... -count=1
```

Expected: PASS.

- [ ] **Step 3: Run e2e OAuth tests**

Run:

```bash
cd services/core && go test -tags e2e -count=1 -timeout 5m ./test/e2e/... -run OAuth
```

Expected: PASS.

- [ ] **Step 4: Run core unit suite**

Run:

```bash
cd services/core && go test ./internal/... -short -count=1
```

Expected: PASS.

- [ ] **Step 5: Run full core e2e if time allows**

Run:

```bash
make test-e2e-core
```

Expected: PASS. If Docker/testcontainers are unavailable, capture the exact failure and keep the focused unit/integration evidence from previous steps.

- [ ] **Step 6: Final commit**

If verification required additional test fixes, commit them:

```bash
git add services/core
git commit -m "test(core): enforce oauth coverage"
```

If there are no changes after verification, do not create an empty commit.
