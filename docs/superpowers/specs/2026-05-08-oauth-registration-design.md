# OAuth Registration Design

## Context

FoodSea currently authenticates users in `core-service` through the `identity`
module. The implemented flow accepts either email or phone plus password, stores
users in `core_db.users`, issues internal JWT access tokens, and stores hashed
refresh tokens in Redis.

This design adds OAuth sign-in and registration for Google and Yandex ID while
preserving the existing FoodSea session model. OAuth is only an entry point into
the existing local account system; all protected APIs continue to use FoodSea
JWTs and refresh tokens.

The root `01-architecture.md` and `02-tech-stack.md` files referenced by
`AGENTS.md` are not present in the current checkout. This design is based on
`AGENTS.md`, `services/core/internal/modules/identity/architecture-notes.md`,
`services/core/cmd/api/architecture-notes.md`, and the current identity code.

## Goals

- Add Google and Yandex ID registration/login through backend-owned
  authorization-code flow.
- Keep `core-service/internal/modules/identity` as the only module that creates
  FoodSea auth sessions.
- Automatically link an OAuth identity to an existing local account when the
  provider returns the same verified email.
- Create a local OAuth-only user when the provider identity is new and the email
  does not belong to an existing user.
- Cover the new OAuth code with 100% test coverage and keep regression coverage
  for existing email/phone auth.

## Non-Goals

- No separate auth-service.
- No protected connect/disconnect endpoints in the first release.
- No password setup/reset flow for OAuth-only users.
- No storage of provider access tokens or refresh tokens after the callback is
  completed.
- No real Google or Yandex calls in CI; tests use fake providers and
  `httptest.Server`.

## API Design

### Start OAuth

`GET /api/v1/auth/oauth/:provider/start?redirect_uri=<uri>`

Supported providers are `google` and `yandex`.

Success response:

```json
{
  "data": {
    "auth_url": "https://accounts.google.com/o/oauth2/v2/auth?...",
    "state": "opaque-state"
  }
}
```

The endpoint validates that the provider is enabled and the `redirect_uri` is in
the configured allowlist. It generates an opaque `state`, stores a short-lived
OAuth session in Redis, and returns the provider authorization URL to the iOS
client.

### Complete OAuth

`POST /api/v1/auth/oauth/:provider/callback`

Request:

```json
{
  "code": "provider-authorization-code",
  "state": "opaque-state",
  "redirect_uri": "foodsea://oauth/google/callback"
}
```

Success response reuses the existing `AuthResponse` shape:

```json
{
  "data": {
    "user": {
      "id": "00000000-0000-0000-0000-000000000000",
      "email": "user@example.com",
      "onboarding_done": false,
      "created_at": "2026-05-08T00:00:00Z",
      "updated_at": "2026-05-08T00:00:00Z"
    },
    "access_token": "foodsea-access-token",
    "refresh_token": "foodsea-refresh-token",
    "access_expires_at": "2026-05-08T00:15:00Z",
    "refresh_expires_at": "2026-06-07T00:00:00Z"
  }
}
```

## Runtime Flow

1. iOS calls `start` with the provider and redirect URI.
2. Backend creates `state`, plus provider-specific `nonce` and PKCE values when
   required by the provider adapter.
3. Backend stores the OAuth session in Redis under a hashed state key with a
   default TTL of 10 minutes.
4. Backend returns the authorization URL to iOS.
5. iOS opens the provider authorization page and receives a redirect containing
   `code` and `state`.
6. iOS posts `code`, `state`, and `redirect_uri` to `callback`.
7. Backend consumes the Redis state once. Reuse, expiration, provider mismatch,
   or redirect URI mismatch fails the callback.
8. Backend exchanges the code with the provider and validates the returned
   identity data.
9. Backend resolves the local FoodSea user and OAuth identity.
10. Backend issues the normal FoodSea token pair through the existing
    `TokenService`.

## Provider Behavior

### Google

Google uses OpenID Connect authorization-code flow. The provider adapter must:

- exchange `code` for tokens against Google's token endpoint;
- validate the ID token signature and claims;
- verify `iss`, `aud`, `exp`, and `nonce`;
- use `sub` as `provider_user_id`;
- use email for local account linking only when `email_verified` is true.

Google email is not the stable account key; `sub` is the stable provider
identifier.

### Yandex ID

Yandex uses authorization-code exchange followed by a user information request.
The provider adapter must:

- exchange `code` against Yandex's token endpoint;
- call Yandex ID userinfo with `Authorization: OAuth <access_token>`;
- use the stable Yandex user id from the userinfo response as
  `provider_user_id`;
- use `default_email` or a returned email value for local account linking only
  when the response contains an email.

If a new Yandex identity has no email and no existing OAuth identity, callback
returns a conflict error because the first release does not support an
email-less local account creation path.

## Data Model

Add a new Ent schema:

`services/core/ent/schema/oauth_identity.go`

Fields:

- `id uuid`, default `uuid.New`, immutable;
- `provider string`, enum-like value `google` or `yandex`;
- `provider_user_id string`, required;
- `email string`, optional and nillable;
- timestamps through the existing timestamp mixin.

Edges:

- required edge from `OAuthIdentity` to `User`;
- `User` gets an inverse edge to OAuth identities.

Constraints:

- unique index on `(provider, provider_user_id)`;
- unique index on `(provider, user_id)` for the first release, so one user can
  have at most one identity per provider.

Change `users.password_hash` from required to optional and nillable. OAuth-only
users have no password hash. Password login for a user without password hash
returns `ErrUnauthorized`.

