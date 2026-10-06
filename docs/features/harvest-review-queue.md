# Harvest Review Queue, Seed Guardrail, and Curated-Author Harvest Tooling

Status: implemented

This is the next increment on top of `docs/features/async-harvest-queue.md` (implemented). That increment made harvested books get created with `is_hidden = true` and `external_source`/`external_id` set, but shipped no way to (a) find them distinctly in the admin UI, (b) safely decide what to harvest without hand-written SPARQL, or (c) stop the demo seed from being mistaken for real data. This contract covers three related, independently shippable asks. Treat `docs/ai/features/catalog-data-import/business-analysis.md`'s settled decisions as context; this contract resolves **OQ-005** (harvest selection criterion) by committing to "способ A"/Approach A tooling: a curated Wikidata author-QID list, not an open category query with `LIMIT`.

## Scope boundaries (read first)

**In scope:**
1. A guardrail in `backend/cmd/seed/main.go` that refuses to run against `APP_ENV=production` unless explicitly forced, plus documentation of dev/demo-only intent.
2. Backend: a new `BookFilter.Visibility` value (`pending_review`) plus exposing `external_source` on the admin book response, so imported-and-unreviewed books can be listed/counted distinctly from other hidden books.
3. Frontend (`apps/admin`): a filter option + badge + a per-row "publish" quick action on the existing books list, and a dashboard count tile, reusing existing list/update/confirm patterns.
4. Backend: a convenience mode on `backend/cmd/harvest` that builds a bounded, safe SPARQL query from a curated list of Wikidata author QIDs, via a new exported query-builder function in `backend/internal/pkg/bookdata`.

**Explicitly out of scope (do not build):**
- Any new environment-detection framework — reuse `config.Config.Env`/`APP_ENV`, which already exists and is already used the same way for the `JWT_SECRET` production check in `backend/internal/config/config.go`.
- Removing or disabling `make seed` / `cmd/seed` — it stays available for local dev, per the request.
- A bulk "publish all" backend endpoint or any new bulk-mutation infrastructure. Bulk publish, if wanted later, is a follow-up; flag it, don't build it.
- Any redesign of the harvest job payload, the asynq task/handler, or `bookdata.WikidataClient.Query`/`Client` interface. The author-list mode only builds a SPARQL string on the CLI side; the job payload (`HarvestPayload{SPARQLQuery string}`) and worker-side handling are unchanged.
- Fuzzy/broad category harvesting (Approach B) tooling — the existing raw `-query`/`-query-file` flags remain available for that; no new convenience layer for it in this increment.
- `authors`/`genres` gaining `external_source`/`external_id` — not needed here (unchanged from the prior increment).
- Changing what "hidden" means/returns in the admin book list — `visibility=hidden` continues to return *all* hidden books (including pending-review ones); `pending_review` is an additive, narrower filter value, not a behavior change to the existing one.

## User Stories

- As an ops/developer operator, I cannot accidentally run `cmd/seed` against a production database without a deliberate override.
- As an admin, I can see, in one place, exactly which books came from a harvest job and still need review, separately from books hidden for other reasons (e.g. a flagged bad book).
- As an admin, I can review an imported book and publish (un-hide) it without leaving the books list, with the same confirmation UX already used elsewhere in the admin app.
- As an admin, I can see at a glance (dashboard) how many imported books are awaiting review.
- As an ops/developer operator, I can harvest a curated list of authors' bibliographies by passing Wikidata author QIDs to `cmd/harvest`, without hand-writing SPARQL, while raw SPARQL (`-query`/`-query-file`) remains available for ad hoc/Approach-B use.

## Acceptance Criteria

**Ask 1 — seed guardrail**
- [ ] `cmd/seed` loads config (reusing `config.Load()`, which already reads `APP_ENV`) and exits non-zero with a clear message if `cfg.Env == "production"`, unless an explicit override is passed (a `-force` flag, consistent with existing single-purpose CLI flag style in `cmd/harvest`).
- [ ] A code comment at the top of `cmd/seed/main.go` states it is dev/demo-only and must not be used to populate a real catalog.
- [ ] `make seed` / `Makefile` comment and `docs/DEPLOYMENT.md`'s existing seed note (line ~73) are consistent with the new guardrail (mention the `-force` override if an operator genuinely needs to run it non-locally).
- [ ] No change to `cmd/seed`'s actual seed data or idempotency logic (`FirstOrCreate` pattern untouched).

