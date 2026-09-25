# Catalog Manual Import (Lookup Assistant + File Import)

Status: implemented

This is the "Способ B" increment of the `catalog-data-import` initiative (see `docs/ai/features/catalog-data-import/business-analysis.md`, Решение 2/4, FR-002–FR-009, FR-012, NFR-002/003). The offline harvester (`cmd/harvest`), the `bookdata` clients, and the admin review queue (`docs/features/bookdata-clients.md`, `docs/features/async-harvest-queue.md`, `docs/features/harvest-review-queue.md`) are already implemented and are **not** touched by this contract. This contract covers two independently shippable, admin-facing asks that both reuse `admin/service` rather than duplicating its validation/creation logic (NFR-001):

1. **Lookup assistant** — `GET /api/v1/admin/books/lookup`, a synchronous single-record Open Library lookup that pre-fills the existing "create book" form.
2. **Bulk file import** — `POST /api/v1/admin/books/import`, a CSV/JSON batch upload that creates books row-by-row with partial success, reusing `admin.Service.CreateBook` and the existing `ResolveOrCreateAuthorIDsByNames`/`ResolveOrCreateGenreIDsByNames` helpers (already built for the harvest handler).

## Scope boundaries (read first)

**In scope:**
- Backend: `GET /api/v1/admin/books/lookup?isbn=&q=&author=` (RoleAdmin).
- Backend: `POST /api/v1/admin/books/import` (RoleAdmin), CSV or JSON body, per-row independent processing, JSON report.
- Frontend (`apps/admin`): a "Найти по ISBN/названию" action in `book-form.tsx` that calls the lookup endpoint and pre-fills text/number fields only.
- Frontend (`apps/admin`): a new `/books/import` page with file picker, submit, and a report table.
- `packages/api-client`: typed `adminBooksApi.lookup`/`adminBooksApi.import` calls and a small, backward-compatible `fetcher` change to support `FormData` bodies (see API/Frontend below).

**Explicitly out of scope (do not build):**
- Any change to `cmd/harvest`, `bookdata` clients, or the async harvest job/worker pipeline.
- Async/queued processing of the uploaded file (Approach C) — this is a synchronous HTTP request bounded by NFR-002.
- Fuzzy author/genre matching — exact `slugify(name)` match, autocreate on miss (already implemented in `admin/service/harvest_resolve.go`; reused as-is).
- An interactive preview/edit step before applying the import — the response *is* the report, produced after the batch is already applied (matches the existing harvest CLI's summary-after-the-fact UX and the business analysis' MVP decision).
- Updating existing books via re-import — only creation; a duplicate `slug` is reported as `skipped_existing`, never an update.
- Any new DB migration — `books.external_source`/`external_id` already exist (migration `000009_books_external_source`) and stay `NULL` for both lookup-assisted manual creation (existing `POST /books` path, unchanged) and file import (this contract does not set them — file-imported books are not "harvested", so they intentionally do **not** show up in the `pending_review` filter added by `docs/features/harvest-review-queue.md`).
- Auto-selecting/creating authors or genres from a lookup result in the form — the lookup assistant only pre-fills scalar fields; author/genre assignment stays a manual `MultiSelect` action against existing records, exactly as it is today.
- A CSV/JSON export or "download template" feature — not requested, not required by any FR here.

## User Stories

- As an admin, I can enter an ISBN or a title while creating a book and have the form's text fields pre-filled from Open Library, so I don't retype known data.
- As an admin, I can upload a CSV or JSON file with a batch of books (e.g. a partner export) and get them created in one operation, without visiting the form once per book.
- As an admin, I can see, after an import, exactly which rows were created, which were skipped as already existing, and which failed and why — so I can fix and re-upload only the bad rows.
- As an admin, I cannot use either feature without the `admin` role, exactly like every other admin mutation.

## Acceptance Criteria