## Domain and Use Cases

Add focused interfaces to `identity/domain`:

- `OAuthStateStore`: creates and consumes short-lived OAuth sessions;
- `OAuthProvider`: builds authorization URLs, exchanges codes, and returns a
  normalized provider profile;
- `OAuthIdentityRepository`: finds and creates OAuth identity links.

Extend `UserRepository` with explicit methods needed by OAuth:

- create an OAuth-only user with optional email;
- find user by email for verified-email linking;
- run user creation/linking in a transaction or through a repository method that
  handles unique constraint races.

Add two use cases:

- `OAuthStart` validates provider and redirect URI, creates state, and returns
  the provider auth URL.
- `OAuthCallback` consumes state, validates provider identity, resolves or
  creates the local user, creates the OAuth identity when needed, and issues a
  FoodSea token pair.

Resolution rules:

1. Existing `(provider, provider_user_id)` identity wins and logs in its user.
2. Missing identity plus verified matching email links to the existing local
   user.
3. Missing identity plus new email creates an OAuth-only user and identity.
4. Missing identity plus no usable email returns `ErrConflict`.
5. Provider/token/state validation failures return `ErrUnauthorized` or
   `ErrInvalidInput` depending on whether the client sent malformed input or the
   provider flow failed.

## Configuration

Extend core config with:

- `OAUTH_STATE_TTL`, default `10m`;
- `OAUTH_ALLOWED_REDIRECT_URIS`, comma-separated allowlist;
- `GOOGLE_OAUTH_CLIENT_ID`;
- `GOOGLE_OAUTH_CLIENT_SECRET`;
- `YANDEX_OAUTH_CLIENT_ID`;
- `YANDEX_OAUTH_CLIENT_SECRET`.

Provider is enabled only when its client id and secret are both present. In
production, configured OAuth providers must have complete credentials and at
least one allowed redirect URI. In development and tests, a provider with missing
credentials is disabled and `start` returns a provider-disabled error.

## Error Handling

Use existing sentinel errors and `httputil.HandleError`:

- invalid provider, missing code, missing state, disallowed redirect URI:
  `ErrInvalidInput` -> 400;
- invalid/expired/reused state or failed provider verification:
  `ErrUnauthorized` -> 401;
- email required for first-time Yandex user or provider identity cannot be
  safely linked: `ErrConflict` -> 409;
- duplicate identity races are resolved by re-reading the winning row before
  returning an error.

No response body includes provider tokens, authorization codes, raw state,
nonce, or PKCE verifier values.

## Security Notes

- Redis keys store only `sha256(state)`, not raw state.
- OAuth state is single-use. `Consume` deletes the state before or atomically
  with returning the session.
- OAuth session stores provider, redirect URI, nonce, optional PKCE verifier,
  and expiration timestamp.
- Logs must never include authorization code, provider tokens, FoodSea refresh
  tokens, raw state, nonce, or PKCE verifier.
- Provider access tokens are held only in memory during callback processing.
- Existing FoodSea access and refresh token semantics stay unchanged.

## Test Plan

The target is 100% statement coverage for new OAuth packages and use cases.
Existing identity tests remain in place as regression coverage.

Unit tests:

- provider enum validation and redirect URI allowlist validation;
- OAuth state generation, hashing, JSON encoding, TTL, consume-once behavior;
- Google auth URL contains client id, redirect URI, scopes, state, nonce, and
  PKCE data when configured;
- Yandex auth URL contains client id, redirect URI, scopes, and state;
- callback resolution for existing OAuth identity;
- callback resolution for verified-email auto-linking;
- callback resolution for new OAuth-only user;
- callback conflict for missing or untrusted email;
- callback unauthorized for expired/reused state;
- callback unauthorized for provider token exchange or identity verification
  failure;
- password login returns 401 for OAuth-only users with nil password hash.

Repository integration tests:

- create and read OAuth identity by provider subject;
- unique `(provider, provider_user_id)` constraint;
- unique `(provider, user_id)` constraint;
- create OAuth-only user with nil password hash;
- duplicate provider-subject race resolves to one local identity.

Handler tests:

- `GET /auth/oauth/:provider/start` success;
- start with bad provider;
- start with disallowed redirect URI;
- callback success returns the existing `AuthResponse` shape;
- callback malformed JSON or missing fields;
- callback maps conflict and unauthorized errors correctly.

E2E tests:

- fake Google provider supports full `start -> callback -> refresh` path;
- fake Yandex provider supports full `start -> callback -> refresh` path;
- callback replay with the same state fails after the first success;
- existing password user with verified provider email is linked and can still
  login with password afterward.

Verification commands:

```bash
cd services/core && go test ./internal/modules/identity/... -coverprofile=identity.out
cd services/core && go tool cover -func=identity.out
cd services/core && go test ./test/e2e/... -run OAuth -count=1
```

The implementation plan should include package-level coverage checks for the new
OAuth code and an explicit review of uncovered lines.

## Documentation Updates

Update these files when implementing the feature:

- `services/core/internal/modules/identity/architecture-notes.md`;
- `services/core/cmd/api/architecture-notes.md`;
- Swagger annotations and generated docs for the two new endpoints;
- env documentation in the nearest available backend documentation file, since
  root `02-tech-stack.md` is absent in the current checkout.

## Open Decisions Resolved

- Use backend-owned authorization-code flow.
- Backend creates `state` and provider authorization URLs.
- Automatically link by verified email.
- First release includes only sign-in/registration, not profile-level
  connect/disconnect.
- Store provider links in a separate `oauth_identities` table.
