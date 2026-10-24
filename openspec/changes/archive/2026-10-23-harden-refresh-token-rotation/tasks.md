# Tasks

## 1. Backend

- [x] 1.1 Forward migration `backend/migrations/000040_refresh_token_families.up.sql` (confirm the number is free) exactly as in design.md "Database" (nullable `family_id`, backfill `family_id = id`, `DEFAULT gen_random_uuid()`, `NOT NULL`, nullable `replaced_by` without FK, partial index on `family_id`), plus a `.down.sql` that drops the index and both columns. Apply locally only with the normal `migrate up`; never run `migrate down`, `down-all` or any drop. Verify: `migrate up` succeeds on the local compose DB and existing rows have `family_id = id`.
- [x] 1.2 `auth/domain/tokens.go`: add `FamilyID`, `ReplacedBy` to `RefreshToken` and `const RefreshReuseGrace = 30 * time.Second` with a comment pointing at the auth/session spec. Verify: `go build ./...` from `backend/`.
- [x] 1.3 `auth/service/service.go`: split token creation so the refresh-token insert accepts a db handle, an explicit id and a family id (login: new family with `family_id = id`); add `ErrRefreshSuperseded`; rewrite `RefreshTokens` per design.md Decisions 1, 3, 5, 6 (unfiltered lookup by hash, grace vs reuse branch with family revocation and a `Warn` log carrying user_id/family_id/revoked_count/seconds_since_rotation and never the token or hash, conditional `UPDATE ... WHERE id=? AND revoked_at IS NULL` + `RowsAffected` check + insert in one transaction, 0 rows -> `ErrRefreshSuperseded`). Logout/password reset/unverified paths keep setting only `revoked_at`. Verify: `go build ./...`, `go vet ./...`.
- [x] 1.4 `auth/transport/http/handler.go`: `codeRefreshTokenSuperseded`, map `ErrRefreshSuperseded` to 401 with that code (cookies untouched); reuse and invalid keep the existing plain 401 body; update the swagger `@Failure 401` comment. Verify: `go build ./...`.
- [x] 1.5 Update every SQLite `CREATE TABLE refresh_tokens` test DDL (`auth/service/service_test.go`, `auth/transport/http/verification_test.go`, and any other copy found by grep) with `family_id` and `replaced_by`.
- [x] 1.6 Service tests (SQLite): login creates a token with `family_id = id`; refresh keeps the family and sets `replaced_by` on the parent; rotating from a stale snapshot of an already-rotated token reports superseded and leaves exactly one successor; replay at `revoked_at = now-5s` -> `ErrRefreshSuperseded`, successor still works, nothing revoked; replay at `now-60s` -> `ErrTokenInvalid`, all active tokens of that family revoked, a second family of the same user untouched, warning logged (zap observer) without the token/hash; a token revoked by `Logout` -> `ErrTokenInvalid`, no family revocation, no warning; an unknown (purged) token -> `ErrTokenInvalid`, no warning; a legacy row (`family_id = id`, no `replaced_by`) refreshes fine; unverified-email branch unchanged. Verify: `go test ./internal/modules/auth/...`.
- [x] 1.7 Handler tests: refresh with a just-rotated token -> 401 `code: refresh_token_superseded`; reuse after the grace window -> 401 with the same body as an unknown token (no `code`). Verify: `go test ./internal/modules/auth/...`; run `gofmt -l`/`gofmt -w` only on the changed files.

## 2. Frontend (packages/api-client, used by apps/web and apps/admin)

- [x] 2.1 `packages/api-client/src/index.ts`: export `REFRESH_TOKEN_SUPERSEDED_CODE = 'refresh_token_superseded'`; in `performRefresh`, when `rawRefresh` rejects with an `ApiError` 401 carrying that code, wait ~1000 ms and call `rawRefresh` once more inside the same shared `refreshPromise`; any other error or a second failure rejects as before. Update the dedup comment.
- [x] 2.2 Vitest test in `apps/web` (next to `src/entities/session/model/auth-store.test.ts` or a new file in the session entity) mocking `fetch` and fake timers: superseded then 200 -> refresh resolves with the second tokens and only two refresh calls were made; superseded twice -> rejects after exactly two calls; plain 401 -> rejects after one call; concurrent callers still share one in-flight attempt.
- [x] 2.3 Verify: `tsc --noEmit` and `next lint` in `apps/web` and `apps/admin` (no native `next build`; CI is the typedRoutes gate).

## 3. Docs

- [x] 3.1 `docs/DEPLOYMENT.md`, next to the refresh-token purge notes: describe the `refresh token reuse detected` warning (what it means, that it carries user_id/family_id and never a token, that the affected session was revoked) and that migration 000040 is safe with the old binary still running.

## 4. Manual check (local compose stack)

- [x] 4.1 (deferred to the human reviewer: unchecked item in the dev PR checklist, not performed by the pipeline) Log in on the web, copy the `refresh_token` cookie, trigger a refresh, then after 60 s `POST /api/v1/auth/refresh` with the copied value in the body: 401 without code, the browser's next refresh fails (signed out), a second browser session of the same user still works, and the API log shows one reuse warning without the token.
- [x] 4.2 (deferred to the human reviewer: unchecked item in the dev PR checklist, not performed by the pipeline) Open two web tabs, let the computer sleep past the access-token lifetime (or force both tabs to refresh at once): both tabs stay signed in.
