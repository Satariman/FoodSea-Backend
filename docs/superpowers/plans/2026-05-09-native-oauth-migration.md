# Native OAuth Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить native OAuth в `core-service` (Google PKCE + Yandex SDK), оставить legacy OAuth за feature flag и поддержать несколько native redirect URI.

**Architecture:** Расширяем текущий `identity` OAuth-контур: вводим разделение `legacy` и `native` путей на уровне роутов/конфига, PKCE-сессию для Google native в Redis state, отдельный SDK-token callback для Yandex, и сохраняем текущую модель локального user linking + FoodSea JWT. Все изменения делаем внутри `services/core/internal/modules/identity` и `services/core/internal/platform/config`.

**Tech Stack:** Go, Gin, Redis, Ent, swaggo, testify.

---

### Task 1: Config + Feature Flags

**Files:**
- Modify: `services/core/internal/platform/config/config.go`
- Modify: `services/core/internal/platform/config/config_test.go`

- [ ] **Step 1: Add failing config tests for native/legacy flags and native redirect list**
- [ ] **Step 2: Run config tests to confirm RED**
- [ ] **Step 3: Implement config fields**
  - `OAuth.LegacyEnabled`
  - `OAuth.NativeEnabled`
  - `OAuth.NativeAllowedRedirectURIs`
  - `OAuth.GoogleNative` (separate client config)
  - `OAuth.YandexNativeSDKEnabled`
- [ ] **Step 4: Re-run config tests to confirm GREEN**

### Task 2: Domain Model for Native OAuth

**Files:**
- Modify: `services/core/internal/modules/identity/domain/oauth.go`
- Modify: `services/core/internal/modules/identity/usecase/mocks_test.go`
- Test: `services/core/internal/modules/identity/usecase/oauth_start_test.go`
- Test: `services/core/internal/modules/identity/usecase/oauth_callback_test.go`

- [ ] **Step 1: Add failing tests for native request/session fields**
- [ ] **Step 2: Run targeted usecase tests to confirm RED**
- [ ] **Step 3: Extend domain structs/interfaces**
  - `OAuthSession.Mode`, `Nonce`, `PKCEVerifier`
  - `OAuthStartRequest.Mode`
  - `OAuthCallbackRequest.RedirectURI`, `CodeVerifier`
  - `OAuthProvider` native methods
- [ ] **Step 4: Update mocks and compile**
- [ ] **Step 5: Re-run tests to confirm GREEN**

### Task 3: Native OAuth Start Use Case (Google)

**Files:**
- Modify: `services/core/internal/modules/identity/usecase/oauth_start.go`
- Test: `services/core/internal/modules/identity/usecase/oauth_start_test.go`

- [ ] **Step 1: Add failing tests for native mode behavior**
  - allowed native redirect URI list
  - generation of `nonce` + PKCE verifier
  - provider-native route selection
- [ ] **Step 2: Run usecase tests to confirm RED**
- [ ] **Step 3: Implement native start flow**
- [ ] **Step 4: Re-run tests to confirm GREEN**

### Task 4: Native OAuth Callback Use Case

**Files:**
- Modify: `services/core/internal/modules/identity/usecase/oauth_callback.go`
- Test: `services/core/internal/modules/identity/usecase/oauth_callback_test.go`

- [ ] **Step 1: Add failing tests for native callback**
  - redirect mismatch
  - missing PKCE verifier for Google native
  - state mode mismatch
- [ ] **Step 2: Run usecase tests to confirm RED**
- [ ] **Step 3: Implement callback validation + native exchange input**
- [ ] **Step 4: Re-run tests to confirm GREEN**

### Task 5: Google Native Provider Adapter (PKCE)

**Files:**
- Modify: `services/core/internal/modules/identity/repository/oauth_provider_google.go`
- Test: `services/core/internal/modules/identity/repository/oauth_provider_google_test.go`

- [ ] **Step 1: Add failing provider tests**
  - auth URL includes `code_challenge` and `code_challenge_method=S256`
  - token exchange sends `code_verifier`
- [ ] **Step 2: Run provider tests to confirm RED**
- [ ] **Step 3: Implement PKCE helper + native auth/exchange**
- [ ] **Step 4: Re-run provider tests to confirm GREEN**

### Task 6: Yandex Mobile SDK Native Callback

**Files:**
- Modify: `services/core/internal/modules/identity/domain/oauth.go`
- Modify: `services/core/internal/modules/identity/usecase/oauth_callback.go`
- Modify: `services/core/internal/modules/identity/handler/dto.go`
- Modify: `services/core/internal/modules/identity/handler/auth_handler.go`
- Test: `services/core/internal/modules/identity/handler/auth_handler_test.go`
- Test: `services/core/internal/modules/identity/usecase/oauth_callback_test.go`

- [ ] **Step 1: Add failing tests for SDK token callback endpoint**
- [ ] **Step 2: Run handler/usecase tests to confirm RED**
- [ ] **Step 3: Implement `POST /auth/oauth/native/yandex/sdk/callback`**
  - request with yandex oauth token from SDK
  - fetch Yandex profile via provider adapter
  - reuse existing identity linking pipeline
- [ ] **Step 4: Re-run tests to confirm GREEN**

### Task 7: Routes, Module Wiring, Legacy Feature Flag

**Files:**
- Modify: `services/core/internal/modules/identity/module.go`
- Modify: `services/core/internal/modules/identity/architecture-notes.md`
- Test: `services/core/internal/modules/identity/handler/auth_handler_test.go`

- [ ] **Step 1: Add failing tests for route availability by flags**
- [ ] **Step 2: Run tests to confirm RED**
- [ ] **Step 3: Wire new native use cases/providers and flag-controlled routes**
  - legacy `/auth/oauth/:provider/*` only when `LegacyEnabled`
  - native `/auth/oauth/native/:provider/*` when `NativeEnabled`
- [ ] **Step 4: Update architecture notes**
- [ ] **Step 5: Re-run tests to confirm GREEN**

### Task 8: Swagger + E2E + Final Verification

**Files:**
- Modify: `docs/api/core-swagger.yaml`
- Modify: `docs/api/core-swagger.json`
- Modify: `services/core/test/e2e/oauth_test.go`
- Modify: `Makefile` (if required for native env defaults)

- [ ] **Step 1: Add failing e2e coverage for native endpoints**
- [ ] **Step 2: Run e2e test subset to confirm RED**
- [ ] **Step 3: Implement required e2e fixtures/fakes**
- [ ] **Step 4: Regenerate swagger if handlers changed**
- [ ] **Step 5: Run full relevant verification**
  - `go test ./services/core/internal/modules/identity/...`
  - `go test ./services/core/internal/platform/config`
  - `go test ./services/core/test/e2e -run OAuth`
- [ ] **Step 6: Prepare focused commit(s)**

