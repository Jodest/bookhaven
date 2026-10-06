# Author Moderation Queue

Status: done

Epic: "Author moderation & import", feature 2 of 4. Builds on [author-status.md](author-status.md) (feature 1, source of truth for `authors.status`, QID, tombstones, public visibility) and reuses the books review queue patterns from [harvest-review-queue.md](harvest-review-queue.md) / [harvest-bulk-publish.md](harvest-bulk-publish.md) and the merge action from [duplicate-author-merge.md](duplicate-author-merge.md).

## Goal

Give admins one place to process harvested `pending` authors (and find legacy junk): see the context needed to decide (QID, linked books, likely duplicates), then Publish, Reject (tombstone) or Merge into an existing author, one at a time or in bulk for selected rows.

## Scope

In scope: the queue page, a read endpoint with per-author context and similar-name hints, a bulk status endpoint (also used for single-row actions), a dashboard tile, links from the harvest page, a merge dialog prefilled from a hint.

Out of scope:
- Feature 3 (import an author from Wikidata search) and feature 4 (enrichment: dates, photo, bio). Feature 3 shipped: see [author-direct-import.md](author-direct-import.md) (the "Импорт из Wikidata" panel on this page).
- **Per-harvest-job filtering.** Not derivable cheaply: asynq jobs carry no timestamps in our API, `HarvestResult` records no author IDs, and authors have no `created_by_job` column. The harvest page links to the plain queue instead; the queue sorts newest first, so the latest run's authors are on top. Revisit only if admins ask for it (it would need `HarvestResult.CreatedAuthorIDs` or a column).
- Automatic merging or auto-rejecting of any kind. Hints are display-only (Decision 3.b in `docs/ai/features/catalog-data-import/business-analysis.md` still holds).
- pg_trgm or any fuzzy-matching infrastructure. Migrations have no extensions, and none are added.
- Changing `books.is_hidden` based on an author decision.
- Deleting authors. Reject makes the author a tombstone; merge is the only path that deletes.
- Widening any of this to moderators. All of it stays admin-only.

## User Stories

- As an admin, after a harvest I open "Авторы на модерации" and see every pending author, newest first, with its Wikidata QID (link), how many books it is linked to (and how many of them are visible) and a few titles.
- As an admin, I see "possibly the same person" hints next to a pending author and can merge it into the hinted author in two clicks (merge dialog opens with that canonical author already selected).
- As an admin, I publish or reject a single author, or select several rows on the page and publish or reject them all at once.
- As an admin, I can open the rejected tab and return an author to pending or publish it (un-reject).
- As an admin, I can switch the queue to published authors with filters "has QID" / "no books" to find legacy junk created before feature 1.
- As an admin, the dashboard shows how many authors are awaiting moderation and links to the queue.

## Business Rules

1. **Queue membership** is `status` only. The default view is `status=pending`. Rejected and published are selectable tabs/filters of the same page. No new state.
2. **Publish** = `status → published`. **Reject** = `status → rejected`. **Return to moderation** = `status → pending`. Any → any transition is allowed (same as feature 1 rule 10). A no-op (same status) counts as success.
3. **Reject keeps `book_authors` links (decision).** Rationale: public payloads already render non-published authors name-only (feature 1 rule 7), links keep the admin book form truthful, and un-reject restores everything without data loss. Tombstone semantics come from feature 1 (QID match or slug match → harvest skips). Consequence: a rejected author keeps appearing as plain text on the public pages of its already-linked books. To remove it from a book, the admin edits the book or uses Merge. The reject confirmation states the number of visible books affected.
4. **Merge from the queue** reuses `POST /admin/authors/:id/merge` unchanged: the pending author is the duplicate (deleted), the chosen author is canonical. QID carry-over follows feature 1 rule 11. Merge is the correct action for "this is actually existing author X". Reject is for "not a valid author" (junk label, wrong entity).
5. **Similar-name hints** are suggestions only, computed per page and never persisted. Rule (explainable, no fuzzy infra):
   - `normalize(name)` = lowercase, `ё→е`, every char that is not a letter, digit or space → space, collapse spaces, trim.
   - `surname(name)`: if the raw name contains a comma, take the normalized text before the first comma ("Толстой, Лев" → `толстой`). Otherwise take the last token of `normalize(name)`. Surnames shorter than 3 runes produce no hints.
   - Candidate = any author that is not the row itself and has `status <> 'rejected'` (pending and published both) and has the same `surname`.
   - Match kind, highest first: `exact_name` (equal `normalize(name)`), `surname_initial` (same surname and same first rune of the first non-surname token), `surname`.
   - At most 5 hints per row, ordered by kind, then published before pending, then name. Each hint carries the candidate's status and QID. The UI flags "разные QID — вероятно, тёзки" when both authors have different QIDs.
   - Known blind spots, accepted: transliteration (Tolstoy / Толстой), pseudonyms, surname-first names without a comma.
