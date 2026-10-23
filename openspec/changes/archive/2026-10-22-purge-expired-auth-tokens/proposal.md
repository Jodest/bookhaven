# Proposal

Issue: #244

## Why

Rows in `refresh_tokens`, `email_verification_tokens` and `password_reset_tokens` are never deleted after expiry, use, logout or reset revocation, so the tables grow without bound. A daily worker purge with explicit retention windows bounds them without changing how live tokens behave.

## What Changes

- A daily background purge in the worker deletes, in bounded batches:
  - refresh tokens revoked more than 7 days ago or expired more than 7 days ago (no observable change: such tokens already answer 401);
  - email verification and password reset tokens whose expiry is more than 30 days in the past (used or not).
- Retention is now part of the auth contract: during the 30-day window a used/expired link keeps answering `already_verified` / `token_used` / `token_expired` exactly as today; after it the link is unknown and answers `invalid_token` («Ссылка недействительна» on the page). **BREAKING (minor)**: previously a used or expired link kept its specific answer forever.
- An additive migration adds indexes on the expiry/revocation columns the purge filters on.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `auth/session`: adds the refresh-token retention rule (purged tokens answer 401 as before).
- `auth/email-verification`: verify endpoint's "already confirmed even for a used or expired token" now holds while the token is retained; adds the verification-token retention rule.
- `auth/password-reset`: adds the reset-token retention rule.

## Impact

- backend: auth module purge job + asynq task, new always-on `auth_maintenance` server and schedule in `cmd/worker`, migration 000038 (indexes only).
- frontend: none (purged links fall into the existing «Ссылка недействительна» state).
- devops: one more queue/server in the existing worker container; single-replica rule unchanged.
- docs: `docs/DEPLOYMENT.md` "Background jobs (worker)", `docs/ROADMAP.md` (#244).
