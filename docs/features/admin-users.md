# Admin Users Management

Status: implemented

Issue: #289 (phase 6 of epic #283 "Admin navigation restructure", see [admin-navigation-restructure.md](admin-navigation-restructure.md)). Covers the "list users" and "change role" parts of proposal #247; "block account" is deferred (see Out of Scope).

## Goal

Give admins a `/users` page in `apps/admin` to find users and change their role (user / contributor / moderator / admin) without editing the database, while guaranteeing that the system can never lose its last admin and an admin cannot change their own role.

## User Stories

- As an admin, I search users by email or name and filter by role, so I can find the account I need.
- As an admin, I change a user's role (e.g. promote to moderator) after a confirmation, and see the result in the table immediately.
- As an admin, I cannot accidentally demote myself or leave the system without an admin; I get a clear message if I try.

## Acceptance Criteria

Backend
- [x] `GET /api/v1/admin/users` (admin only) returns `{ items, total, page, limit }` with `adminUserResponse` items; `search` matches email OR name (case-insensitive substring), `role` filters by exact role, `page`/`limit` follow `parsePagination` (default 20, max 100); order `created_at DESC, id DESC`.
- [x] Invalid `role` filter -> 400 `{"error": "...", "code": "invalid_role"}`.
- [x] `PATCH /api/v1/admin/users/:id/role` (admin only + `requireBearer`) with body `{"role": "<role>"}` returns 200 `adminUserResponse` with the new role.
- [x] Invalid id -> 400 `invalid user id`; unknown/missing role value -> 400 code `invalid_role`; user not found -> 404 code `user_not_found`.
- [x] Changing own role (actor id == target id) -> 409 code `cannot_change_own_role`, no write.
- [x] Demoting the last admin -> 409 code `last_admin`, no write (invariant check, see Business Rules).
- [x] Actor no longer an admin by the time the transaction runs (concurrently demoted) -> 403 `forbidden`, no write.
- [x] Setting the role the user already has -> 200, no write, no log line.
- [x] Each successful change emits one structured info log line (actor id, target id, old role, new role).
- [x] `password_hash` never appears in any response.
- [x] `users.UpdateMe` no longer writes the `role` column (see Risks: lost update).
- [x] Swagger annotations in Russian; `backend/docs` regenerated (`make swagger`).

Frontend (`apps/admin`)
- [x] `/users` renders `UsersView` instead of `PlaceholderPage` (admin-only via the existing nav config; no config changes).
- [x] Table: name (or "—"), email, role, email verified (yes/no), created date, role control.
- [x] Search input (email/name) + role select ("Все роли" + 4 roles); both synced to URL params (`search`, `role`, `page`), page resets to 1 on change; `Pagination` from `shared/ui`.
- [x] Role control is a select per row; choosing a different role asks `window.confirm` with a text naming the user and both roles; cancel reverts the select, no request.
- [x] Own row: role select disabled with hint "Это вы".
- [x] On success: success banner, `['admin','users']` invalidated.
- [x] Error mapping by `ApiError.code`: `last_admin` -> "Нельзя снять роль с последнего администратора."; `cannot_change_own_role` -> "Нельзя изменить собственную роль."; `user_not_found` -> "Пользователь не найден — обновите список."; `invalid_role` -> "Недопустимая роль."; other -> generic message.
- [x] Loading skeleton, empty state "Пользователи не найдены.", load error "Не удалось загрузить пользователей."
- [x] ESLint boundaries pass (`app` imports only `views`/`shared`).

## Business Rules