**Lookup assistant**
- [ ] `GET /api/v1/admin/books/lookup?isbn=<isbn>` returns 200 with a normalized record (`title`, `authors[]`, `genres[]`, `isbn`, `description`, `cover_url`, `published_year`, `page_count`, `language`, `source_name`, `source_id`) when Open Library has a match.
- [ ] `GET /api/v1/admin/books/lookup?q=<title>&author=<author>` (author optional) does the same via title+author search.
- [ ] Missing both `isbn` and `q` → 400. No match → 404. Open Library rate-limited/unavailable (`bookdata.ErrRateLimited`/`ErrSourceUnavailable`) → 502 with a message telling the admin to retry or enter data manually.
- [ ] Endpoint requires `RoleAdmin` (`middleware.Auth` + `RequireRole`), same as every other `/api/v1/admin` route; it is a `GET` so it follows the existing pattern of *not* requiring the extra bearer-only check applied to state-changing admin routes (it performs no write).
- [ ] `book-form.tsx` gains a "Найти по ISBN/названию" button next to the ISBN field (or a small inline lookup bar) that calls the endpoint with whatever ISBN/title is currently typed, and on success overwrites `title`, `description`, `isbn`, `cover_url`, `published_year`, `page_count`, `language` (only fields the record actually returned — never clobber a field with `null`/empty from the lookup response). `author_ids`/`genre_ids` are left untouched; the returned author/genre **names** are shown as read-only hint text under the respective `MultiSelect` so the admin knows what to pick manually.
- [ ] 404/502/400 from the lookup show an inline message near the button; they never block manual form entry or submission.
- [ ] Saving is unchanged — still an explicit `POST /api/v1/admin/books` triggered by the existing submit button.