6. **Bulk status** applies to an explicit list of author IDs (selected rows on the current page), max 100 per request. Each ID is processed independently, and a missing ID is a row error, not a request failure (ImportBooks / BulkPublishBooks report pattern). Unlike books bulk publish (filter-based), this is selection-based on purpose: author moderation is a per-row judgment, and "publish everything pending" would bypass the point of the queue. The checkbox is a native `<input type="checkbox">`, so no new `packages/ui` primitive is needed.
7. **Book context**: `books_total` = all linked books, `books_visible` = linked books with `is_hidden = false`, `sample_books` = up to 3 linked books (id, title, is_hidden), ordered by title.
8. **Legacy junk finder**: filters `qid=with|without` and `books=none` work with any status (most useful with `status=published`). No automatic reclassification of legacy authors.
9. Authorization: every endpoint is in the existing `authors := admin.Group("/authors", adminRole)` group, and mutations add `requireBearer`. Moderators get 403, unauthenticated users get 401.
10. Status changes affect public reads immediately, except the cached home calendar and precomputed similarities (feature 1 risks, unchanged).

## Acceptance Criteria

Backend
- [x] `GET /api/v1/admin/authors/moderation` returns a paginated list of `{author, books_total, books_visible, sample_books, similar}` items. Defaults: `status=pending`, `page=1`, `limit=20` (max 50). Ordered `created_at DESC, name ASC`.
- [x] Query params: `status` (pending|published|rejected; invalid → 400), `qid` (`with`|`without`|empty; invalid → 400), `books` (`none`|empty; invalid → 400), `search` (same full-text condition as `ListAuthors`). `total` honors all filters.
- [x] `books_total` / `books_visible` / `sample_books` are correct for authors with 0 books, only hidden books, and mixed books.
- [x] `similar` follows Business Rule 5 exactly: comma-format surname, `ё/е`, punctuation and initials ("Л. Н. Толстой" ↔ "Лев Толстой" → `surname_initial`), self excluded, rejected excluded, max 5, ordering. Surnames shorter than 3 runes yield `[]`. Never null (empty array).
- [x] Hints come from at most 2 extra queries per page (sample books, candidates), not N+1 per row.
- [x] `POST /api/v1/admin/authors/bulk-status` with body `{"ids": [...], "status": "published"|"rejected"|"pending"}` → 200 `{processed, updated, errors: [{author_id, name, message}]}`. The status is validated (400 on invalid). `ids` must be non-empty, ≤ 100 and valid UUIDs (400 otherwise). Duplicate IDs are deduped. An unknown ID becomes a row error and the rest proceed.
- [x] Bulk status changes **only** `status` (and `updated_at`). Name, slug, bio, QID and `book_authors` are untouched, including for reject (assert links survive).
- [x] `GET /api/v1/admin/dashboard` gains `pending_authors` (count of `status='pending'`).
- [x] 401 / 403 for unauthenticated / moderator on both new endpoints. 401 without bearer on the POST.
- [x] Existing `GET /admin/authors`, `PATCH /admin/authors/:id`, `POST /admin/authors/:id/merge` behavior is unchanged.