1. Only `admin` can list users and change roles (`RequireRole(admin)`); moderators get 403 (they don't see the tab anyway).
2. Valid roles: exactly the `user_role` enum — `user`, `contributor`, `moderator`, `admin`. Add `usersDomain.IsValidRole(string) bool` next to the `Role` constants.
3. An admin cannot change their own role (any value, including the same one). Checked before the transaction.
4. The number of admins must never drop to zero. Inside the transaction: if the target is currently `admin`, the new role is not `admin`, and the locked admin set has <= 1 row -> `last_admin`. Note: with rules 3 and 5 this is unreachable through this endpoint today (the actor is an admin distinct from the target, so there are always >= 2 admins when the target is one); it stays as a defence-in-depth invariant and has its own test.
5. The actor must still be an admin at write time (closes the "two admins demote each other simultaneously" race, see Concurrency).
6. Promotion to `admin` is allowed (admins can create other admins). No email/notification is sent.
7. Role changes take effect on the target's next request: roles are NOT carried in the JWT — `middleware.Auth` loads the user from the DB on every request and `RequireRole` checks `user.Role` from that row (`backend/internal/transport/http/middleware/auth.go`; `auth` service never puts the role into tokens). A demoted admin/moderator gets 403 on the next admin request; no token revocation or refresh-token handling is needed. The admin SPA's cached `user.role` is stale until reload, but any admin API call returns 403 and the existing `forbiddenPath` handling shows the access-denied screen — accepted (server is authoritative, same as the phase 5 risk note).

### Concurrency approach

Single transaction in the admin repository:

```sql
SELECT id, role FROM users WHERE role = 'admin' OR id = $target ORDER BY id FOR UPDATE;
```

(GORM: `tx.Clauses(clause.Locking{Strength: "UPDATE"})`.) One statement, deterministic `ORDER BY id` lock order -> no deadlocks between concurrent role changes. Under PostgreSQL READ COMMITTED, a waiting `FOR UPDATE` re-evaluates the `WHERE` on rows changed by the committed winner, so a row demoted by the other transaction drops out of the admin set. Then: target row missing -> not found; actor not in the admin set -> forbidden; apply rules 4/6; write with a column-scoped update (`Model(&User{}).Where("id = ?", target).Updates(map[string]any{"role": r, "updated_at": now})`), never `Save`. A plain conditional `UPDATE ... WHERE (SELECT count(*) ...) > 1` is NOT sufficient under READ COMMITTED (both subqueries see 2 admins) and must not be used.

## UX

- Page title "Пользователи"; layout mirrors `GenresView` (search form, table, pagination).
- Role labels (RU): user — "Читатель", contributor — "Автор контента", moderator — "Модератор", admin — "Администратор". Keep the labels in `entities/user` so filter and row control share them.
- Confirm text: `Изменить роль пользователя «<name или email>»: <старая> → <новая>?`; for promotion to admin append " Пользователь получит полный доступ к админ-панели."
- While the mutation for a row is pending, that row's select is disabled.

## API

`GET /api/v1/admin/users?search=&role=&page=&limit=` — admin, cookie or bearer (reads follow existing admin GETs).

`PATCH /api/v1/admin/users/:id/role` — admin + bearer. Request `{"role": "moderator"}`.

```go
type adminUserResponse struct {
    ID              string     `json:"id"`
    Email           string     `json:"email"`
    Name            *string    `json:"name"`
    AvatarURL       *string    `json:"avatar_url"`
    Role            string     `json:"role"`
    ProfileType     string     `json:"profile_type"`
    EmailVerifiedAt *time.Time `json:"email_verified_at"`
    CreatedAt       time.Time  `json:"created_at"`
}
type listUsersResponse struct { Items []adminUserResponse; Total int64; Page, Limit int } // same json tags as listGenresResponse
type changeUserRoleRequest struct { Role string `json:"role"` }
```

Errors use the existing `{"error": "...", "code": "..."}` shape (as in `ImportWikidataAuthor`). Guard violations deliberately use 409, not 403: in the admin SPA any 403 on `/api/v1/admin/*` (including mutations) replaces the page with the access-denied screen (`entities/session/lib/query-client.ts`), which must happen only for a real loss of privileges.

Routes in `RegisterAdminRoutes` (`backend/internal/transport/http/router.go`):

```go
users := admin.Group("/users", adminRole)
users.GET("", adminHandler.ListUsers)
users.PATCH("/:id/role", adminHandler.ChangeUserRole, requireBearer)
```

Actor id: `c.Get("user").(*usersDomain.User)` set by `middleware.Auth` (401 if absent).

## Database

No migration. `users` already has `role user_role` and all response columns (`000001_initial.up.sql`). Search uses `email ILIKE ? OR name ILIKE ?` (same style as `ListGenres`) — a seq scan is fine at current user counts; a `pg_trgm` index is a follow-up only if it becomes slow. Agents must never run destructive DB commands (`migrate down`, dropping/truncating tables); nothing in this feature needs them.

## Backend placement

Lives in the **admin module**, not `users`. Rationale: the admin module is the established home for admin-only operations over other modules' tables — it already reads/writes catalog and reviews data through its own repository using their domain types (`catalogDomain`, `reviewsDomain`), is the single `adminhttp.Handler` behind `RegisterAdminRoutes`, and has the `adminRole`/`requireBearer` wiring and zap logger. Putting this in `users` would require a second admin handler or passing `usersService` into `RegisterAdminRoutes`. The admin module reuses `usersDomain.User`/`Role` (no duplicate model).

Files:
- `backend/internal/modules/users/domain/user.go`: `IsValidRole`.
- `backend/internal/modules/admin/repository/users.go`: `UserFilter{Search, Role string; Limit, Offset int}`; add to `Repository`:
  - `ListUsers(ctx, UserFilter) ([]usersDomain.User, int64, error)`
  - `ChangeUserRole(ctx, targetID uuid.UUID, decide func(target usersDomain.User, adminIDs []uuid.UUID) (usersDomain.Role, error)) (*usersDomain.User, usersDomain.Role /*old*/, error)` — runs the locked transaction above, calls `decide` (service-owned rules), skips the write when the returned role equals the current one, returns `gorm.ErrRecordNotFound` for a missing target, and rolls back on any `decide` error.
- `backend/internal/modules/admin/service/users.go`: errors `ErrUserNotFound`, `ErrInvalidRole`, `ErrCannotChangeOwnRole`, `ErrLastAdmin`, `ErrActorNotAdmin`; `ListUsers(ctx, search, role string, page, limit int)`; `ChangeUserRole(ctx, actorID, targetID uuid.UUID, role string) (*usersDomain.User, error)` — validate role, self-check, call repo with `decide` implementing rules 4-5, log `s.log.Info("admin changed user role", zap.String("actor_id"...), zap.String("target_id"...), zap.String("old_role"...), zap.String("new_role"...))` only when changed.
- `backend/internal/modules/admin/transport/http/handler_users.go`: `ListUsers`, `ChangeUserRole`, DTOs, error mapping (`ErrActorNotAdmin` -> 403 `{"error":"forbidden"}`); extend the handler `Service` interface.
- `backend/internal/transport/http/router.go`: routes above.
- `backend/internal/modules/users/service/service.go` + `repository.go`: make `UpdateMe` persist only `name`, `avatar_url`, `profile_type`, `updated_at` (e.g. new `UpdateProfile` repo method with `Select(...)`/`Updates`), so a concurrent profile save can't revert a role change. Keep `Update` if other callers use it, but `UpdateMe` must not call `Save`.

## Frontend

`packages/api-client/src/index.ts`:
- `export type AdminUserRole = User['role']`; `export interface AdminUser { id; email; name: string | null; avatar_url: string | null; role: AdminUserRole; profile_type: User['profileType']; email_verified_at: string | null; created_at: string }` (snake_case like other `Admin*` types).
- `api.admin.users.list(params?: { page?; limit?; search?; role? }) => PaginatedResponse<AdminUser>`; `api.admin.users.changeRole(id, role) => AdminUser` (PATCH, body `{ role }`; bearer is attached automatically for `/api/v1/admin` mutations). Comment the error codes like `importWikidata` does.

`apps/admin/src`:
- `entities/user/` (`index.ts`, `api/index.ts`, `lib/roles.ts`): `adminUsersApi = { list, changeRole }`, `ADMIN_USERS_QUERY_KEY = ['admin','users'] as const`, `USER_ROLES` ordered list, `userRoleLabel(role)`. Name is `user` (admin-side); no clash — `apps/admin` has no `entities/user` today.
- `features/change-user-role/` (`index.ts`, `model/use-change-user-role.ts`, `lib/messages.ts`, `ui/user-role-select.tsx`): mutation hook (invalidates `ADMIN_USERS_QUERY_KEY` on success), `changeRoleConfirmText(user, from, to)`, `changeRoleErrorText(error)` (maps `ApiError.code`), `UserRoleSelect({ user, disabled, onChanged, onError })` — owns confirm + mutate, reverts on cancel.
- `views/users/` (`index.ts`, `ui/users-view.tsx`): list query keyed `[...ADMIN_USERS_QUERY_KEY, { search, role, page }]`, search/role/URL params like `GenresView`, own-row detection via `useAuthStore` from `@/entities/session` (`user.id === row.id`), banners.
- `app/users/page.tsx`: render `UsersView` (keep `PlaceholderPage` in `shared/ui` — still used by other placeholders).
- FSD: `features/change-user-role` imports `entities/user` only; `views/users` imports `entities/user`, `entities/session`, `features/change-user-role`, `shared`; `app` imports `views/users` only.

## Existing Patterns

- Admin list handler + pagination: `ListAuthors`/`ListGenres` and `parsePagination` in `backend/internal/modules/admin/transport/http/handler.go`; repo `ListGenres` in `admin/repository/repository.go`.
- Error code responses: `ImportWikidataAuthor` handler (`"code": "no_ru_label"`).
- Transactions: `admin/repository/repository.go` (`MergeAuthors`).
- Service test fakes: extend base `fakeRepository` (`admin/service/service_test.go`) with stub methods and embed it in a new `fakeUsersRepository` (pattern: `author_moderation_test.go`).
- SQLite repository tests: `admin/repository/author_moderation_sqlite_test.go` (the GORM sqlite dialect drops `FOR UPDATE`, so locking itself is not exercised there).
- Handler tests: `fakeAdminService` in `admin/transport/http/handler_test.go` (add the two methods); code assertions like `handler_author_import_test.go`.
- Frontend: `views/genres/ui/genres-view.tsx` (search/URL/pagination), `features/moderate-author` (mutation hook + confirm text lib), `views/authors/ui/authors-view.test.tsx` (`next/navigation` mocking).

## Tests

Backend
- `admin/service/users_test.go`: invalid role (no repo call); self-change (no repo call); target not found -> `ErrUserNotFound`; actor not in admin set -> `ErrActorNotAdmin`; last admin demotion -> `ErrLastAdmin`; promote user -> moderator ok; demote admin when 2 admins ok; same role -> no write; list passes search/role/limit/offset and validates role filter.
- `admin/repository/users_sqlite_test.go`: list search (email and name, case-insensitive), role filter, ordering, total; `ChangeUserRole` writes only `role`/`updated_at`, passes the correct admin set to `decide`, rolls back on `decide` error, not found.
- `admin/transport/http/handler_users_test.go`: list 200 envelope + no `password_hash` key; bad role filter 400; change role 200; invalid uuid 400; error mapping (`invalid_role` 400, `user_not_found` 404, `cannot_change_own_role` 409, `last_admin` 409, actor-not-admin 403); missing `user` in context 401.
- `users/service/service_test.go`: `UpdateMe` does not touch `role` (fake asserts the profile-only update method is used).
- Concurrency is argued, not tested (no Postgres in unit tests); optional manual check in the PR description.

Frontend
- `features/change-user-role/lib/messages.test.ts`: confirm text (name fallback to email, admin promotion suffix); error code mapping incl. unknown.
- `features/change-user-role/ui/user-role-select.test.tsx`: cancel confirm -> no call, value reverted; confirm -> `changeRole(id, role)` called; disabled prop respected; `last_admin` error surfaced via `onError`.
- `views/users/ui/users-view.test.tsx`: renders rows; own row select disabled; search/role update URL params with page reset; empty and error states; success invalidates `['admin','users']`.
- CI frontend build (typedRoutes) is the gate — do not run a native `next build` on Windows.

## Documentation

- This contract: status updates as phases land.
- `docs/features/admin-navigation-restructure.md` phase 6 points here.
- `docs/ROADMAP.md`: on merge, move #289 to Completed (or fold into the admin navigation entry) and annotate #247 as partially done (block account remaining).
- PR body: `Closes #289`; note #247 stays open for blocking.

## Implementation Notes

- Search uses `LOWER(email) LIKE ? OR LOWER(name) LIKE ?` (lowercased pattern) instead of `ILIKE` so the SQLite repository tests run the same query; semantics are identical in PostgreSQL (case-insensitive substring, Unicode-aware `lower()`).
- The repository `decide` callback is the named type `repository.RoleDecider`. Locked rows come back `ORDER BY id`, so `adminIDs` is id-ordered.
- `users` repository: `Update` (whole-row `Save`) was replaced by `UpdateProfile`, which updates only `name`, `avatar_url`, `profile_type`, `updated_at` (it had no other callers). After this change no code path `Save`s a `User` (auth writes `email_verified_at` / `password_hash` with column-scoped raw `UPDATE`s).
- Frontend: the list query reads `search`/`role`/`page` from the URL (the search input is local state applied on submit). `UserRoleSelect` keeps the confirmed role shown until the refetched row arrives and reverts it on error; `onError` receives the already-mapped message. A success banner names the user and both roles.

## Out of Scope (follow-ups)

- **Block/deactivate account** (#247 remainder): needs a schema change (e.g. `blocked_at`/`is_active` on `users`), enforcement in `middleware.Auth`/`OptionalAuth`, login and refresh paths, revoking refresh tokens, and UX for blocked users on `apps/web`. It is an auth-surface change with its own security review — keep #247 open for it.
- Editing user profile fields, deleting users, audit table (a structured log line is enough for now), trigram search index.

## Risks

- **Lost update via `Save`** (real today): `users.UpdateMe` loads the whole row and `Save`s it, so a profile edit racing an admin role change could silently restore the old role. Fixed in this feature by column-scoped updates; reviewers should check no other path `Save`s a `User`.
- **Lock scope**: `FOR UPDATE` locks every admin row for the duration of a short transaction; acceptable (few admins, rare operation). Keep the transaction minimal (no external calls inside `decide`).
- **403 handling in the SPA**: if a guard is ever mapped to 403, the admin is kicked to the access-denied screen. Guards must stay 409/400; only a genuinely demoted actor gets 403.
- **Stale SPA role**: a demoted admin keeps seeing admin navigation until reload; every request is rejected server-side — accepted.
- **Last-admin guard is unreachable today** given self-change protection; it is still required and tested so that future callers (e.g. bulk role changes, CLI) cannot break the invariant.
- **Search wildcards**: `%`/`_` in the search string act as LIKE wildcards (same as existing admin lists) — harmless for an admin-only endpoint.
- **Interface growth**: adding methods to admin `Repository` and handler `Service` requires stubbing them in the shared fakes (`fakeRepository`, `fakeAdminService`) or compilation of all admin tests fails.