**Bulk file import**
- [ ] `POST /api/v1/admin/books/import` accepts a CSV or JSON file (`multipart/form-data`, field name `file`, plus a `format` field or filename-extension sniffing — developer's call, document the choice) and requires `RoleAdmin` + the existing bearer-token check applied to other state-changing admin routes.
- [ ] CSV schema (header row, column order-independent by name): `title` (required), `authors` (required, `;`-separated names), `genres` (optional, `;`-separated names), `isbn`, `description`, `cover_url`, `published_year`, `page_count`, `language`, `slug` (all optional). JSON schema: an array of objects with the same field names (snake_case), `authors`/`genres` as string arrays instead of `;`-joined strings.
- [ ] A structurally unreadable file (invalid JSON syntax, CSV with no parseable header) is rejected wholesale with a 400 and a general "file format" error — no rows are processed (E-01).
- [ ] Each row is validated and created independently: an error in one row does not prevent the rest of the file from being processed (FR-004). Validation reuses `admin.Service.CreateBook`'s existing rules (title required, ISBN-10/13 checksum, published year/page count bounds, language support, slug URL-safety/uniqueness) — no parallel validation implementation (NFR-001). The import layer adds exactly one import-specific rule on top: a row with zero resolvable author names is an error (E-02), since `CreateBook` itself does not require at least one author for the single-book form.
- [ ] Authors/genres by name are resolved via the existing `Service.ResolveOrCreateAuthorIDsByNames`/`ResolveOrCreateGenreIDsByNames` (exact `slugify(name)` match, autocreate on miss) — the same functions the harvest handler already uses. No new author/genre matching logic is written.
- [ ] A row whose generated/explicit `slug` collides with an existing book (`ErrDuplicateSlug`) is reported as `skipped_existing`, not an error, and does not modify the existing book (E-04). Two occurrences of the same title in one file: the first creates the book, the second collides on the same generated slug and is naturally reported `skipped_existing` — no separate in-file dedup pass is needed.
- [ ] `language`, if omitted on a row, defaults to `"ru"` (matches the existing form's default) before validation.
- [ ] The whole batch defaults to `is_hidden = false` (visible) unless the request includes a batch-level `is_hidden: true` flag (FR-006) — there is no per-row visibility override in this increment.
- [ ] Imported books never set `external_source`/`external_id` (those stay `NULL`) — this path is not the harvest path and must not be confused with it in the `pending_review` queue.
- [ ] The response is a JSON report: `{ "processed": n, "created": n, "skipped_existing": n, "errors": [{ "row": n, "title": "...", "message": "..." }] }` (`row` is 1-based over data rows, header excluded), returned with HTTP 200 even if some/all rows failed — the batch *request* succeeded; row outcomes are reported in the body, mirroring `HarvestResult`/`HarvestRecordError`'s existing shape in `backend/internal/modules/harvest/service/handler.go`.
- [ ] An empty file (header/`[]` only) is not an error: `processed: 0, created: 0, skipped_existing: 0, errors: []`.
- [ ] File size and row count are bounded per NFR-002: request body capped (e.g. `echomiddleware.BodyLimit` scoped to this route, ~2 MiB) → oversized upload rejected before parsing (413); parsed row count above a documented cap (e.g. 1000) → rejected wholesale before any row is processed (422), with a message pointing operators at `cmd/harvest`/`cmd/seed` for larger loads (E-06). Exact byte/row limits are the developer's call; they must be named constants, documented in code and in this file once implemented.
- [ ] `apps/admin` gains a `/books/import` page: file input (`.csv`/`.json`), submit button, and a result view listing created/skipped/error counts plus a per-row error table. Linked from the books list header (next to "Добавить книгу").
- [ ] `packages/api-client`'s `fetcher` is extended to not force `Content-Type: application/json` when the request body is a `FormData` instance (so the browser can set the multipart boundary itself); this is additive and does not change behavior for any existing JSON caller.

## Business Rules

- Lookup and import both require `RoleAdmin`, enforced the same way as every existing `/api/v1/admin` route — no new authorization mechanism.
- Lookup is read-only and makes exactly one outbound Open Library call per request; it never writes to the database and never creates authors/genres/books itself. Saving is always the existing explicit `POST /api/v1/admin/books` call.
- Import never calls an external network service in its own request path (NFR-003) — it only resolves names against the local DB and creates rows via existing service methods.
- Import is create-only: a `slug` collision is a skip, never an update to the existing record (matches the single-book form's own "409 on collision, no silent overwrite" rule in `docs/features/admin.md`).
- Import's author/genre matching is the same exact-normalized-name-match-then-autocreate policy already implemented for the harvest handler (Решение 3.a) — reusing, not reimplementing, `ResolveOrCreateAuthorIDsByNames`/`ResolveOrCreateGenreIDsByNames` is a hard requirement, not just a preference.
- Google Books is not involved in either endpoint (not applicable — out of scope for the whole initiative, see business analysis NFR-008).

## UX

- **Lookup**: a small inline action near the ISBN/title fields in the existing create-book form (`book-form.tsx`). No new page, no modal required — a button + optional loading/disabled state + inline success/error text is sufficient, consistent with the form's existing inline-validation style. The action is available on the create form; extending it to the edit form is optional/nice-to-have, not required by SCN-002 (which is scoped to "form создания книги").
- **Import**: one new page (`apps/admin/src/views/book-import` → `apps/admin/src/app/books/import/page.tsx`), reachable via a link/button on `books-list-view.tsx`'s header. Minimal flow: pick file → submit → show report (counts + scrollable error table with row/title/message). No progress bar (synchronous request, bounded size per NFR-002). Use existing `Table`/`Badge`/`Button` components from `@bookhaven/ui`, matching the visual style of `reviews-view.tsx`/`books-list-view.tsx`.
- Both features fail closed and legibly: a lookup miss or import file error never leaves the admin without an explanation or a way to proceed manually.

## API

- `GET /api/v1/admin/books/lookup?isbn=&q=&author=` — `RoleAdmin` only (`adminRole` middleware, same group as other admin GETs — no extra bearer requirement, it's a read). Responses: 200 (record found), 400 (no `isbn`/`q` given), 404 (not found at source), 502 (source rate-limited/unavailable), 500 (unexpected).
- `POST /api/v1/admin/books/import` — `RoleAdmin` + `requireBearer` (state-changing, same as `POST /books`). `multipart/form-data`: `file` (required), `format` (optional, `csv`|`json`; if omitted, sniff from the uploaded filename's extension), `is_hidden` (optional form field, `"true"`/`"false"`, batch-level). Responses: 200 (report body, regardless of per-row outcomes), 400 (unreadable file), 401/403 (auth), 413 (body too large), 422 (too many rows, or file readable but semantically empty of the expected shape — developer's call which of these two is 422 vs 400), 500 (unexpected).
- Register both routes in `backend/internal/transport/http/router.go`'s `RegisterAdminRoutes`, inside the existing `books := admin.Group("/books", adminRole)` group: `books.GET("/lookup", adminHandler.LookupBook)` and `books.POST("/import", adminHandler.ImportBooks, requireBearer)`. (Echo's router disambiguates the static `/lookup` segment from the `/:id` param route regardless of registration order, but keep `/lookup` declared near the other static book routes for readability.)
- Extend the admin transport `Service` interface (`backend/internal/modules/admin/transport/http/handler.go`) with `LookupBook(ctx, isbn, query, author string) (*bookdata.ExternalBookRecord, error)` and `ImportBooks(ctx, rows []service.ImportRowInput, defaultHidden bool) service.ImportReport` (naming is the developer's call; keep it consistent with existing method naming).
- `packages/api-client`: add `adminBooksApi.lookup(params)` (GET, query string) and `adminBooksApi.import(file, options)` (POST, `FormData`) plus `LookupRecord`/`ImportReport`/`ImportRowError` types mirroring the response shapes above.

## Database

No new migration. `books.external_source`/`external_id` (migration `000009_books_external_source`) already exist and stay `NULL` for every book created by either endpoint in this contract. No changes to `authors`/`genres` tables — they already support name/slug creation via the existing `admin.Service.CreateAuthor`/`CreateGenre` reused by `ResolveOrCreateAuthorIDsByNames`/`ResolveOrCreateGenreIDsByNames`.

## Frontend

- `apps/admin/src/features/manage-book/ui/book-form.tsx`: add lookup state (loading/result/error), a lookup trigger function calling `adminBooksApi.lookup`, and `setValue(...)` calls limited to fields the response actually contains. Do not touch `author_ids`/`genre_ids` from the lookup result.
- New `apps/admin/src/views/book-import/ui/book-import-view.tsx` (+ `index.ts`) and `apps/admin/src/app/books/import/page.tsx`, following the existing view/page split used by every other admin route (`books-list`, `book-new`, etc.).
- `apps/admin/src/entities/book/api/index.ts`: forward the two new `adminBooksApi` calls, same as it already forwards `list`/`get`/`create`/`update`.
- `packages/api-client/src/index.ts`: new `LookupRecord`, `ImportReport`, `ImportRowError` types; `admin.books.lookup`/`admin.books.import`; the `buildRequestHeaders`/`fetcher` `FormData` accommodation described in Acceptance Criteria.
- `books-list-view.tsx`: add an "Импортировать" link/button near "Добавить книгу", pointing at `/books/import`. No change needed to `shell.tsx`'s `adminOnlyPaths` guard — it already matches any path starting with `/books/`.

## Existing Patterns

- `backend/internal/modules/admin/service/harvest_resolve.go` — `ResolveOrCreateAuthorIDsByNames`/`ResolveOrCreateGenreIDsByNames`: reuse directly for import, do not reimplement name matching.
- `backend/internal/modules/harvest/service/handler.go` — `processRecord`/`HarvestResult`/`HarvestRecordError`: the closest existing analog for "per-record independent processing + aggregated report + reuse `admin.Service.CreateBook`"; mirror its shape for `ImportReport`/`ImportRowError` and its error-doesn't-abort-the-batch control flow.
- `backend/internal/modules/admin/service/service.go` — `BookInput`, `CreateBook`, `slugify`, `isValidISBN`: the only place book validation/creation logic may live; import must call into this, not duplicate it.
- `backend/internal/pkg/bookdata/bookdata.go` — `Client` interface (`FindByISBN`, `FindByTitleAuthor`), `ExternalBookRecord`, sentinel errors (`ErrNotFound`, `ErrRateLimited`, `ErrSourceUnavailable`): reuse directly for the lookup endpoint; `bookdata.NewOpenLibraryClient()` is already constructed with no options in `backend/cmd/worker/main.go` — mirror that zero-config construction in `backend/cmd/api/main.go` (the placeholder default `User-Agent` contact is an already-accepted, documented risk from `docs/features/bookdata-clients.md`, not a new one introduced here).
- `backend/internal/modules/admin/transport/http/handler.go` — existing handler error-mapping `switch` blocks (e.g. `CreateBook`) for the JSON error-response convention (`{"error": "..."}`, matching HTTP status per error type); mirror this for the two new handlers rather than inventing a new error envelope.
- `apps/admin/src/views/reviews/ui/reviews-view.tsx` / `books-list-view.tsx` — feedback banner + `Table` rendering pattern for the import report view.
- Service construction: add the lookup client to `admin.Service` via a **variadic functional option** (`NewService(repo repository.Repository, opts ...Option)`, e.g. `WithLookupClient(bookdata.Client)`), not by changing `NewService`'s required signature — this keeps `backend/cmd/worker/main.go`'s existing `adminservice.NewService(adminRepo)` call and every existing `admin/service` test's `NewService(repo)` call working unmodified. Only `backend/cmd/api/main.go` needs the new option (`adminservice.NewService(adminRepo, adminservice.WithLookupClient(bookdata.NewOpenLibraryClient()))`).

## Tests

- `backend/internal/modules/admin/service`: table-driven tests for `LookupBook` (isbn path, title+author path, not-found passthrough, no client configured, empty-params validation error) using a fake `bookdata.Client`; table-driven tests for the import orchestration function (success, duplicate-slug skip, missing-title error, zero-author error, invalid-ISBN error, language default applied, batch `is_hidden` flag respected, `external_source`/`external_id` stay nil, empty input → zero report, mixed batch partial success).
- `backend/internal/modules/admin/transport/http`: handler tests for `GET /books/lookup` (200/400/404/502) and `POST /books/import` (CSV happy path, JSON happy path, malformed file → 400, oversized file → 413, too-many-rows → 422, non-admin → 403, missing bearer on import → 401/403 per existing convention). Update every fake implementation of the transport `Service` interface (at least `handler_test.go`'s own fake; check for others the way `docs/features/harvest-review-queue.md` had to update three fakes for a smaller interface change) to add the two new methods.
- Frontend: extend `apps/admin/src/features/manage-book` (add a `book-form.test.tsx` case, or extend if one exists) for the lookup button's pre-fill/error states; a new test for `book-import-view` covering file selection, submit, and report rendering (success + row-error rendering) if the existing suite's depth on views justifies it — otherwise flag as manual verification, consistent with `docs/features/harvest-review-queue.md`'s own stated bar for `apps/admin` view tests.
- `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`; `apps/admin` type-check/lint clean. Do not require the full monorepo test suite for this feature (matches `docs/features/admin.md`'s stated testing bar).

## Documentation

- Mark this file `Status: implemented` when done. Done.
- No `docs/ROADMAP.md` line-item change required — same rationale as the two prior increments (`bookdata-clients.md`, `harvest-review-queue.md`): this fulfills the `catalog-data-import` initiative, not a standalone roadmap entry.
- NFR-002 decision: `POST /admin/books/import` caps the request body at **2 MiB** (`echomiddleware.BodyLimit("2M")`, `adminhttp.ImportMaxRequestBody`, applied on the route in `router.go` — oversized uploads get 413 before any parsing) and the parsed row count at **500 data rows** (`maxImportRows` in `backend/internal/modules/admin/transport/http/import_parse.go` — an oversized file gets 422 with a message pointing at `cmd/harvest`/`cmd/seed`, per E-06). 500 was chosen at the lower end of the business analysis' "order of hundreds of rows" guidance, comfortably inside a synchronous HTTP request budget.
- The CSV/JSON import schema (columns, delimiters, per-column optionality) is documented as a comment block at the top of `backend/internal/modules/admin/transport/http/import_parse.go`, not duplicated here.
- Implementation notes:
  - `admin.Service` gained a `WithLookupClient(bookdata.Client)` functional option (`backend/internal/modules/admin/service/service.go`); only `backend/cmd/api/main.go` passes it (`bookdata.NewOpenLibraryClient()`), mirroring `cmd/worker/main.go`'s zero-config construction.
  - `LookupBook` (`backend/internal/modules/admin/service/lookup.go`) and `ImportBooks` (`backend/internal/modules/admin/service/import.go`) are plain `*Service` methods, not a separate package — unlike the harvest handler, they needed no dependency the `admin/service` package didn't already own.
  - A column entirely absent from a CSV header (e.g. no `authors` column at all) is not a whole-file 400 — every row simply gets an empty value for that column and is validated per-row like any other missing value (E-02 for `authors`, `CreateBook`'s own `ErrInvalidTitle` for `title`). Only a header/data line that cannot be parsed as CSV at all, or JSON that fails to decode as an array of objects, is a whole-file 400 (E-01).
  - Non-numeric `published_year`/`page_count` CSV cells are treated as "not provided" (left `nil`) rather than failing the row, since both fields are optional and still bounds-checked by `CreateBook` when present.

## Risks

- **`author_ids`/`genre_ids` remain ID-based in the single-book form; the lookup response only has names.** Mitigated by explicitly not auto-selecting them (see Acceptance Criteria) — the admin still picks from existing authors/genres via the current `MultiSelect`, or creates them first via the existing author/genre admin pages. This is a deliberate scope cut, not an oversight; expanding it (auto-resolve/autocreate from the lookup result, like import does) is a reasonable but separate follow-up.
- **File import author/genre autocreate can multiply near-duplicate authors** (e.g. "Л. Толстой" vs "Лев Толстой" in the same file) — identical, already-accepted risk to the harvest path (R-002 in the business analysis); mitigated the same way, via the import report making new authors visible for manual consolidation after the fact (report only lists book-row outcomes, not which authors/genres were newly created — consider surfacing that too if it proves confusing in practice, not required for MVP).
- **`fetcher`'s `FormData` accommodation touches shared, widely-used code** (`packages/api-client`) — low risk since it's additive (only skips forcing JSON content-type when the body is `FormData`), but every existing JSON-based admin call must be regression-tested (type-check + existing test suite) to confirm no behavior change.
- **NFR-002's exact size/row limits are not pinned by the business analysis** ("order of hundreds of rows") — the developer must pick and document concrete constants; too low frustrates real usage, too high risks the exact synchronous-HTTP-timeout problem Способ B was scoped away from (see business analysis Решение 2). Recommend erring toward the lower end (hundreds, not low thousands) and pointing to `cmd/harvest`/`cmd/seed` for anything larger.
- **Import's `ImportReport`/handler `Service` interface changes are mechanical but touch several files** (service, transport interface, router wiring, api-client, admin view) — same category of "small feature, bigger diff" risk already flagged in `docs/features/harvest-review-queue.md` for `DashboardCounts`; not a reason to change approach, just a heads-up for review.