Frontend (`apps/admin`)
- [x] New page `/authors/moderation` ("Модерация авторов"), reachable from: the authors list header button "Очередь модерации", the dashboard tile, and the harvest page. It is admin-only via the existing `adminOnlyPaths` prefix `/authors`.
- [x] Status tabs/select (На модерации, default / Опубликованные / Отклонённые), `qid` and `books` selects, search. All of them are URL params (`status`, `qid`, `books`, `search`, `page`), following the `authors-view.tsx` `updateParams` pattern.
- [x] Each row shows: checkbox, name + status badge (`AuthorStatusBadge`), slug, QID as an external link `https://www.wikidata.org/wiki/<QID>` (`target="_blank" rel="noopener noreferrer"`) or "—", "Добавлен" date, books "N (видимых M)" + up to 3 titles linking to `/books/<id>/edit` (hidden ones marked), and a hints list (name, match label, status badge, QID, "разные QID" warning, "Это он — слить" button).
- [x] Per-row actions depend on the current status: pending → Опубликовать / Отклонить / Слить. Rejected → Вернуть на модерацию / Опубликовать. Published → Отклонить / Слить. Edit is still available via the authors list.
- [x] Publish and return-to-moderation use `confirm()`. Reject uses `confirm()` with text naming the author and, if `books_visible > 0`, saying the name stays on N published book pages without a link. All of them call `bulkStatus({ids:[id], status})`.
- [x] "Слить" opens `MergeAuthorDialog` with `duplicate` = the row. "Это он — слить" opens it with `initialCanonical` = the hint (preselected, still changeable). On success the queue, the authors list and the dashboard are invalidated, and the feedback banner shows the reassigned count.
- [x] Bulk toolbar appears when ≥ 1 row is selected: "Опубликовать выбранные (N)" / "Отклонить выбранные (N)" (for the rejected tab: "Вернуть на модерацию (N)"). It uses `confirm()`, calls `bulkStatus`, and shows "Обновлено X из N" + the first errors in the feedback banner. Selection clears on success, page change and filter change. A header checkbox selects or clears the current page.
- [x] After any mutation, `['admin','authors']` (prefix, which also covers the moderation key) and `['admin','dashboard']` are invalidated.
- [x] Dashboard: card "Авторы на модерации" = `pending_authors`, linking to `/authors/moderation`.
- [x] Harvest page: next to every existing "К очереди на проверку" link (header and per-job), a "К авторам на модерации" link to `/authors/moderation`.
- [x] `MergeAuthorDialog` gains an optional `initialCanonical` prop. The existing authors-view usage and its tests are unchanged.

## UX

- Separate page rather than a mode of `/authors` (recommended). The queue rows need extra columns (QID, books, hints), checkboxes and status-dependent actions. The authors list already hosts the create/edit form, so mixing both makes either worse. The authors list gets a header link "Очередь модерации" and keeps its status filter and badges from feature 1.
- Feedback uses the same green or destructive banner as `authors-view.tsx` / `books-list-view.tsx`. No toasts, no new dialog components (only the existing merge dialog).
- Empty states: "Нет авторов на модерации" (pending tab) or "Авторы не найдены" (other filters).

## API

`GET /api/v1/admin/authors/moderation?status=&qid=&books=&search=&page=&limit=` (admin role)
```json
{
  "items": [{
    "author": { "...authorResponse (id, name, slug, status, external_source, external_id, ...)" : "" },
    "created_at": "2026-10-06T10:00:00Z",
    "books_total": 3,
    "books_visible": 1,
    "sample_books": [{ "id": "uuid", "title": "…", "is_hidden": true }],
    "similar": [{ "author": { "...authorResponse": "" }, "match": "exact_name|surname_initial|surname" }]
  }],
  "total": 42, "page": 1, "limit": 20
}
```
Register `authors.GET("/moderation", ...)` before `/:id` routes (Echo static segments win anyway; keep it next to `authors.GET("")`).

