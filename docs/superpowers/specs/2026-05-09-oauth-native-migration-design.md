# OAuth Native Migration Design

## Context

Документ фиксирует:

1. как OAuth реализован в `core-service` **сейчас** (фактическое состояние кода),
2. что нужно сделать для перехода на **native OAuth** для мобильного приложения (iOS-first),
3. как провести миграцию без поломки текущего backend-owned flow.

Базовый модуль: `services/core/internal/modules/identity`.

---

## Current Implementation (As-Is)

### Public API

- `GET /api/v1/auth/oauth/:provider/start?redirect_uri=...`
- `POST /api/v1/auth/oauth/:provider/callback`

Провайдеры: `google`, `yandex`.

### Runtime Flow

1. Клиент вызывает `start` с `provider` и `redirect_uri`.
2. Backend валидирует allowlist redirect URI (`OAUTH_ALLOWED_REDIRECT_URIS`).
3. Backend генерирует `state`, сохраняет OAuth-сессию в Redis и возвращает `auth_url`.
4. Клиент открывает provider auth page.
5. Клиент получает `code` + `state` и отправляет `callback` в backend.
6. Backend забирает и удаляет `state` (одноразовый `GETDEL`), обменивает `code` у провайдера, резолвит/создаёт локального пользователя, выдаёт FoodSea JWT.

### State Storage

- Redis key: `oauth:state:<sha256(state)>`
- Session fields: `provider`, `redirect_to`, `created_at`, `expires_at`, `state`
- TTL: `OAUTH_STATE_TTL` (по умолчанию `10m`)

### Provider Adapters

- Google: `oauth_provider_google.go`
  - authorization code flow через `client_id + client_secret`
  - `nonce` выставляется равным `state`
  - `id_token` парсится и валидируются claims (`iss`, `aud`, `exp`, `nonce`, `sub`)
- Yandex: `oauth_provider_yandex.go`
  - code exchange через `client_id + client_secret`
  - userinfo через `Authorization: OAuth <access_token>`
  - для релиза используются scope `login:email login:avatar`

### Identity Linking

- Таблица `oauth_identities`:
  - unique `(provider, provider_user_id)`
  - unique `(provider, user_id)`
- Стратегия:
  1. есть identity -> логин в связанного юзера,
  2. нет identity, но есть verified email -> линк к существующему пользователю,
  3. нет identity и email новый -> создаётся OAuth-only user (`password_hash = NULL`),
  4. нет verified email -> `409`.

### Known Gaps of Current Mobile Flow

1. **Web-style UX**: мобильный клиент фактически работает как посредник web flow.
2. **`localhost` redirect не production-friendly** на реальном устройстве.
3. **Google native requirements (PKCE, client type separation)** не реализованы.
4. В `OAuthCallbackRequest` есть `redirect_uri`, но в use case callback он **не используется**.
5. Google `id_token` claims валидируются, но в текущем адаптере нет полноценной криптографической проверки подписи по JWK.

---

## Target (To-Be): Native OAuth

## Goals

- Добавить полноценный native-compatible flow для iOS (дальше расширяемо на Android).
- Внедрить PKCE для Google native.
- Развести web и native provider credentials/config.
- Сохранить текущий backend-owned flow как fallback на период миграции.

## Non-Goals

- Не меняем доменную модель FoodSea токенов.
- Не выносим auth в отдельный сервис.
- Не добавляем в этой итерации social account management UI (disconnect/link в профиле).

---

## Design Overview

### 1) Separate OAuth Modes

Вводим два режима:

- `web` (текущий, backward-compatible),
- `native` (новый, PKCE-first).

Определение режима через отдельный endpoint namespace (рекомендовано):

- `GET /api/v1/auth/oauth/native/:provider/start`
- `POST /api/v1/auth/oauth/native/:provider/callback`

Текущие `/auth/oauth/:provider/*` оставляем как legacy web flow.

### 2) Native OAuth Session Model

Расширяем OAuth state payload:

- `provider`
- `redirect_to`
- `mode` (`native` / `web`)
- `nonce` (для OIDC)
- `pkce_verifier` (только native flow)
- `created_at`
- `expires_at`

### 3) Provider Strategy for Native

#### Google Native

- Использовать отдельный Google OAuth client для iOS/native.
- Запрос авторизации с `code_challenge` (`S256`) + `code_challenge_method=S256`.
- В token exchange передавать `code_verifier`.
- Для native-контура не использовать web client secret как обязательный атрибут flow.
- Добавить полноценную валидацию `id_token` подписи (JWKS + `kid` + `alg`) поверх текущей claim validation.

#### Yandex Native

Два допустимых варианта:

1. OAuth через системный браузер + callback в app redirect URI,
2. Yandex Mobile Auth SDK (iOS) как целевой вариант UX.

