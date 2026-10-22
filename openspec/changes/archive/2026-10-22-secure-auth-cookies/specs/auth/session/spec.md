## MODIFIED Requirements

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
