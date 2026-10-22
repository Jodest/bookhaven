# Proposal

Issue: #196 ([proposal] No brute-force protection on login)

## Why

`POST /api/v1/auth/login` has no throttling, so password guessing and credential stuffing are unlimited, while resend-verification and forgot-password are already rate limited.

## What Changes

- Login counts attempts per client IP (60 attempts of any outcome per 15 minutes) and per submitted address (10 failed attempts per 15 minutes); over a limit it answers 429 `rate_limited` with `retry_after_seconds` and `Retry-After`, the same shape as the other limited auth endpoints.
- Limits are checked before the account lookup and are keyed only on request data, so a 429 never reveals whether the account exists.
- Throttling, not lockout: the block lifts by itself when the 15-minute window ends; a correct password (including the 403 `email_not_verified` case) clears the address counter but still counts against the IP (otherwise an attacker with an account of their own could alternate victim attempts with own logins and keep the IP counter flat).
- A successful password reset clears the failed-login counter of the account's address (the victim's way out of a throttle caused by someone else).
- If the limit store (Redis) is unavailable, login keeps working without limits (fail-open, logged) — deliberately unlike the email-sending endpoints, which fail closed.
- Web and admin login forms show «Слишком много попыток входа. Попробуйте снова через N мин.» on 429.

## Capabilities

### New Capabilities

### Modified Capabilities

- `auth/session`: new login rate-limit requirement and the 429 message on both login pages.
- `auth/password-reset`: a successful reset also clears the address's failed-login counter.

## Impact

- backend: ratelimit package (release/reset of a counter), auth service and login handler, api wiring in `cmd/api`, Swagger.
- frontend: web and admin session stores (429 message); admin gets a copy of the retry-after formatter.
- devops: none (reuses `REDIS_URL`; new `rl:login-*` keys).
- docs: `docs/DEPLOYMENT.md` (Redis keys, client-IP note, launch checklist item).
