# Bulk Publish for Pending-Review Harvested Books

Status: implemented

This is a follow-up to `docs/features/harvest-review-queue.md` (implemented), which explicitly deferred a bulk "publish all" action ("Bulk publish, if wanted later, is a follow-up; flag it, don't build it."). That contract shipped the `pending_review` admin book filter, the `external_source`/`pending_import_review` fields, and a per-row "Опубликовать" quick action that reuses the existing `PATCH /api/v1/admin/books/:id` full-replace endpoint. This contract adds a single new action that publishes every book currently matching the admin's `pending_review` view (optionally narrowed by the existing search box), so an admin no longer has to click "Опубликовать" once per row after a large harvest run.

## Scope boundaries (read first)

**In scope:**
1. A new backend endpoint that publishes (sets `is_hidden = false` on) every book matching `is_hidden = true AND external_source IS NOT NULL`, optionally narrowed by the same free-text `search` the books list already supports, processing each matching row independently (a failure on one row does not stop the rest) and returning a per-batch report.
2. `apps/admin`: an "Опубликовать все (N)" button on the books list, visible only when the `pending_review` filter is active and at least one row matches, reusing the existing confirm() + mutation + feedback pattern already on this page.
3. `packages/api-client`: a new `adminBooksApi.bulkPublish` method and matching response type.

**Explicitly out of scope (do not build):**
- A multi-select (checkbox-per-row) UI. Decision, made explicitly per the backlog proposal's own prompt: bulk publish means "publish everything matching the current `pending_review` (+ optional search) filter," not an arbitrary cross-page selection. Rationale: (a) the stated pain point is clearing an entire harvest run, not cherry-picking a subset; (b) the admin app's UI kit (`packages/ui`) has no `Checkbox` primitive today, so a multi-select would require a new component, which the single-action design avoids; (c) the existing per-row "Опубликовать" button already covers the cherry-pick case and is untouched by this contract. If a real need for partial/arbitrary multi-select emerges later, it is a new follow-up, not silently folded into this one.
- Any new review/approval workflow, state machine, or status column. "Pending review" keeps meaning exactly what `harvest-review-queue.md` defined (`is_hidden = true AND external_source IS NOT NULL`); bulk publish is just the existing single-row publish mutation applied to every matching row.
- Async/background job processing for the bulk action. Given the existing `maxImportRows = 500` precedent (`backend/internal/modules/admin/transport/http/import_parse.go`) for synchronous batch admin HTTP calls, this stays a synchronous HTTP request with the same order-of-magnitude cap (see Business Rules) — no asynq task, no progress bar.
- Any change to `BookFilter`, the `pending_review` filter's definition, `DashboardCounts`, or the dashboard UI — unchanged from `harvest-review-queue.md`.
- Any change to the per-row `PATCH /api/v1/admin/books/:id` endpoint or its validation rules (`service.UpdateBook`/`validateBookInput`) — the bulk action calls the same service method per row, unchanged.

## User Stories

- As an admin, after a harvest job finishes and populates the pending-review queue with many books, I can publish all of them in one action instead of clicking "Опубликовать" on every row.
- As an admin, I can narrow the queue with the existing search box first (e.g. by author or title) and then bulk-publish just that narrowed set, if I don't want to publish everything yet.
- As an admin, if one book in the batch fails to publish (e.g. it somehow fails a validation rule), I still want the rest of the batch published and a clear report of what happened, not a single all-or-nothing failure.

## Acceptance Criteria

