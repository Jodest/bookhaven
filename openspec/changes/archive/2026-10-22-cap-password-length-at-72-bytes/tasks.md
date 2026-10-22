# Tasks

## 1. Backend

- [x] 1.1 In the auth service add `MaxPasswordBytes = 72` and `ErrPasswordTooLong`; return it as the first step of `Register` and of `ResetPassword` when `len(password) > MaxPasswordBytes` (bytes, not runes; do not use the validator `max=` tag). Verify: service tests - Register with 73 ASCII bytes and with 40 Cyrillic letters returns `ErrPasswordTooLong` and creates no user; Register with 72 ASCII bytes and with 36 Cyrillic letters succeeds and the hash matches; ResetPassword with 40 Cyrillic letters on a valid token returns `ErrPasswordTooLong`, the token stays unused and the old password still matches; 36 Cyrillic letters succeeds.
- [x] 1.2 In the auth handler map `ErrPasswordTooLong`: Register -> 400 `{"error":"password must be at most 72 bytes"}`; ResetPassword -> 400 `errorResponse{code: "invalid_password"}`. Update both Swagger descriptions. Verify: handler tests via the existing `postJSON` helper assert 400 (and the `invalid_password` code for reset), never 500.
- [x] 1.3 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./internal/modules/auth/...`. gofmt only the changed files. No DB changes, no migrations.

## 2. Frontend (apps/web)

- [x] 2.1 Add `src/shared/lib/password-rules.ts` (`PASSWORD_MAX_BYTES`, byte-length check via `TextEncoder`, `PASSWORD_MAX_MESSAGE = 'Пароль слишком длинный: до 72 латинских или 36 русских букв'`) with a unit test covering 72/73 ASCII and 36/37 Cyrillic letters.
- [x] 2.2 Add the byte-limit refine to the password field of the register form and the reset-password form (no HTML `maxLength`). Verify: component tests - a 40-Cyrillic-letter password shows the message and does not call the API (register form, and in `reset-password-view.test.tsx` next to the "short password" case).
- [x] 2.3 Verify: `tsc --noEmit` and `next lint` for apps/web, vitest for the touched tests. No native `next build`.

## 3. Docs

- [x] 3.1 In `docs/features/password-reset.md` mark the "bcrypt 72-byte limit" follow-up as resolved by this change (one line; the file is frozen history otherwise).
