# Proposal

## Why

The `access_token`, `refresh_token` and `has_session` cookies are always sent without the `Secure` attribute, so behind HTTPS in production a browser could still send them over plain HTTP. Issue #389.

## What Changes

- The auth cookies carry `Secure` when the API runs in production (`APP_ENV=production`), and stay non-Secure in development so the local HTTP docker-compose stack keeps working.
- A new env variable `AUTH_COOKIE_SECURE` (true/false) overrides the default in either direction.
- Clearing the auth cookies (logout, refresh rejected for an unconfirmed email, password reset) uses the same `Secure`/`SameSite`/`HttpOnly`/`Path` attributes as setting them.
- Production compose sets `AUTH_COOKIE_SECURE` explicitly; env examples and `docs/DEPLOYMENT.md` document it.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `auth/session`: the «Auth cookies» requirement gains the `Secure` attribute rule (on in production, off in development, env override) and matching attributes on clearing.

## Impact

- backend: config (new env), auth HTTP transport cookie helpers and their wiring, tests.
- frontend: none (web/admin only read `access_token`/`has_session` in middleware; Set-Cookie passes through the Next `/api` rewrite unchanged).
- devops: `docker-compose.prod.yml` (api service env), `.env.example`, `.env.production.example`.
- docs: `docs/DEPLOYMENT.md` env table.
