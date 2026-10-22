# auth/session Specification

## Purpose

Signs a confirmed user in and keeps them signed in: login, access and refresh tokens, their cookies, rotation, logout and the current-user endpoint, plus how the web and admin apps keep the session alive.

## Requirements

### Requirement: Login
The system SHALL accept `POST /api/v1/auth/login` with `{email, password}` (400 when the email is invalid or the password empty). An unknown email, an account without a password or a wrong password MUST all return the same 401 «invalid credentials». The password MUST be checked before the email confirmation: only with a correct password does an unconfirmed account get 403 with code `email_not_verified`, and then no tokens are created and no cookies are set. On success the response is 200 `{user, access_token, refresh_token, expires_in}` and the auth cookies are set.

#### Scenario: Wrong password on an unconfirmed account
- **WHEN** someone logs in to an unconfirmed account with a wrong password
- **THEN** the response is 401 «invalid credentials», revealing nothing about the confirmation

#### Scenario: Unconfirmed account
- **WHEN** a user with an unconfirmed email logs in with the correct password
- **THEN** the response is 403 with code `email_not_verified` and no cookies are set

### Requirement: Tokens
The access token SHALL be a signed JWT of type `access` for the user, valid for 15 minutes; `expires_in` is its lifetime in seconds. The refresh token SHALL be a random opaque value valid for 30 days; only its hash is stored. Every login creates a new refresh token, so a user can have several sessions.

#### Scenario: Token lifetime
- **WHEN** a user logs in
- **THEN** `expires_in` is 900

### Requirement: Auth cookies
Login and refresh SHALL set three HttpOnly, SameSite=Strict cookies: `access_token` (path `/`, lifetime of the access token), `refresh_token` (path `/api/v1/auth/refresh` only, 30 days) and `has_session` (path `/`, 30 days), a marker without any authorizing value that tells the apps a session may still be refreshable. The three MUST always be set and cleared together. All three cookies MUST carry the `Secure` attribute when the API runs in production and MUST NOT carry it in development, so the local HTTP stack keeps working; an explicit deployment setting MAY override this default in either direction. Clearing the cookies (logout, a refresh rejected for an unconfirmed email, a password reset) MUST use the same `Secure`, `SameSite`, `HttpOnly` and path attributes as setting them, with an immediate expiry.

#### Scenario: Refresh cookie scope
- **WHEN** a logged-in browser calls any API other than `/api/v1/auth/refresh`
- **THEN** the refresh token cookie is not sent

#### Scenario: Secure cookies in production
- **WHEN** a user logs in or refreshes against the API running in production without an override
- **THEN** `access_token`, `refresh_token` and `has_session` are all set with `Secure`, `HttpOnly` and `SameSite=Strict`

#### Scenario: Local HTTP development
- **WHEN** a user logs in against the API running in development over plain HTTP without an override
- **THEN** the three cookies are set without `Secure` and the browser keeps the session

#### Scenario: Explicit override
- **WHEN** the deployment explicitly switches secure cookies on in development (or off in production)
- **THEN** the three cookies follow the explicit setting instead of the environment default

#### Scenario: Clearing matches the set attributes
- **WHEN** a user logs out from the API running with secure cookies
- **THEN** the three cookies are cleared with an immediate expiry, `Secure`, `HttpOnly`, `SameSite=Strict` and their original paths

### Requirement: Refresh with rotation
The system SHALL accept `POST /api/v1/auth/refresh` with the refresh token in the body (`refresh_token`) or, when absent, from the cookie; with neither the response is 400. An unknown, expired or revoked token MUST return 401. A valid token MUST be revoked and replaced: the response is 200 with a new token pair and new cookies. If the token's user has an unconfirmed email, all of that user's refresh tokens MUST be revoked, the cookies cleared, and the response is 401 with code `email_not_verified`.

#### Scenario: Reusing a rotated token
- **WHEN** a client refreshes with a refresh token that was already used for a refresh
- **THEN** the response is 401

### Requirement: Logout
`POST /api/v1/auth/logout` SHALL require authentication, revoke all of the user's refresh tokens (every session of the user) and clear the auth cookies; the response is 200 `{status: "ok"}`.

#### Scenario: Logout ends other sessions
- **WHEN** a user logged in on two devices logs out on one
- **THEN** the other device can no longer refresh its session

### Requirement: Current user
`GET /api/v1/auth/me` SHALL require authentication and return the user `{id, email, name, avatar_url, role, profile_type, email_verified, created_at, updated_at}`. It MUST NOT return the password hash.

#### Scenario: Me without a session
- **WHEN** an anonymous request calls `/api/v1/auth/me`
- **THEN** the response is 401

### Requirement: Keeping the session alive in the apps
The web and admin apps SHALL refresh the access token proactively at about 80% of its lifetime and, as a fallback, refresh once and retry when an authenticated request returns 401. Concurrent refresh attempts MUST be merged into one call, because each refresh rotates the token. Login, registration, refresh, logout and the public email/password endpoints MUST NOT trigger a refresh. When the refresh fails, or the retried request fails again, the original error is returned without a second refresh, and a 401 means the user is signed out.

#### Scenario: Expired access token
- **WHEN** a signed-in user's access token has expired and a request returns 401
- **THEN** the app refreshes once and retries the request without the user noticing

### Requirement: Login pages and return path
The web app SHALL provide `/login` («Войти», a «Забыли пароль?» link and a link to registration); the admin app SHALL provide its own `/login` («Вход в панель управления»). After login the user MUST be sent to the `redirect` query parameter only when it is a same-origin relative path (not `//…`, no backslash or control characters, no scheme); otherwise to `/profile` on the web and `/` in the admin. The web app MUST NOT return to a guest-only page (`/login`, `/register`, `/forgot-password`, `/reset-password`, `/verify-email`). For `email_not_verified` the web login shows that the email is not confirmed with a button to resend the link; the admin shows «Email не подтверждён. Подтвердите его по ссылке из письма.».

#### Scenario: Open redirect attempt
- **WHEN** a user logs in on the web with `?redirect=//evil.example`
- **THEN** they are sent to `/profile`

#### Scenario: Returning to the page that required login
- **WHEN** a visitor is sent to `/login?redirect=/shelves` and logs in
- **THEN** they land on `/shelves`