- [ ] New endpoint `POST /api/v1/admin/books/bulk-publish`, registered in the same `books := admin.Group("/books", adminRole)` group as the other book routes, with `requireBearer` (same as `POST /books/import`, `PATCH /books/:id`) — admin role only, consistent with every other book-mutating route (moderators cannot bulk-publish, same as they cannot hit `PATCH /books/:id` today).
- [ ] Request body is optional JSON `{"search": "<string>"}`, mirroring the existing `search` query param already accepted by `GET /admin/books`; an absent/empty `search` means "all pending-review books." The endpoint ignores any `visibility` the client might send — it always and only targets `is_hidden = true AND external_source IS NOT NULL` (the same predicate as the `pending_review` filter), regardless of request body content, so it can never be used to mass-publish books that aren't harvested-and-pending.
- [ ] Backend: `admin/service.Service` gains `BulkPublishBooks(ctx context.Context, search string) (BulkPublishReport, error)`. It calls `repo.ListBooks(ctx, repository.BookFilter{Visibility: "pending_review", Search: search, Limit: bulkPublishCap + 1})` to fetch matching rows, and for each one builds a `BookInput` from the row's already-loaded fields (`Title`, `Slug`, `Description`, `CoverURL`, `ISBN`, `PublishedYear`, `PageCount`, `Language`, author/genre IDs from `book.Authors`/`book.Genres`) with `IsHidden: false`, then calls the existing `s.UpdateBook(ctx, book.ID, input)` — the same method and validation path the single-row frontend action already drives via `PATCH /books/:id`. No new validation logic is introduced.
- [ ] If more than `bulkPublishCap` (500, matching `maxImportRows`) books match, the service returns a sentinel error (e.g. `ErrBulkPublishTooLarge`) without publishing anything, and the handler returns `422` with a message telling the admin to narrow the search or that an operator can re-run the harvest review in batches. Partial processing never starts in this case (all-or-nothing only for the "is the batch too big to run" guard, not for the per-row outcomes within an accepted batch).
- [ ] Within an accepted batch, each row is processed independently: a `UpdateBook` error on one row is recorded and does not stop the loop, mirroring `ImportBooks`' existing pattern in `backend/internal/modules/admin/service/import.go` exactly (same "processed/succeeded/errors" report shape, same "continue on row error" control flow).
- [ ] `BulkPublishReport{Processed int, Published int, Errors []BulkPublishRowError}` where `BulkPublishRowError{BookID string, Title string, Message string}`; response is always `200 OK` with this report regardless of per-row outcomes (same convention as `POST /books/import`'s `ImportReport`), except for the `422` too-large case above.
- [ ] Zero matching rows is not an error: `{"processed": 0, "published": 0, "errors": []}` with `200 OK`.
- [ ] `packages/api-client`: `adminBooksApi.bulkPublish(params?: { search?: string }): Promise<BulkPublishReport>` calling `POST /api/v1/admin/books/bulk-publish`; `BulkPublishReport`/`BulkPublishRowError` types exported alongside the existing `AdminBook`/`ImportReport`-style types.
- [ ] `apps/admin` books list: when `visibility === 'pending_review'` and `data.total > 0`, render an "Опубликовать все (N)" button (N = `data.total`, the already-fetched filtered count — no extra request), positioned near the filter `Select`/table, not inside any row.
- [ ] Clicking it shows `confirm(...)` (same pattern as the per-row action, e.g. "Опубликовать все книги на проверке (N)?"), then calls `adminBooksApi.bulkPublish({ search: search || undefined })`, then invalidates `['admin', 'books']` and `['admin', 'dashboard']` (the pending-import-review count tile changes too) and shows a feedback summary derived from the report (e.g. "Опубликовано N из M." plus an error count/sample if `errors.length > 0`), reusing the existing `feedback` state and styling already on this page — no new dialog/toast component.
- [ ] The existing per-row "Опубликовать" button and its mutation are untouched and remain available in the same views.
- [ ] Backend tests: `admin/service` test for `BulkPublishBooks` (all-succeed, partial-failure continues the batch, too-large returns the sentinel error without publishing, zero-match returns an empty report, `search` is passed through to the underlying filter); `admin/transport/http` handler test for the new route (success report shape, 422 on too-large, role/auth enforcement consistent with other book-mutating routes); `admin/repository` needs no new test (reuses `ListBooks`/`UpdateBook`, already covered).
- [ ] Frontend test: extend `apps/admin/src/views/books-list/ui/books-list-view.test.tsx` for the new button's visibility condition (hidden unless `pending_review` + nonzero total), confirm-then-call behavior, and feedback rendering — following this file's existing test patterns for the per-row publish action.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`; admin app type-check/lint clean.

## Business Rules

- Bulk publish targets exactly the `pending_review` predicate (`is_hidden = true AND external_source IS NOT NULL`), optionally AND'd with the same full-text `search` condition `ListBooks` already applies — never any other visibility state, regardless of what a caller sends in the request body. This is enforced server-side (the service hardcodes `Visibility: "pending_review"`), not merely a frontend convention.
- The batch is capped at `bulkPublishCap = 500` matching rows (same figure as `maxImportRows`, both existing synchronous-HTTP admin batch operations) to bound one request's latency/DB load; exceeding it is a client-visible `422` with no partial effect, not a silent truncation. An admin facing a larger queue narrows via `search` or (per `harvest-review-queue.md`'s own existing risk note about harvest batch sizing) avoids harvesting more than that many un-reviewed books at once.
- Per-row processing reuses `UpdateBook`'s existing validation unchanged. A row can still fail (e.g. if a referenced author/genre was deleted since the book was harvested) — that failure is reported, not silently swallowed or allowed to abort sibling rows, matching `ImportBooks`' established precedent for "many independent records in one admin batch action."
- No optimistic locking/snapshotting: the query that selects matching rows and the per-row update both run against current DB state at call time, not a client-supplied ID list. If another admin publishes or deletes a row concurrently, the worst case is a harmless no-op (re-publishing an already-published book has no effect) or a "not found" row error — not data corruption. This is an accepted low-probability race, not mitigated further (no row locking), consistent with the rest of this admin app's concurrency posture.

## UX

- Books list (`apps/admin/src/views/books-list`): one new button, conditionally rendered next to the existing search input/visibility `Select` row, labeled "Опубликовать все (N)". Only visible for the `pending_review` filter value with `N > 0`. Uses the same `confirm()` + `feedback` banner conventions already in this file — no new UI primitives, no new route/page.
- No change to the per-row action, the dashboard, or any other view.

## API

`POST /api/v1/admin/books/bulk-publish` — new. Body (optional): `{"search": "<string>"}`. Response `200`: `{"processed": number, "published": number, "errors": [{"book_id": string, "title": string, "message": string}]}`. Response `422` when the matching set exceeds `bulkPublishCap`: `{"error": "<message telling the admin to narrow search>"}`. Auth: Bearer + admin role (same as `PATCH /admin/books/:id`).

No changes to any existing endpoint.

## Database

No schema changes. Reuses the existing `books.is_hidden`/`books.external_source` columns and the `pending_review` filter predicate already shipped in `harvest-review-queue.md`.

## Frontend

- `apps/admin/src/views/books-list/ui/books-list-view.tsx`: add a `bulkPublishMutation` (`useMutation`) alongside the existing `publishMutation`, a `handleBulkPublish` handler with `confirm()`, and the conditional button in the toolbar area. Feedback message built from the returned `BulkPublishReport`.
- `packages/api-client/src/index.ts`: add `BulkPublishReport`/`BulkPublishRowError` interfaces and `adminBooksApi.bulkPublish`.

## Existing Patterns

- `backend/internal/modules/admin/service/import.go` (`ImportBooks`/`ImportReport`/`ImportRowError`) — the direct template for `BulkPublishBooks`'s "independent per-record processing + aggregate report, HTTP 200 regardless of row outcomes" shape. Follow its structure closely rather than inventing a different report shape.
- `backend/internal/modules/admin/transport/http/import_parse.go` (`maxImportRows = 500`) — precedent for `bulkPublishCap`'s value and rationale (bounding a synchronous admin batch HTTP call).
- `backend/internal/modules/admin/service/service.go`'s `UpdateBook` — reused as-is per row; do not duplicate its validation.
- `backend/internal/transport/http/router.go` — `books := admin.Group("/books", adminRole)` plus per-route `requireBearer` on mutating routes — register the new route the same way.
- `apps/admin/src/views/books-list/ui/books-list-view.tsx` — existing `publishMutation`/`handlePublish`/`feedback` pattern — mirror directly for the bulk action rather than introducing a different confirmation/feedback mechanism.

## Tests

- Backend: `admin/service` — table-driven test for `BulkPublishBooks` covering all-succeed, one-row-fails-rest-proceed, too-large (no publishing happens), zero-match, and `search` passthrough.
- Backend: `admin/transport/http` handler test for `POST /admin/books/bulk-publish` — success report, 422 on too-large, non-admin/unauthenticated rejection.
- Frontend: extend `books-list-view.test.tsx` for button visibility, confirm-gated mutation call, and feedback rendering.
- `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`; admin app type-check/lint clean — same bar as `harvest-review-queue.md`.

## Documentation

- Mark this file `Status: implemented` when done.
- `docs/ROADMAP.md`: no change required for the same reason stated in `harvest-review-queue.md` (fulfills a follow-up noted in an existing contract, not a standalone roadmap line item) — unless the developer finds it clearer to add one line under "Completed" next to the existing catalog-data-import entry.
- Backlog issue #114: close it as resolved once this contract ships (it is the issue this contract answers).

## Risks

- **No multi-select means an admin cannot bulk-publish an arbitrary cross-page subset without a search term that happens to isolate it.** Accepted per the explicit scope decision above; a real need for arbitrary multi-select is a distinct future follow-up (would also require adding a `Checkbox` primitive to `packages/ui`, which this contract deliberately avoids).
- **Synchronous HTTP processing of up to 500 rows in one request** could be slow (500 sequential `UpdateBook` calls, each doing its own slug-existence/author/genre-existence checks and a DB write) — acceptable given the existing `maxImportRows` precedent for the same trade-off, but flagged in case 500 proves too slow in practice; lowering the cap or moving to an async job is a follow-up if so, not pre-built here.
- **No row locking** — a rare concurrent double-publish or publish-vs-delete race is possible but harmless (idempotent publish, or a reported-but-ignorable row error). Not mitigated further, consistent with the rest of the admin app.
- **The `search`-scoped variant means "bulk publish" isn't always literally "every pending book"** — this is intentional (see Scope boundaries) but worth the developer double-checking the button's count label (N) always matches exactly what will be published, to avoid an admin's surprise if `search` text and the bulk-publish call ever got out of sync (e.g. a stale `search` state value at click time). Use the same `search` value that produced the currently-displayed `data.total`, not a different one.
