# Proposal

Issue: #624

## Why

Login, forgot-password and resend-verification are throttled, but `POST /api/v1/auth/register` is not: anyone can create accounts in bulk and make the mailer send a verification email to arbitrary third-party addresses.

## What Changes

- `POST /api/v1/auth/register` gets two fixed one-hour windows checked before the account lookup: at most 10 requests per client IP and at most 5 requests per submitted address (trimmed, case-insensitive), counting every outcome that passes validation (201 and 409 alike), so a limited answer is identical for new and existing addresses.
- Over a limit the endpoint answers 429 `rate_limited` with `retry_after_seconds` and a `Retry-After` header, the same shape as login/resend/forgot-password; no account is created and no email is sent.
- 400 validation errors and internal errors do not consume quota; an unavailable limit store lets registration work without limits (fail open, logged), like login.
- Client IP is the one login already uses (`X-Forwarded-For` from Caddy, trusted hops only).
- The web `/register` page shows «Слишком много попыток регистрации. Попробуйте снова через N.» on a 429. The admin app has no registration; it is unchanged.
- No shared per-address email counter across register and resend: register emails a given address at most once (a repeat is 409 without an email), and resend keeps its own limits.

## Capabilities

### New Capabilities

### Modified Capabilities

- `auth/registration`: adds the registration rate limits and the `/register` rate-limit message.

## Impact

- backend: auth service and handler (register throttling, 429 mapping), two new limiters wired in the api entrypoint, Swagger for `/auth/register`.
- frontend: `apps/web` session store / register form message on 429; `apps/admin` untouched.
- devops: none (reuses the existing Redis; new keys `rl:register-ip:*`, `rl:register-email:*` expire by themselves).
- docs: `docs/DEPLOYMENT.md` (per-IP limits list), `docs/ROADMAP.md` (#624 entry).
