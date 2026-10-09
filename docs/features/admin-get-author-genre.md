# Admin: get author / genre by ID

Status: done

Tracking: issue #312 (B2 of epic #310 — admin catalog/import/moderation unification).

## Goal

Give the admin panel single-record getters for authors and genres, mirroring
`GET /admin/books/:id`, so later edit pages (F3, #316) can load a record by ID
instead of scanning the paginated list.

## User Stories

- As an admin, I can fetch one author or genre by ID, in any status, with the
  same fields the admin list returns, to prefill an edit form.

## Acceptance Criteria

- [x] `GET /api/v1/admin/authors/:id` returns 200 with `authorResponse` (same struct/mapper as list items: `toAuthorResponse`).
- [x] `GET /api/v1/admin/genres/:id` returns 200 with `genreResponse` (same struct/mapper as list items: `toGenreResponse`).
- [x] Records in `pending`, `published` and `rejected` status are all returned.
- [x] Invalid UUID -> 400 `{"error":"invalid author id"}` / `{"error":"invalid genre id"}`.
- [x] Unknown ID -> 404 `{"error":"author not found"}` / `{"error":"genre not found"}`.
- [x] Genre alias ID (`merged_into_id` set) -> 404 (see Business Rules).
- [x] Other repo errors -> 500 `{"error":"failed to get author"}` / `"failed to get genre"`.
- [x] Routes are admin-role only (moderator/user -> 403, anonymous -> 401); no `requireBearer` (read-only, same as `books.GET("/:id")`).
- [x] Static routes `/authors/moderation`, `/genres/moderation`, `/genres/wikidata-search` still resolve to their own handlers.
- [x] `api.admin.authors.get(id)` / `api.admin.genres.get(id)` in api-client; exposed as `adminAuthorsApi.get` / `adminGenresApi.get`.
- [x] Swagger annotations added and `backend/docs` regenerated.

## Business Rules

- Any status is returned (admin view; no status filter).
- **Merged author**: `MergeAuthors` deletes the duplicate row, so a merged
  author ID simply does not exist -> 404. No merge pointer exists to expose.
- **Genre alias**: alias rows are hidden from `ListGenres` and from the repo's
  `GetGenreByID` (`id = ? AND merged_into_id IS NULL`); `genreResponse` has no
  merge-pointer field. Follow existing semantics: alias ID -> 404. Do not add
  `merged_into_id` to the response and do not add a new repo method.
- No new error values: reuse `ErrAuthorNotFound` / `ErrGenreNotFound`.

## API

Router (`backend/internal/transport/http/router.go`, inside the existing
`authors` / `genres` groups that already carry `adminRole`):

```
authors.GET("/:id", adminHandler.GetAuthor)   // place after the static GETs, next to PATCH /:id
genres.GET("/:id", adminHandler.GetGenre)     // same
```

Echo's router gives static segments priority over `:param` regardless of
registration order, so `/moderation` and `/wikidata-search` are not shadowed;
still register `/:id` after the static GETs for readability (as `books` does
with `/lookup` before `/:id`).

Service (`backend/internal/modules/admin/service/service.go`), mirroring `GetBook`:

```go
func (s *Service) GetAuthor(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) // repo.GetAuthorByID; gorm.ErrRecordNotFound -> ErrAuthorNotFound; else wrap "get author: %w"
func (s *Service) GetGenre(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error)   // repo.GetGenreByID;  gorm.ErrRecordNotFound -> ErrGenreNotFound;  else wrap "get genre: %w"
```

Add both to the `Service` interface in `transport/http/handler.go` (next to
`GetBook`). Handlers `GetAuthor` / `GetGenre` copy `GetBook` (`parseID` ->
400, not-found -> 404, else 500, success -> `toAuthorResponse` /
`toGenreResponse`). Swagger: `@Tags admin`, `@Security BearerAuth`,
`@Param id path string true`, `@Success 200 {object} authorResponse|genreResponse`,
failures 400/401/403/404/500, `@Router /admin/authors/{id} [get]` /
`/admin/genres/{id} [get]`.

No repository changes: `GetAuthorByID` and `GetGenreByID` already exist.

## Database

None.

## Frontend

- `packages/api-client/src/index.ts`:
  - `admin.authors.get: (id: string) => fetcher<AdminAuthor>(\`/api/v1/admin/authors/${id}\`)`
  - `admin.genres.get: (id: string) => fetcher<AdminGenre>(\`/api/v1/admin/genres/${id}\`)`
  - Verify `AdminAuthor` / `AdminGenre` types already match `authorResponse` / `genreResponse` (they are the list item types; no change expected).
- `apps/admin/src/entities/author/api/index.ts`: add `get: api.admin.authors.get`.
- `apps/admin/src/entities/genre/api/index.ts`: add `get: api.admin.genres.get`.
- No UI/pages (out of scope; F3 #316).

## Existing Patterns

- Handler: `GetBook` in `backend/internal/modules/admin/transport/http/handler.go`.
- Service: `GetBook` in `backend/internal/modules/admin/service/service.go`.
- Fake service in `transport/http/handler_test.go` (`fakeAdminService`) — add `GetAuthor` / `GetGenre` methods with configurable result/error.
- Service fake repo in `service/service_test.go` (`fakeRepository`) already implements `GetAuthorByID` / `GetGenreByID`.
- Entity API re-export: `apps/admin/src/entities/book/api/index.ts`.

## Tests

Backend (handler, `transport/http`):
- `GetAuthor`: 200 (body has `id`, `status`, `external_source`/`external_id`), 200 for a `pending`/`rejected` record, 400 invalid UUID (service not called), 404 on `ErrAuthorNotFound`, 500 on other error.
- `GetGenre`: same set; 200 body includes `status` and `wikidata_qid`.
- Role: handlers do not check role themselves (enforced by `RequireRole` on
  the group, covered by `middleware/auth_test.go`); there is no router test
  package. Do not add a router harness just for this — note route
  registration in review instead.

Backend (service):
- `GetAuthor` / `GetGenre`: found; `gorm.ErrRecordNotFound` -> `ErrAuthorNotFound` / `ErrGenreNotFound`; other error wrapped (not the not-found sentinel).
- Optional (if a sqlite repo fixture is handy, e.g. `genre_moderation_sqlite_test.go`): alias genre ID via service -> `ErrGenreNotFound`. Existing repo semantics already guarantee this.

Frontend: none required (thin re-exports); `tsc`/lint must pass.

Commands (Docker only; format only changed files, never `gofmt -w` the tree):
`docker compose exec backend go test ./internal/modules/admin/...`,
`docker compose exec backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal` (see `Makefile`).

## Documentation

- This contract -> `Status: done` on completion.
- Regenerated swagger files in `backend/docs`.

## Risks

- Swagger regeneration on Windows/Docker may touch line endings in unrelated generated files — commit only the expected diff.
- A future need to show alias genres in the admin (e.g., "merged into X") would need a new repo method and a response field; intentionally not done here.