**Ask 2 — admin review queue**
- [ ] `admin/repository.BookFilter.Visibility` accepts a new value `"pending_review"`; `GormRepository.ListBooks` filters `is_hidden = true AND external_source IS NOT NULL` for it. Existing values (`""`, `"visible"`, `"hidden"`) are unchanged in behavior.
- [ ] `GET /api/v1/admin/books?visibility=pending_review` returns only imported-and-hidden books, paginated the same way as today; existing `visibility=hidden` still returns all hidden books (superset), unchanged.
- [ ] `bookResponse` (admin book JSON) gains `external_source` (`*string`, `omitempty`, mirrors existing nullable-string field style in that struct) so the frontend can tell "imported" apart from "manually hidden" wherever a book is rendered, not just in the dedicated queue view.
- [ ] `admin/repository.Repository.DashboardCounts` (and its `GormRepository`/`admin/service.Service` implementations, and the `Handler.Service` interface) gain a `pendingImportReview int64` count (`is_hidden = true AND external_source IS NOT NULL`); `GET /api/v1/admin/dashboard` response gains `pending_import_review`. Keep the existing all-positional-return-values convention for minimal diff (a struct-based refactor is not required for this change).
- [ ] `apps/admin`: the books list filter (`Select`) gains a `pending_review` option (label e.g. "Импортировано (на проверке)"); the table shows an "Импортировано" badge (derived from `external_source` presence) alongside the existing Скрыта/Видима badge.
- [ ] `apps/admin`: in the `pending_review` view (or wherever `external_source` is present on a hidden row), a "Опубликовать" quick action is available per row; clicking it shows a `confirm()` prompt (matching the existing review-moderation pattern in `apps/admin/src/views/reviews/ui/reviews-view.tsx`) and, on confirm, calls the existing `adminBooksApi.update(id, {...currentBookFields, is_hidden: false})` (built from the already-fetched `AdminBook` row — `author_ids`/`genre_ids` mapped from `authors`/`genres`), then refetches the list. No new backend endpoint.
- [ ] `apps/admin`: dashboard gains a "Импортировано, на проверке" card linking to `/books?visibility=pending_review`, following the existing `cards` array pattern in `dashboard-view.tsx`.
- [ ] `packages/api-client`: `AdminBook` gains `external_source: string | null`; `AdminDashboard` gains `pending_import_review: number`; `adminBooksApi`/`api.admin.books.list` params gain `'pending_review'` as a valid `visibility` value.
- [ ] Backend tests: repository test for the new filter value; handler test for `visibility=pending_review` query param and for the new dashboard field; service test for the new `DashboardCounts` return value.
- [ ] Frontend tests (to the extent the existing suite covers list/dashboard views): filter option renders and is selectable; badge renders when `external_source` is present; publish action calls update with `is_hidden: false` and requires confirmation.

