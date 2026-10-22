# Design

## Context

`golang.org/x/crypto/bcrypt.GenerateFromPassword` returns `bcrypt.ErrPasswordTooLong` for input over 72 bytes. The only call sites that hash a user-supplied password are in `backend/internal/modules/auth/service/service.go`:

- `Service.Register` (line ~154), reached from `POST /auth/register` (`registerRequest`, `validate:"required,min=8"`);
- `Service.ResetPassword` (line ~645), reached from `POST /auth/reset-password` (`resetPasswordRequest`, `validate:"required,min=8"`).

Both wrap the bcrypt error as a generic error and the handlers answer 500. Verified absent: no change-password endpoint, no admin user creation (`admin` module only lists users and changes roles; `users` module never hashes). `cmd/seed` hashes fixed constants - out of scope. `Login` uses `CompareHashAndPassword`, which does not error on long input; no stored hash can derive from a >72-byte password, so a long login password just fails as 401 - unchanged.

## Decisions

1. **Bytes, not runes.** go-playground `max=72` on a string counts runes (`utf8.RuneCountInString`), so a 72-letter Cyrillic password (144 bytes) would pass and still 500. Do not use `max=72`. The limit is `len(password) > 72` (Go `len` on a string = UTF-8 bytes).
2. **Guard lives in the auth service, not a new validator tag.** Add to `auth/service` (next to the other sentinel errors):
   - `const MaxPasswordBytes = 72`
   - `var ErrPasswordTooLong = errors.New("password is too long")`
   - `Register`: return `ErrPasswordTooLong` as the very first step (before the existing-email lookup, so a long password never yields 409 vs 400 ambiguity and nothing is created).
   - `ResetPassword`: return `ErrPasswordTooLong` as the very first step, before `isTokenFormat` / the token lookup, so the token is never read or burned (the hashing already happens before the transaction, but the early return keeps "password checked first" explicit).
   Rationale: one rule, enforced where bcrypt is called, so any future caller is covered; no change to the shared `infrastructure/validator` used by every module. A custom `maxbytes` tag was considered and rejected as more surface for one field.
3. **Handler mapping** (`auth/transport/http/handler.go`), keeping each endpoint's existing 400 shape:
   - Register: `errors.Is(err, service.ErrPasswordTooLong)` -> 400 `{"error": "password must be at most 72 bytes"}` (same `map[string]string{"error": ...}` shape as the current validation 400).
   - ResetPassword: -> 400 `errorResponse{Error: "password must be at most 72 bytes", Code: codeInvalidPassword}`.
4. **Frontend uses the same byte rule.** HTML `maxLength` counts UTF-16 code units, not bytes, so it is **not** used (it would either cut a valid Latin password or let a Cyrillic one through). Zod `.refine` with `new TextEncoder().encode(value).length <= 72`.

## API

No new endpoints, no new error codes, no auth change (both endpoints stay public).

- `POST /api/v1/auth/register`: new 400 case `{"error":"password must be at most 72 bytes"}`.
- `POST /api/v1/auth/reset-password`: new 400 case `{"error":"password must be at most 72 bytes","code":"invalid_password"}`.

Update the Swagger `@Description` of both handlers to mention "8 символов - 72 байта"; regenerate swagger only if the repo commits generated docs (follow the existing practice).

## Frontend

apps/web only (apps/admin has no form that sets a password).

- New `apps/web/src/shared/lib/password-rules.ts` (+ `.test.ts`): `PASSWORD_MAX_BYTES = 72`, `passwordByteLength(value)`, `isPasswordWithinByteLimit(value)`, `PASSWORD_MAX_MESSAGE = 'Пароль слишком длинный: до 72 латинских или 36 русских букв'`. Shared layer because both `features/auth` and `features/reset-password` need it (FSD: no cross-feature import).
- `features/auth/ui/register-form.tsx`: `password: z.string().min(8, ...).refine(isPasswordWithinByteLimit, PASSWORD_MAX_MESSAGE)`.
- `features/reset-password/ui/reset-password-form.tsx`: same refine on `password`. The fallback for a server-side `invalid_password` (line ~46) keeps showing `PASSWORD_MIN_MESSAGE`: the form only knows the state, not which rule failed, and the client rule makes the too-long case unreachable in practice. Leave it unchanged.

## Existing Patterns

- Sentinel errors + `errors.Is` mapping in `auth/transport/http/handler.go` (`ErrUserExists`, `ErrResetTokenUsed`).
- Handler tests via `postJSON` in `auth/transport/http/verification_test.go`; service tests in `auth/service/password_reset_test.go` (`passwordMatches`).
- Zod form tests: `apps/web/src/views/reset-password/ui/reset-password-view.test.tsx` ("short password -> field error, no API call").

## Risks

- Existing accounts: none can have a >72-byte password (hashing always failed), so no migration or login change is needed.
- Exactly-72-byte boundary must be accepted on both sides; tests cover 72 and 73 bytes and a Cyrillic case where rune count < 72 but bytes > 72.
- Register 400 bodies are shown raw by the web auth store; with the client rule in place users never see the English text.
