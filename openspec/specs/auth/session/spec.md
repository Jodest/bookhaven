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

### Requirement: Login rate limits
`POST /api/v1/auth/login` SHALL throttle login attempts with two counters over fixed 15-minute windows: at most 60 attempts of any outcome per client IP and at most 10 failed attempts per submitted email address (compared case-insensitively, ignoring surrounding spaces). A failed attempt is any request that passes request validation but does not present the correct password for an existing account with a password (unknown email, account without a password, wrong password). The per-IP counter counts every attempt that reaches it, including correct passwords, so an attacker cannot keep it flat by alternating attempts against a victim with logins to an account of their own. Both limits MUST be checked before the account is looked up and depend only on the request, so a limited response is identical for existing and unknown addresses. When a limit is reached, the response MUST be 429 with code `rate_limited`, a `retry_after_seconds` body field and a `Retry-After` header equal to the time left in the window, and the password MUST NOT be checked. A request with a correct password (a 200, or a 403 `email_not_verified`) MUST clear the failed-attempt counter of its address but MUST still count against the IP. A request that fails with an internal error MUST NOT count against either counter. A 400 validation error MUST NOT count. The limits MUST NOT lock an account permanently: they lift by themselves when the window ends. When the limit store is unavailable, login MUST keep working without limits rather than fail. The endpoint stays public (no bearer token).

#### Scenario: Too many wrong passwords for one address
- **WHEN** someone sends 10 wrong passwords for an address within 15 minutes and then an 11th attempt
- **THEN** the 11th response is 429 `rate_limited` with `retry_after_seconds` and `Retry-After`, and the password is not checked

#### Scenario: Limited response does not reveal the account
- **WHEN** an address with an account and an address without one each reach the per-address limit
- **THEN** both receive the same 429 `rate_limited` response

#### Scenario: Correct password clears the address counter
- **WHEN** a user mistypes the password 9 times and then logs in with the correct password
- **THEN** the login succeeds and the next wrong password for that address starts a fresh count

#### Scenario: Blocked address even with the right password
- **WHEN** an address has reached its limit and its owner then submits the correct password inside the same window
- **THEN** the response is 429 `rate_limited` and no session is created

#### Scenario: Many addresses from one IP
- **WHEN** one client IP sends 60 attempts spread over different addresses within 15 minutes and then another attempt
- **THEN** the response is 429 `rate_limited` regardless of the address

#### Scenario: Correct passwords count against the IP quota
- **WHEN** one client IP alternates wrong passwords for a victim address with correct logins to its own account until it has made 60 attempts within 15 minutes
- **THEN** the next attempt from that IP is 429 `rate_limited`, even though the per-address counters keep being reset

#### Scenario: The block lifts by itself
- **WHEN** an address was limited and its 15-minute window has passed
- **THEN** a login with the correct password succeeds

#### Scenario: Limit store unavailable
- **WHEN** the limit store cannot be reached and a user logs in with the correct password
- **THEN** the login succeeds as if there were no limits

### Requirement: Login rate-limit message
On a 429 from login, the web `/login` and the admin `/login` SHALL show «Слишком много попыток входа. Попробуйте снова через N.», where N is the wait from `retry_after_seconds` rounded up and written as «N сек», «N мин» or «N ч»; without that field the wait MUST be shown as «15 мин». The form MUST stay usable and the message MUST NOT say whether the account exists.

#### Scenario: Web login is throttled
- **WHEN** a visitor submits the web login form and the API answers 429 with `retry_after_seconds` 600
- **THEN** the form shows «Слишком много попыток входа. Попробуйте снова через 10 мин.»

#### Scenario: Admin login is throttled without a wait value
- **WHEN** staff submit the admin login form and the API answers 429 without `retry_after_seconds`
- **THEN** the form shows «Слишком много попыток входа. Попробуйте снова через 15 мин.»

### Requirement: Refresh token retention
The system SHALL permanently delete stored refresh tokens that were revoked (by rotation, logout, an unconfirmed-email refresh or a password reset) or that expired, no earlier than 7 days after the revocation or expiry, by a background purge that runs at least once a day. A token that has not expired and has not been revoked MUST never be deleted by the purge. The purge MUST NOT change any response: a deleted token is answered by `POST /api/v1/auth/refresh` exactly like a revoked or expired one (401). The purge has no endpoint and needs no role.

#### Scenario: Long-revoked token
- **WHEN** a client refreshes with a refresh token that was rotated more than 7 days ago and has since been purged
- **THEN** the response is 401, the same as for a revoked token that is still stored

#### Scenario: Active session survives the purge
- **WHEN** the purge runs while a user holds a refresh token that is neither expired nor revoked
- **THEN** the token is kept and the next refresh with it succeeds