**Ask 3 — Approach A curated-author-list harvest tooling**
- [ ] New exported function in `backend/internal/pkg/bookdata` (new file, e.g. `query_builder.go`, or added to `wikidata.go` next to `buildTitleAuthorQuery`/`buildISBNQuery`): `BuildAuthorListQuery(authorQIDs []string, limit int) (string, error)`. Reuses the existing literary-work class filter (`wdt:P31/wdt:P279* wd:Q7725634`, same QID already used by `buildTitleAuthorQuery`/`buildISBNQuery`) and the existing `ru`/`en` label-preference pattern (`rdfs:label` + `FILTER(LANG(...) IN ("ru","en"))`), constrained by a `VALUES ?author { wd:Q... wd:Q... }` clause built from the given QIDs, joined via `?item wdt:P50 ?author`. Output variables (`item`, `itemLabel`, `authors`, `genres`, `isbn`, `published`, `language`) match what `parseSparqlBindings` already expects — no changes needed there.
- [ ] `BuildAuthorListQuery` validates each QID matches `^Q[0-9]+$` (case-sensitive, no `wd:` prefix expected from the caller — the function adds it), dedupes, and returns an error (`bookdata.ErrInvalidQuery` or a new sentinel, developer's call, documented) if the list is empty or any QID is malformed.
- [ ] `limit <= 0` applies a sane default (e.g. 500); the function does not silently accept an unbounded/absurdly large limit — apply a documented upper bound (e.g. 5000, matching the product owner's own stated range of "100 to 5000") and clamp or error above it (developer's call, document the choice).
- [ ] `cmd/harvest` gains `-authors` (comma-separated QIDs, e.g. `-authors Q7200,Q1183296`) and `-authors-file` (a `.txt` file, one QID per line, blank lines and `#`-prefixed comment lines ignored — mirrors the existing plain-text convention of `-query-file`) flags, plus `-limit` (int, only meaningful with `-authors`/`-authors-file`; flag it as a usage error if combined with `-query`/`-query-file`).
- [ ] Exactly one input mode is required among `{-query, -query-file, -authors, -authors-file}`; combining any two is a usage error (extend the existing `buildQuery`-equivalent argument-resolution function rather than duplicating flag-parsing logic).
- [ ] The resolved SPARQL (whichever mode produced it) is still passed through the existing `harvestservice.NewHarvestTask(query)` → `asynq.Client.Enqueue` path unchanged — the job payload shape (`HarvestPayload{SPARQLQuery string}`) and worker-side handling are untouched, per Scope boundaries.
- [ ] `cmd/harvest -h` / the top-of-file comment documents the new flags and gives a one-line example of the author-list mode (mirrors the existing raw-SPARQL example already in the file header).
- [ ] Unit tests: `bookdata` package — table-driven tests for `BuildAuthorListQuery` (valid QIDs produce a query containing the `VALUES`/class-filter/label-filter/`LIMIT` clauses; invalid QID format errors; empty list errors; default and clamped `limit` behavior). `cmd/harvest` — tests for the extended argument-resolution function (mutual exclusivity across all four flags, QID-file parsing including comments/blank lines, QID-list splitting/trimming).
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`.

## Business Rules

- Production-guardrail check is on `APP_ENV` only (via the existing `config.Config.Env`, default `"development"`) — no new environment-detection heuristic (e.g. sniffing `DATABASE_URL` hostnames) is introduced; this matches the precedent already set by `config.Load()`'s own `JWT_SECRET` production check.
- "Pending review" is defined purely as `books.is_hidden = true AND books.external_source IS NOT NULL` — no new column, no new state machine. Once an admin un-hides such a book (`is_hidden = false`), it is indistinguishable from any other visible book in the public catalog and in the default admin list, exactly as the request specifies. It also stops appearing in `visibility=pending_review`.
- A book that started as `pending_review`, was hidden again later for an unrelated reason (e.g. flagged), and still has `external_source` set will still show under `pending_review` (the filter can't and doesn't need to distinguish "still awaiting its first review" from "was published, then re-hidden" — this is an accepted, low-impact simplification; not worth a new column for this increment).
- Approach A's safety net (author QID allow-list + literary-work class filter + bounded `LIMIT`) reduces duplicate-book and off-topic-content risk but does not eliminate the pre-existing duplicate risk against the 19 manually-seeded demo books (their `external_source`/`external_id` are `NULL`, so the idempotency check in the harvest handler can't see them). This increment does not attempt to backfill `external_source`/`external_id` onto seed data or reconcile it — flagged as a known residual risk (see Risks), mitigated operationally by not re-harvesting authors already present in the demo seed, and by the pending-review step giving admins a chance to catch/delete an accidental duplicate before it's published.
- `-limit` on the author-list CLI mode bounds the *query's* `LIMIT`, not the number of QIDs — an operator can still pass many authors; the bound exists to stop one query from pulling an unexpectedly huge result set (and correspondingly long Open-Library-enrichment tail at ~1 req/sec) in one job.

## UX

- Books list (`apps/admin/src/views/books-list`): existing visibility `Select` gets a fourth option. Existing table gains one badge (Импортировано) shown when `external_source` is present, and a conditional "Опубликовать" action button next to "Редактировать" when the row is hidden-and-imported. Confirmation via `window.confirm(...)`, matching `reviews-view.tsx`'s existing pattern (no new dialog component needed).
- Dashboard: one new card, same visual treatment as the existing five, linking to the filtered books list.
- No new routes, no new pages — additive to existing views only.

## API

`GET /api/v1/admin/books?visibility=pending_review&search=&page=&limit=` — same endpoint, new accepted `visibility` value; response shape (`listBooksResponse`) unchanged except each `bookResponse` item now includes `external_source`.

`GET /api/v1/admin/dashboard` — response gains `pending_import_review: number` alongside the existing five counts.

No new endpoints. `PATCH /api/v1/admin/books/:id` (existing, full-replace semantics) is reused as-is for publishing a reviewed book — the frontend builds the full `AdminBookInput` from the already-loaded row with `is_hidden: false`.

## Database

No schema changes required — `books.external_source`/`external_id` already exist (migration `000009_books_external_source`, prior increment). The new `pending_review` filter and dashboard count are plain `WHERE is_hidden = true AND external_source IS NOT NULL` queries against existing columns.

Optional, not required for correctness: a partial index `CREATE INDEX ON books(is_hidden) WHERE external_source IS NOT NULL` could speed up the pending-review query at larger catalog scale. Given current expected volume (hundreds to low thousands of rows per the product owner's own examples), this is not required for this increment — flag as a follow-up if the admin books list becomes slow, don't add a migration preemptively.

## Frontend

- `apps/admin/src/views/books-list/ui/books-list-view.tsx`: add the `pending_review` filter option, the imported badge, and the publish quick action + confirm + mutation (reuse `useQuery`/`useMutation` conventions already in this file/adjacent views).
- `apps/admin/src/views/dashboard/ui/dashboard-view.tsx`: add the new card to the existing `cards` array.
- `packages/api-client/src/index.ts`: extend `AdminBook`, `AdminDashboard`, and the `visibility` param union type as listed in Acceptance Criteria.
- `apps/admin/src/entities/book/api/index.ts`: no change needed (already forwards `api.admin.books.list`/`update`).

## Existing Patterns

- `backend/internal/modules/admin/repository/repository.go` — `BookFilter.Visibility` switch in `ListBooks`; extend the same `switch` statement, don't add a parallel filtering mechanism. `DashboardCounts`' sequential `Count(...)` calls — add one more in the same style.
- `backend/internal/modules/admin/transport/http/handler.go` — `toBookResponse`, `dashboardResponse`, `ListBooks`/`Dashboard` handlers — extend the existing structs/functions, don't introduce new response types.
- `backend/internal/pkg/bookdata/wikidata.go` — `buildTitleAuthorQuery`/`buildISBNQuery`/`escapeSPARQLString`/the `wd:Q7725634` literary-work class QID/the `ru`/`en` label-filter idiom — reuse directly for `BuildAuthorListQuery`; these are unexported today, so either export what's needed or (preferred, smaller diff) add the new function in the same package/file where it can call them directly.
- `backend/cmd/harvest/main.go` — `buildQuery` (argument-resolution factored out of `main()` for testability) — extend this function (or add a sibling) rather than inlining new flag logic into `main()`.
- `apps/admin/src/views/reviews/ui/reviews-view.tsx` — `confirm(...)` + mutation pattern for a moderation action; mirror for the books-list publish action.
- `backend/internal/config/config.go` — `Config.Env`/`APP_ENV` and the existing production-only guardrail precedent (`JWT_SECRET` check in `Load()`) — mirror this precedent in `cmd/seed`, don't invent a new mechanism.

## Tests

- Backend: `admin/repository` test for `pending_review` filter; `admin/service` test for the new `DashboardCounts` value; `admin/transport/http` handler tests for the new query param and dashboard field (update the three existing fake implementations of `DashboardCounts`'s signature: `handler_test.go`, `harvest/service/handler_test.go`'s `fakeAdminRepository`, `admin/service/service_test.go`'s `fakeRepository`).
- Backend: `bookdata.BuildAuthorListQuery` table-driven unit tests (valid/invalid QIDs, empty list, default/clamped limit, generated-query shape assertions via substring/contains checks — not a full SPARQL parser).
- Backend: `cmd/harvest` argument-resolution tests extended for the new flags (mutual exclusivity, QID parsing from flag and file).
- Backend: `cmd/seed` guardrail — a small unit test around the extracted guard-check function (e.g. `shouldRefuseToRun(env string, force bool) bool`) rather than testing `main()`/DB connection directly.
- Frontend: extend existing books-list/dashboard tests (if present) for the new filter option, badge, publish action, and dashboard card; otherwise note as manual verification if the suite doesn't currently cover these views' finer interactions — don't block on adding a new test harness for this increment.
- `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`; admin app type-check/lint clean, consistent with `docs/features/admin.md`'s existing testing bar ("run focused backend tests... do not require the full repository suite").

## Documentation

- Mark this file `Status: implemented` when done.
- Update `docs/ai/features/catalog-data-import/business-analysis.md`'s OQ-005 entry to note it's resolved (Approach A, curated author-QID list) by this contract — a one- or two-line status update, not a rewrite of the analysis.
- `docs/DEPLOYMENT.md` (~line 73) and the `Makefile` `seed` target comment: update to mention the production guardrail and its override flag.
- `cmd/harvest` file header comment: extend with the author-list mode example (see Acceptance Criteria).
- No `docs/ROADMAP.md` change required (same rationale as the prior increment — this fulfills a prerequisite of the `catalog-data-import` initiative, not a standalone roadmap line item) unless the developer finds an existing "harvest" placeholder worth updating.

## Risks

- **Duplicate risk against the 19 demo-seeded books is not fully closed by this increment.** Approach A tooling reduces the chance of an operator accidentally harvesting broad/irrelevant content, but if an operator harvests an author already present in the hardcoded seed (e.g. Tolstoy), the harvest handler's `(external_source, external_id)` idempotency check can't see the seed rows (`external_source IS NULL` there) and will create a second, separate book row for the same work. Mitigation is operational (avoid re-harvesting seeded authors; the pending-review queue this contract adds gives a human a chance to catch/delete the duplicate before publishing) rather than a schema/code fix — flagged, not solved, consistent with the request's own framing ("no admin HTTP endpoint... or single-lookup" work is in scope here).
- **`DashboardCounts`' signature grows again** (6th positional return value) — mechanical but touches 6 files (repository, service, handler interface, and 3 test fakes). Low risk, just note it so the developer isn't surprised by the diff size for what looks like a small feature.
- **`pending_review` can't distinguish "never reviewed" from "published then re-hidden while still import-sourced"** (see Business Rules) — accepted simplification; would need a new timestamp/flag to fix properly, not justified by current requirements.
- **No hard ceiling on `-authors`/`-authors-file` list size** (only the query's `LIMIT` is bounded) — an operator could pass hundreds of QIDs, producing a `VALUES` clause large enough to risk a Wikidata query-complexity/timeout rejection. Not mitigated in this increment beyond documentation (recommend keeping author lists to a reasonable batch size, e.g. dozens, and running multiple harvests for larger curated lists) — flag as a follow-up if it proves a real operational problem.
- **Frontend test coverage for `apps/admin` list/dashboard views may be thin today** — this contract extends existing views rather than adding new ones, so new interactions (badge, publish button) may end up manually verified rather than automated if the existing suite doesn't already exercise these views deeply. Acceptable per `docs/features/admin.md`'s own stated testing bar, but called out so it isn't silently skipped.

## Update: author status (2026-10-06)

Harvested authors are no longer public on creation: the harvest worker resolves authors by Wikidata QID first (then by name slug) and creates missing ones as `pending`; authors an admin `rejected` act as tombstones and are never re-created or re-linked by later harvests. Public pages show only `published` authors; a published book still shows a non-published author's name as plain text. See [author-status.md](author-status.md).
