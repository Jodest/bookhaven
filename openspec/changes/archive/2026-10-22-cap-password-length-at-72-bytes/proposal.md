# Proposal

## Why

bcrypt cannot hash a password longer than 72 bytes, and neither registration nor password reset limits the password length, so such a password returns 500 instead of a validation error (issue #192). Cyrillic letters take 2 bytes each, so a 37-letter Russian password already hits the limit.

## What Changes

- `POST /api/v1/auth/register` answers 400 (no account created) for a password longer than 72 bytes in UTF-8.
- `POST /api/v1/auth/reset-password` answers 400 `invalid_password` for such a password, before the token is checked, so the link is not used up.
- `/register` and `/reset-password` validate the same byte limit client-side and show «Пароль слишком длинный: до 72 латинских или 36 русских букв».
- Login is unchanged (no stored hash can come from a password over 72 bytes; a longer login attempt is simply a 401).
- No other code path hashes a user-supplied password (no change-password, no admin-created users; the seed command is out of scope).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `auth/registration`: password maximum of 72 bytes on the endpoint and the page.
- `auth/password-reset`: password maximum of 72 bytes on the reset endpoint and the reset page.

## Impact

- backend: auth module (service guard + handler mapping), tests.
- frontend: apps/web register form and reset-password form, shared password rule helper, tests. apps/admin has no password-setting form.
- devops: none.
- docs: Swagger annotations of the two endpoints; follow-up note in `docs/features/password-reset.md` marked done.