`POST /api/v1/admin/authors/bulk-status` (admin role + bearer)
- Request: `{"ids": ["uuid", ...], "status": "published"}`
- 200: `{"processed": 2, "updated": 1, "errors": [{"author_id": "uuid", "name": "", "message": "author not found"}]}`
- 400: invalid status, empty / >100 / malformed ids, or a bad body.

`GET /api/v1/admin/dashboard`: adds `"pending_authors": number`.

Swagger annotations on both handlers. Regenerate swagger the same way as feature 1.

## Database

No migration. Existing `idx_authors_status_not_published` (000015) serves the pending/rejected filters and the dashboard count. Book counts come from a grouped subquery over `book_authors` joined to `books`. `book_authors` has no `author_id`-leading index (known risk from the merge contract). This is acceptable at current scale and is not added preemptively.

Repository queries (Postgres, portable to the SQLite test style where practical: use `SUM(CASE WHEN ...)`, not `FILTER`):
- List: `authors a LEFT JOIN (SELECT ba.author_id, COUNT(*) AS books_total, SUM(CASE WHEN b.is_hidden THEN 0 ELSE 1 END) AS books_visible FROM book_authors ba JOIN books b ON b.id = ba.book_id GROUP BY ba.author_id) s ON s.author_id = a.id`, filters `a.status = ?`, `a.external_id IS [NOT] NULL`, `s.author_id IS NULL` for `books=none`, search as in `ListAuthors`.
- Sample books: one query for the page's author IDs (`book_authors JOIN books WHERE author_id IN ?`), trimmed to 3 per author in Go.
- Hint candidates: one query `SELECT … FROM authors WHERE status <> 'rejected' AND id NOT IN (?page ids) AND (<OR of patterns>)`. For each distinct page surname `s`, the patterns are: `translate(lower(name),'ё','е') = s`, `LIKE '% ' || s`, `LIKE s || ',%'`, `LIKE s || ' ,%'` (LIKE metacharacters escaped). Then re-check every candidate in Go with the same `normalize` / `surname` functions (the SQL is only a prefilter, Go is authoritative). Put the normalization helpers in `admin/service` (pure functions, unit-tested).

## Backend

`admin/repository`
- `AuthorModerationFilter{Status, QID, Books, Search string; Limit, Offset int}`.
- `ListAuthorsForModeration(ctx, f) ([]AuthorModerationRow, int64, error)`, where `AuthorModerationRow{Author catalogDomain.Author; BooksTotal, BooksVisible int64}`.
- `ListAuthorSampleBooks(ctx, authorIDs []uuid.UUID) (map[uuid.UUID][]SampleBook, error)`.
- `FindAuthorsBySurnames(ctx, surnames []string, excludeIDs []uuid.UUID) ([]catalogDomain.Author, error)`, with the SQL prefilter above.
- `SetAuthorStatus(ctx, id uuid.UUID, status string) (found bool, err error)`: `UPDATE authors SET status=?, updated_at=NOW() WHERE id=?`, where `RowsAffected == 0` → not found. Do not use `Save` (full-row write).
- `DashboardCounts`: add the pending authors count. **Refactor to a struct** (`DashboardStats{Books, HiddenBooks, Authors, Genres, PendingReviews, PendingImportReview, PendingAuthors int64}`). A 7th positional return is past the point the earlier contract tolerated. This touches the repository, the service, the handler's `Service` interface and the 3 fakes (`admin/transport/http/handler_test.go`, `admin/service/service_test.go`, `harvest/service/handler_test.go`).