Для backend migration этот документ фиксирует вариант (1), чтобы не блокировать релиз SDK-зависимостью.

---

## API Contract Changes

### Native Start

`GET /api/v1/auth/oauth/native/:provider/start?redirect_uri=<uri>`

Response:

```json
{
  "data": {
    "auth_url": "...",
    "state": "..."
  }
}
```

### Native Callback

`POST /api/v1/auth/oauth/native/:provider/callback`

Request:

```json
{
  "code": "provider-code",
  "state": "opaque-state",
  "redirect_uri": "app://oauth/callback"
}
```

Важно: `redirect_uri` в callback должен реально участвовать в валидации и token exchange; текущее поведение (field required, но игнорируется) убрать.

---

## Configuration Changes

### Keep Existing

- `OAUTH_ALLOWED_REDIRECT_URIS`
- `OAUTH_STATE_TTL`

### Add Native-Specific Config

- `OAUTH_GOOGLE_NATIVE_CLIENT_ID`
- `OAUTH_GOOGLE_NATIVE_AUTH_URL` (default as now)
- `OAUTH_GOOGLE_NATIVE_TOKEN_URL` (default as now)
- `OAUTH_GOOGLE_NATIVE_SCOPES` (default `openid,email,profile`)

Опционально для staged rollout:

- `OAUTH_NATIVE_ENABLED` (global flag)
- `OAUTH_NATIVE_PROVIDERS` (comma-separated)

Yandex можно сначала переиспользовать текущие creds, но лучше иметь явные:

- `OAUTH_YANDEX_NATIVE_CLIENT_ID`
- `OAUTH_YANDEX_NATIVE_CLIENT_SECRET` (если провайдер требует в текущем сценарии)
- `OAUTH_YANDEX_NATIVE_SCOPES` (default `login:email,login:avatar`)

---

## Security Requirements

1. PKCE verifier хранится только в short-lived Redis state.
2. State одноразовый (`GETDEL`), повтор -> `401`.
3. Redirect URI must match allowlist exactly (как сейчас) и mode-aware.
4. Полная валидация Google ID token подписи по JWKS.
5. Логи не содержат:
   - code,
   - provider access token,
   - id_token raw,
   - state raw,
   - pkce verifier.

---

## Backward Compatibility / Rollout

1. Шаг 1: добавить native endpoints параллельно текущим.
2. Шаг 2: мобильный клиент переключить на `/oauth/native/*`.
3. Шаг 3: оставить legacy `/oauth/*` на grace period.
4. Шаг 4: после стабилизации:
   - ограничить legacy flow флагом,
   - затем удалить.

---

## Implementation Work Breakdown

1. **Domain**
   - расширить OAuth session (mode, nonce, pkce_verifier),
   - добавить native-specific request/response модели.

2. **Use cases**
   - `OAuthStartNative`: генерация state + PKCE,
   - `OAuthCallbackNative`: валидация redirect_uri + consume state + exchange через verifier.

3. **Providers**
   - Google native adapter (PKCE + strict ID token validation),
   - Yandex native adapter (browser-based native callback flow).

4. **Handlers / Routes**
   - новые `/auth/oauth/native/:provider/start|callback`,
   - `redirect_uri` в callback использовать в реальной валидации.

5. **Config**
   - native env variables + validation rules.

6. **Docs / Swagger**
   - расширить `docs/api/core-swagger.yaml`,
   - добавить mobile integration notes.

---

## Testing Plan (100% for new native code)

### Unit

- PKCE generation/encoding edge cases.
- State creation/consume для native payload.
- Redirect allowlist validation (positive/negative).
- Provider-specific URL build and token exchange request payload.
- ID token signature + claims validation matrix.

### Integration (module-level)

- `start(native)` success/failure paths.
- `callback(native)`:
  - invalid state,
  - reused state,
  - mismatched redirect,
  - provider token exchange fail,
  - identity linking scenarios.

### E2E

- Fake Google provider: full native start->callback->refresh path.
- Fake Yandex provider: full native start->callback->refresh path.
- Regression: legacy web `/oauth/*` remains functional during migration.

### Coverage Gate

- New files in native OAuth path: target 100% statements + branches where practical.
- Existing OAuth modules: no regression below current baseline.

---

## Open Questions

1. Выбираем ли сразу Yandex Mobile SDK для iOS, или сначала browser-based native callback?
2. Нужно ли отключить legacy `/oauth/*` сразу в production, или оставить feature flag rollout?
3. Должен ли backend поддержать несколько native redirect URI (dev/test/prod app schemes) в одном окружении?

---

## Success Criteria

- iOS может пройти Google/Yandex OAuth без `localhost` callback.
- Native flow использует PKCE (Google) и проходит security checks.
- Backward-compatible migration без деградации текущей авторизации.
- Тесты для нового native OAuth кода покрывают 100% добавленного функционала.

