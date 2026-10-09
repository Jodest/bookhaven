# Admin Book List Filters

Status: implemented

> **Update ([book-status-model.md](book-status-model.md)):** the `visibility` filter (all/visible/hidden/pending_review) is replaced by `status` (pending/published/rejected, empty = all, invalid = 400). `source` stays independent and is ANDed with `status`.

## Goal

Let a content admin narrow the admin books list (`/books` in `apps/admin`) beyond the existing `search` (title/ISBN/description full-text) and `visibility` (all/visible/hidden/pending_review) filters, so they can find and manage books in a specific genre, by a specific author, or by catalog provenance (manually entered vs harvested) as the catalog grows.

## Scope decision (read first)

Source: direct human request during `/backlog-next` prioritization (issue #138), explicitly under-specified — "scope to be determined by feature-planner." Candidates considered: genre, author, language (original-work-language), source (manual/harvested), has-ISBN/missing-ISBN, published-year range.

**In scope — 3 additive filters:** genre, author, source (manual vs harvested). Each is independently useful for day-to-day catalog curation, each reuses an existing backend pattern or endpoint with no new infrastructure (see Existing Patterns), and together they stay a small, shippable increment — the same "defer the rest explicitly" discipline `harvest-review-queue.md` used when it deferred bulk-publish.

**Out of scope, deferred — with rationale, same as other contracts' in/out-of-scope calls:**
- **Language (original-work-language) filter** — a real, existing field with its own public-catalog filter (`language-filter.tsx`, `GET /api/v1/languages`), but lower admin-moderation value than genre/author/source for this increment; revisit if an admin workflow actually needs it.
- **Published-year range filter** — this is a public-catalog *browsing* filter (`search-and-filters.md`) more than an admin *moderation* need; two extra number inputs and params for marginal admin benefit right now.
- **Has-ISBN / missing-ISBN filter** — a genuine data-quality audit need, but narrower and better suited to its own small follow-up once cataloged volume makes ISBN gaps a real pain point.

This decision is the contract's scope boundary; a developer implementing this should not add the deferred filters "while they're in there."

## User Stories

- As a content admin, I can filter the books list to a single genre, to review and manage all books in that genre.
- As a content admin, I can filter the books list to a single author, to review and manage all of that author's books.
- As a content admin, I can filter the books list by source (all / manually added / harvested), independent of visibility, so I can e.g. audit all harvested books regardless of whether they've already been published (the existing `pending_review` visibility value only shows *hidden* harvested books, not harvested books in general).
- As a content admin, I can combine genre/author/source with the existing search and visibility filters, and the combination is reflected in the URL so it's shareable/bookmarkable (matching the existing `search`/`visibility`/`page` pattern).

## Acceptance Criteria

- [x] `GET /api/v1/admin/books` accepts three new optional query params: `genre_id` (uuid), `author_id` (uuid), `source` (one of `""`/`manual`/`harvested`), each combinable with the existing `search`/`visibility`/`page`/`limit`.
- [x] `genre_id`: only books belonging to that genre are returned.
- [x] `author_id`: only books by that author are returned.
- [x] `source=manual`: only books with `external_source IS NULL`. `source=harvested`: only books with `external_source IS NOT NULL`. `source=""`/omitted: unfiltered (current behavior).
- [x] `source=harvested` combined with `visibility=visible` returns harvested books that have already been published — a combination the current `visibility=pending_review` value cannot express (it hardcodes `is_hidden = true`).
- [x] All three new filters are no-ops when omitted — existing requests/tests without them are unaffected.
- [x] `BulkPublishBooks` (`admin/service/bulk_publish.go`) continues to build its own `BookFilter` with only `Visibility`/`Search`/`Limit` set — unaffected by the new fields (they zero-value to "no filter"), confirmed by not changing that file.
- [x] Admin UI: the books list page gets a genre select, an author select, and a source select, each writing to the URL (`genre_id`, `author_id`, `source`) via the existing `updateParams` helper, each resetting `page` to `1` on change (matching the existing `visibility` select's behavior).
- [x] Genre/author selects are populated from the existing admin `GET /api/v1/admin/genres` / `GET /api/v1/admin/authors` list endpoints (already used by `book-form.tsx` for the same purpose) — no new lookup endpoint.
- [x] Clearing a filter (selecting "All"/empty option) removes the corresponding query param, matching existing `visibility` behavior.

## Business Rules

- Genre and author filters are single-select (one `genre_id`, one `author_id` at a time) — not multi-select. This avoids the duplicate-row-from-JOIN problem multi-value genre filtering has on the public catalog (`search-and-filters.md`'s `GenreID []uuid.UUID` + `IN` join), and matches a reasonable admin "drill into one genre/author" workflow without requiring `DISTINCT` handling.
- `source` is orthogonal to `visibility`: both can be set simultaneously; the resulting `WHERE` conditions are ANDed, same as `search` + `visibility` today.
- Genre/author filters apply to currently-selected genre/author only and do not change the already-existing books-list search/visibility semantics or default sort (`created_at DESC`).

## API

`GET /api/v1/admin/books` — add three optional query params to the existing handler (`backend/internal/modules/admin/transport/http/handler.go`'s `ListBooks`):

| Parameter | Type | Description |
|---|---|---|
| `genre_id` | uuid | Only books belonging to this genre |
| `author_id` | uuid | Only books by this author |
| `source` | string | `""` (all, default) / `manual` (external_source IS NULL) / `harvested` (external_source IS NOT NULL) |

No response shape change (`listBooksResponse` unchanged). No new endpoints — genre/author dropdown options come from the already-existing `GET /api/v1/admin/genres` and `GET /api/v1/admin/authors`.

## Database

No migration needed. `genre_id`/`author_id` filters join the existing `book_genres`/`book_authors` many-to-many tables (already indexed via their composite PKs/FKs, same tables the public catalog's equivalent filter already joins). `source` filters on the existing `books.external_source` column (no new index needed at current catalog scale; revisit only if `EXPLAIN` on a representative admin query shows a problem).

### GORM Model Changes

Extend `backend/internal/modules/admin/repository/repository.go`'s `BookFilter`:

```go
type BookFilter struct {
	Search     string
	Visibility string
	GenreID    *uuid.UUID
	AuthorID   *uuid.UUID
	Source     string // "" (all), "manual", "harvested"
	Limit      int
	Offset     int
}
```

### Repository changes

In `GormRepository.ListBooks`, mirror the catalog module's existing join pattern (`backend/internal/modules/catalog/repository/repository.go`) exactly:

```go
if filter.GenreID != nil {
	query = query.Joins("JOIN book_genres ON book_genres.book_id = books.id").
		Where("book_genres.genre_id = ?", filter.GenreID)
}
if filter.AuthorID != nil {
	query = query.Joins("JOIN book_authors ON book_authors.book_id = books.id").
		Where("book_authors.author_id = ?", filter.AuthorID)
}
switch filter.Source {
case "manual":
	query = query.Where("external_source IS NULL")
case "harvested":
	query = query.Where("external_source IS NOT NULL")
}
```

Add alongside the existing `Visibility` `switch` statement; no change to the `Preload("Authors")`/`Preload("Genres")` calls, pagination, or ordering.

## Frontend

- `apps/admin/src/views/books-list/ui/books-list-view.tsx`: add `genreId`/`authorId`/`source` read from `searchParams` (same pattern as `visibility`), pass through to `adminBooksApi.list(...)`, add three `Select` controls next to the existing visibility `Select`, each calling `updateParams({ <field>: value, page: '1' })`.
- Genre/author options: fetch via `useQuery` from `adminGenresApi.list({ limit: ... })` / `adminAuthorsApi.list({ limit: ... })` — same pattern `book-form.tsx` already uses (`['admin', 'genres', 'all']` / `['admin', 'authors', 'all']` query keys), map to `{ value: id, label: name }`.
- `source` options: static `[{ value: '', label: 'Все' }, { value: 'manual', label: 'Добавлено вручную' }, { value: 'harvested', label: 'Импортировано' }]` — reuses the same `Select` component already used for `visibility`.
- `packages/api-client/src/index.ts`: extend `admin.books.list`'s params type with `genreId?: string; authorId?: string; source?: 'manual' | 'harvested' | ''`, map to `genre_id`/`author_id`/`source` query params (same camelCase-param → snake_case-query translation the rest of this method already does for `visibility`).
- `apps/admin/src/entities/book/api/index.ts`: no change — it already re-exports `api.admin.books.list` as-is.

## Existing Patterns

- `backend/internal/modules/catalog/repository/repository.go`'s `BookFilter.GenreID`/`AuthorID` JOIN pattern — copy directly into the admin repository's `ListBooks`, same JOIN clauses.
- `backend/internal/modules/admin/repository/repository.go`'s existing `Visibility` `switch` statement in `ListBooks` — add the new `Source` `switch` right alongside it, same structure.
- `apps/admin/src/views/books-list/ui/books-list-view.tsx`'s existing `visibility` `Select` + `updateParams` — copy for `source`; same URL-param-driven state approach for `genreId`/`authorId`.
- `apps/admin/src/features/manage-book/ui/book-form.tsx`'s existing `adminGenresApi`/`adminAuthorsApi` list queries used to populate the genre/author `MultiSelect` — reuse the same queries (single-select instead) to populate the new filter dropdowns, no new endpoint.
- `backend/internal/modules/admin/service/bulk_publish.go`'s `BulkPublishBooks` — confirms the new `BookFilter` fields are safe additions since it builds its own filter literal and never receives caller-supplied `GenreID`/`AuthorID`/`Source`.

## Tests

### Backend
- `backend/internal/modules/admin/repository/repository_test.go` (or new table-driven cases): `ListBooks` with `GenreID` set returns only matching books; with `AuthorID` set returns only matching books; with `Source: "manual"` excludes harvested books; with `Source: "harvested"` excludes manual books; combined `Source: "harvested"` + `Visibility: "visible"` returns only published harvested books (the case the existing `pending_review` value can't express).
- `backend/internal/modules/admin/transport/http/handler_test.go`: `ListBooks` parses `genre_id`/`author_id`/`source` query params into `BookFilter` correctly (extend the existing `lastBookFilter` assertion pattern used for `visibility`).
- `backend/internal/modules/admin/service/bulk_publish_test.go`: no behavior change expected — rerun existing tests to confirm the new `BookFilter` fields don't affect `BulkPublishBooks` (regression guard, not new test logic).
- `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`.

### Frontend
- `apps/admin/src/views/books-list/ui/books-list-view.test.tsx`: selecting a genre/author/source updates the URL and the `adminBooksApi.list` call params; clearing a filter removes it from the URL; filters combine with existing `search`/`visibility`/`page` without resetting them (except `page` itself, which resets to `1`, matching existing `visibility` behavior).

## Documentation

- `docs/ROADMAP.md`: add a line under "Completed" (or "Proposed" while in progress) linking to this contract, matching how other admin increments (e.g. `harvest-bulk-publish.md`) are tracked.
- Mark this file `Status: implemented` when done.

## Risks

- **Single-select genre/author is a deliberate simplification**, not a limitation discovered later — an admin wanting "books in genre A OR genre B" isn't supported; acceptable for this increment's scope, revisit only if requested.
- **`source` filter naming vs `external_source`'s existing per-record values** (e.g. `"wikidata"`): `source` here is intentionally a coarse manual/harvested boolean-like split, not a filter on the specific harvest source string — simpler for now; a future increment could add a third, finer-grained filter if BookHaven gains more than one harvest source in practice.
- **No new index for `external_source IS NULL/NOT NULL`**: acceptable at current catalog scale (same judgment call `search-and-filters.md` made for rating/year scans); revisit if `EXPLAIN` shows a real cost once the catalog grows substantially.