`admin/service`, new file `author_moderation.go` (one-file-per-action convention, like `bulk_publish.go` / `merge_author.go`)
- `ListAuthorModerationQueue(ctx, filter, page, limit) ([]AuthorQueueItem, int64, error)`: validates status / qid / books (new `ErrInvalidModerationFilter` → 400), clamps limit to 50, runs list + samples + candidates, computes hints.
- `BulkSetAuthorStatus(ctx, ids []uuid.UUID, status string) (BulkStatusReport, error)`: validates status (`ErrInvalidAuthorStatus`), dedupes, caps at `bulkAuthorStatusCap = 100` (`ErrBulkStatusTooLarge` → 400, decided in the handler), then per-ID `SetAuthorStatus` with continue-on-error. The report mirrors `BulkPublishReport`, and the not-found row message is `author not found`.
- Pure helpers `normalizeAuthorName`, `authorSurname`, `firstInitial`, `matchKind`.

`admin/transport/http/handler.go`: `ListAuthorModerationQueue`, `BulkSetAuthorStatus` handlers, response types (`authorQueueItemResponse`, reusing `toAuthorResponse`), and `dashboardResponse.PendingAuthors`. Router: `authors.GET("/moderation", …)` and `authors.POST("/bulk-status", …, requireBearer)`.

## Frontend

`packages/api-client/src/index.ts`
- Types: `AdminAuthorQueueItem`, `AdminAuthorHint { author: AdminAuthor; match: 'exact_name' | 'surname_initial' | 'surname' }`, `AdminAuthorSampleBook`, `AdminAuthorQueueParams { status?, qid?: 'with' | 'without', books?: 'none', search?, page?, limit? }`, `AdminAuthorBulkStatusReport`.
- `AdminDashboard.pending_authors: number`.
- `adminAuthorsApi.moderationQueue(params)` and `adminAuthorsApi.bulkStatus(ids, status)`, following the existing `merge` / `bulkPublish` fetchers.

`apps/admin` (FSD)
- `app/authors/moderation/page.tsx`: thin page, same as `app/authors/page.tsx`.
- `views/author-moderation/ui/author-moderation-view.tsx` + `index.ts`: filters, table, selection state, bulk toolbar, feedback, merge dialog state. Query key `['admin','authors','moderation', params]`.
- `features/moderate-author/` (new slice): `useAuthorStatusMutation` (wraps `bulkStatus` + invalidations) and the confirm-text builders, so the view stays thin. Keep it inside the view if it ends up tiny (developer's call, but one mutation hook only).
- `entities/author`: re-export the new API methods. Optionally add a small `author-hints` UI (list of hints with match label) or keep it in the view. Add a `WikidataLink` / QID cell helper only if reused.
- `features/merge-author/ui/merge-author-dialog.tsx`: `initialCanonical?: Pick<AdminAuthor, 'id' | 'name'>` seeds the `canonical` state (narrow the state type accordingly). Also invalidate `['admin','dashboard']` on success.
- `views/authors/ui/authors-view.tsx`: header link "Очередь модерации" → `/authors/moderation`.
- `views/dashboard/ui/dashboard-view.tsx`: new card in the `cards` array.
- `views/harvest/ui/harvest-view.tsx`: second link next to both "К очереди на проверку" links.
- typedRoutes: use the existing `as any` href convention. The CI frontend build is the typedRoutes gate (do not run a native `next build`).

## Existing Patterns

- `admin/service/bulk_publish.go` + `import.go`: per-row continue-on-error report, cap constant.
- `admin/service/merge_author.go` + `features/merge-author`: merge action and dialog, reused, not reimplemented.
- `admin/repository` `ListAuthors` (search/status/pagination) and `DashboardCounts`.
- `views/books-list/ui/books-list-view.tsx`: per-row publish + bulk button + `confirm()` + feedback banner + query invalidations.
- `views/authors/ui/authors-view.tsx`: URL-param filters (`updateParams`), `AuthorStatusBadge`, `AUTHOR_STATUS_LABELS`.
- `views/dashboard/ui/dashboard-view.tsx`: `cards` array.
- Feature 1 SQLite real-SQL tests (`admin/repository/merge_qid_sqlite_test.go`): the template for the new repository tests.

## Tests

Backend (run via `docker compose run`; never `migrate down` or reset the DB):
- `admin/service` pure helpers, table-driven: normalize (`ё`, punctuation, initials, extra spaces), surname (comma format, single token, < 3 runes), match kinds, ordering, cap of 5, rejected and self excluded.
- `admin/service` `ListAuthorModerationQueue` with fakes: filter validation (400 sentinels), limit clamp, hints assembled from candidate results, empty `similar` = `[]`.
- `admin/service` `BulkSetAuthorStatus`: all succeed, unknown ID → row error and others proceed, invalid status, >100 → sentinel with nothing changed, dedupe, empty list.
- `admin/repository` SQLite tests: list filters (status, qid with/without, books=none, search where feasible), `books_total` / `books_visible` counts, sample books ≤ 3 per author, the surname prefilter returns the comma-format and initials cases, `SetAuthorStatus` changes only status, keeps `book_authors`, and reports not-found. Postgres-only bits (`plainto_tsquery`) are checked via SQL text or skipped, as in feature 1.
- `admin/transport/http`: both handlers (shapes, 400 cases, auth 401/403), dashboard `pending_authors`. Update all `DashboardCounts` fakes.

Frontend (vitest, existing patterns):
- `author-moderation-view.test.tsx`: renders rows with QID link href, books summary and hints. Per-row actions vary by status. Confirm-gated publish/reject calls `bulkStatus` with the right args, and the reject confirm text mentions visible books. Selection + bulk toolbar + feedback "Обновлено X из N". Selection resets on filter change. The URL params update.
- `merge-author-dialog.test.tsx`: `initialCanonical` preselects and enables confirm. Existing tests unchanged.
- `dashboard-view.test.tsx`: new card + href. `harvest-view.test.tsx`: authors queue link present. `authors-view.test.tsx`: header link.

## Documentation

- This file → `Status: done` + Implementation Notes on completion.
- `docs/features/author-status.md`: one-line pointer that feature 2 shipped the queue (its "Out of scope" bullet).
- `docs/features/duplicate-author-merge.md`: note that similar-name hints now exist in the moderation queue (display-only, per Business Rule 5), which supersedes its "no detection UI" call for the queue only, with no auto-merge.
- `docs/features/harvest-job-report.md`: note the authors-queue link and that per-job author filtering is deliberately not built.
- `docs/ROADMAP.md`: tick epic item 2.
- Swagger regenerated.

## Risks

- **Hint false positives and negatives.** Shared surnames (Толстой Лев / Толстой Алексей) produce `surname` hints. Transliteration and pseudonyms are missed. Mitigated by the match label, the status / QID display and the "разные QID" warning. The human always decides.
- **Merge into a canonical author that has a different QID** drops the duplicate's QID (feature 1 rule 11). The next harvest of that QID then recreates a pending author. The hint warning surfaces this, and the admin should usually treat different QIDs as namesakes (publish both) rather than merge.
- **Reject keeps links**, so rejected names keep showing as plain text on already-linked published books. This is intentional and reversible. The admin edits the book to drop the author if needed.
- **Prefilter query cost.** OR'd `LIKE '%…'` patterns scan `authors` once per page load. Fine at thousands of rows. If authors grow to 100k+, revisit with an expression index or pg_trgm (a separate decision).
- **Slug-only tombstones.** A rejected author without a QID only blocks harvest by exact slug. A differently spelled label recreates a pending author (inherent to feature 1).
- **Concurrent moderation** (two admins, or a merge racing a bulk status): last write wins, and a merged-away ID shows up as a "not found" row error. No locking, consistent with the admin app.
- **The `DashboardCounts` refactor** touches 6 files, all mechanical.
- **No per-job filter.** After two quick harvests the queue mixes both runs. Newest-first ordering mitigates this.

## Implementation Notes

- No migration. Repository code lives in `admin/repository/author_moderation.go`; service in `admin/service/author_moderation.go`; handlers in `admin/transport/http/handler.go`; routes registered next to `authors.GET("")` in `internal/transport/http/router.go`.
- `DashboardCounts` keeps its method name and now returns `repository.DashboardStats` (repository, service, handler `Service` interface and all fakes updated, including the three extra admin service fakes in `bulk_publish_test.go` / `import_test.go` / `merge_author_test.go`).
- List query: `authors` + the grouped `book_authors JOIN books` subquery, scanned into a flat row struct (not the GORM model, which has a many2many field). Sample books use one `ROW_NUMBER() OVER (PARTITION BY author_id ORDER BY title, id)` query (`rn <= 3`, also trimmed in Go).
- Hint prefilter uses `replace(lower(name), 'ё', 'е')` instead of `translate()` (same result for one character; `replace` also exists in SQLite, so the SQL is exercised by the SQLite tests) and `LIKE ... ESCAPE '\'` with `%`, `_` and `\` escaped.
- Candidate pool = prefilter results (page IDs excluded in SQL, as specified) **plus the page's own rows** in Go, so two pending duplicates on the same page hint at each other (Business Rule 5: only the row itself is excluded). Hints sort by kind, then published first, then name, then ID; capped at 5; always `[]`, never null.
- `surname_initial` compares the first rune of the first given-name token: for the comma format the tokens after the comma, otherwise all tokens except the last.
- Bulk status: the handler validates status, non-empty `ids` and UUID syntax (400) before calling the service; the service validates status again, dedupes, then applies the 100 cap to distinct IDs (`ErrBulkStatusTooLarge` → 400). `SetAuthorStatus` uses `Updates(map{status, updated_at})`; a no-op still counts as found/updated (Postgres reports matched rows). Row errors carry an empty `name` (the service does not load authors); the UI fills names from the current page.
- `GET /admin/authors/moderation` clamps `limit` to 50 in both the handler (echoed in the response) and the service; invalid `status`/`qid`/`books` → 400 `invalid moderation filter: ...`.
- Frontend: `features/moderate-author` holds `useAuthorStatusMutation` (wraps `adminAuthorsApi.bulkStatus`, invalidates `['admin','authors']` + `['admin','dashboard']`) and the confirm/summary text builders. The view is `views/author-moderation`, page `app/authors/moderation/page.tsx`. Status tabs are buttons (`aria-pressed`); pending is the default and is not written to the URL. Bulk toolbar: pending → publish + reject, published → reject, rejected → return to moderation + publish. `WikidataLink` and the hint list stay inside the view (not reused elsewhere).
- `MergeAuthorDialog` gained `initialCanonical` (state narrowed to `Pick<AdminAuthor, 'id' | 'name'>`) and now also invalidates `['admin','dashboard']`.
- Harvest page: the per-job author-queue link follows the existing per-job condition (completed job with `created > 0`).
- Swagger regenerated (`swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`).
- Tests: service helpers/queue/bulk (`admin/service/author_moderation_test.go`), SQLite real-SQL repository tests (`admin/repository/author_moderation_sqlite_test.go`, search is Postgres-only and not covered there), handlers + 401/403/bearer via the real middleware chain (`admin/transport/http/handler_author_moderation_test.go`), dashboard `pending_authors`; admin vitest for the view, merge dialog `initialCanonical`, dashboard tile, harvest and authors-list links.
- Live check on the local stack: queue/filters/search/400s, bulk status with an unknown ID and duplicates, 401 without auth, dashboard counter, and a Cyrillic `surname_initial` hint between a temporary pending author and an existing published one. The temporary author was then merged away (0 books reassigned); the author count and pending count are back to their original values.
