# Genre Status (pending / published / rejected) + Wikidata QID

Status: done

Issue: #301. Epic #290 "Genre moderation and Wikidata import", feature 1 of 3 (phase 7 of [admin-navigation-restructure.md](admin-navigation-restructure.md)). Modeled on [author-status.md](author-status.md).

## Goal

Stop unvetted genres (harvest/import autocreated junk) from appearing on the public site. Genres get a `status` lifecycle and an optional Wikidata identity (`wikidata_qid`). Harvest matches by QID first, new genres land in `pending`, rejected genres are tombstones, and every public read shows only `published` genres. Books stay publishable regardless of their genres' status; a non-published genre is simply omitted.

## Out of scope (later features of the epic)

- **#302 moderation queue + merge**: `/moderation/genres` (replaces the placeholder), bulk publish/reject, merge duplicate genres, nav moderation badge + tab badge for genres.
- **#303 Wikidata genre import**: `/import/genres` (replaces the placeholder), search/import genres by QID.
- Any change to book publication (`books.is_hidden`) based on genre status.
- Open Library subjects as a genre source. Harvest keeps ignoring them; do not add them.

What #302/#303 need from this feature is listed under "Hand-off to #302 / #303".

#302 has shipped: [genre-moderation-queue.md](genre-moderation-queue.md). Its merge keeps the source genre as a rejected alias row (`merged_into_id`) instead of deleting it, and the harvest/file-import resolvers follow aliases to their target.

## User Stories

- As a reader, I only see genres an admin has published: genre chips, footer, catalog filter, random-book picker and the genres on book cards and book pages.
- As an admin, harvested and imported genres stay hidden until I publish them. I can see and filter genre status in `/catalog/genres` and change status and QID in the edit form.
- As an admin, a genre I rejected is never recreated or re-linked by future harvests (tombstone by QID or slug).

## Business Rules

1. `genres.status` is one of `pending`, `published`, `rejected`. NOT NULL. **Every existing genre is migrated to `pending`** (decided by the human). As a result, the public site shows no genres at all until an admin publishes some. This is acceptable because there is no prod, only the local compose stack. After migrating, publish the wanted genres via `/catalog/genres`.
2. `genres.wikidata_qid` is nullable. It is unique when not null, and stores a bare QID matching `^Q[0-9]+$`. The service normalizes it (trim, uppercase) and validates it. The DB has no format CHECK, same as authors.
3. Creation paths and the resulting status:
   | Path | Status | Matching |
   |---|---|---|
   | `POST /admin/genres` (manual, `/catalog/genres`) | `published` (a `status` in the body is ignored) | n/a; optional `wikidata_qid` accepted |
   | Admin JSON/CSV file import (`admin/service/import.go` -> `ResolveOrCreateGenreIDsByNames`) | `pending` | slug only |
   | Harvest worker (`harvest/service/handler.go` `processRecord`) | `pending` | QID first, then slug (rule 5) |
   | Seed (`cmd/seed`) | `published`, set explicitly in Go | n/a |
   | Raw SQL inserts without a status | `pending` (DB default) | n/a |
   - **Why manual create is `published`:** this mirrors author rule 2. An admin typing a genre into the catalog page has deliberately created a vetted entry, and making it `pending` would force a second "publish" click for the same person. Moderation exists to catch *automatic* sources.
   - **Why file import is `pending` (unlike authors, where it is `published`):** this is a human decision. Import files carry free-text genre labels that easily duplicate existing genres ("роман"/"романы"), so they need review.
   - **Why the DB default is `pending` (unlike authors):** unknown inserts are unvetted. Go code must always set `Status` explicitly and never rely on the default. The default also performs the rule-1 backfill.
4. Admin may change status through `PATCH /admin/genres/:id` (any value to any value). `wikidata_qid` on create or update:
   - omitted or `null`: unchanged (on create: none)
   - `""`: clears the QID
   - a valid QID: sets it
   - an invalid QID: 400 (`ErrInvalidQID`, the existing error)
   - a QID owned by another genre: 409 (`ErrDuplicateGenreQID`)
5. **Harvest genre resolution** applies to each genre ref `{name, qid?}`, in order:
   1. If `qid` is present, look it up by `wikidata_qid`.
      - Found and `rejected`: **tombstone, skip it.** No `book_genres` row, nothing is created.
      - Found and `pending` or `published`: link it. A `pending` genre matched by QID is reused, never duplicated.
   2. Otherwise, or if not found, look it up by `slugify(name)`.
      - Found with no QID while the ref has a `qid`: **backfill** `wikidata_qid`. Do this only if no other genre already owns that QID (step 1 missed, so this only guards against a race; on a unique violation, skip the backfill and still link). Backfill never changes status.
      - Found and `rejected`: skip it (the tombstone applies on any match path). Do the QID backfill first, as above, so future runs match by QID.
      - Found with a **different** QID: link the existing genre and leave it untouched. Do **not** create a namesake genre (this deliberately differs from authors). Log at info level with both QIDs. Genres are a small vocabulary, and two Wikidata items with the same ru label are near-duplicates that #302 merge resolves. A `-q123` genre twin would only add junk.
      - Found with the same QID, or the ref has no `qid`: link it.
   3. Not found: create `{name, slug: slugify(name), status: pending, wikidata_qid: qid or NULL}` and link it.
   - Empty names are skipped. Duplicates by resolved ID are collapsed.
   - If every genre is skipped, the book is still created (hidden, in the review queue) with no genres. Log one info line listing the skipped refs. No new harvest-report counters.
6. **File import** keeps slug-only matching (no QID) and creates missing genres as `pending`. A slug-matched genre is linked whatever its status, including `rejected`, because it is the admin's explicit input and it is invisible publicly while not published. This mirrors author rule 5. The rejected-skip in rule 5 applies to harvest only.
7. **Public visibility:** every public read returns only `published` genres. Non-published genres on a book are omitted from the `genres` array. Filtering by a non-published genre ID gives an empty result (200, total 0). Public response shapes do not change, and no `status` field appears publicly.
8. Similar books count **published genres only**. This applies to both the shared-genre signal and the candidate's genre count (Jaccard denominator), in the live query and in the precomputed job. While every genre is pending (right after the migration), similarity relies on authors only.
9. Admin reads see every status everywhere, and admin genre payloads always include `status` and `wikidata_qid`.

## Acceptance Criteria

DB
- [x] Migration `000017_genres_status_wikidata_qid` adds `status` (default `pending`, CHECK), `wikidata_qid` and a partial unique index. After `migrate up`, every existing genre is `pending`. The down migration drops these (provided per convention; **never run by agents**).

Harvest / Wikidata
- [x] The three query builders in `pkg/bookdata/wikidata.go` (`buildTitleAuthorQuery`, `buildISBNQuery`, `BuildAuthorListQuery`) return genre QID paired with its ru label. The ru-label FILTER is kept.
- [x] The parser fills `ExternalBookRecord.GenreRefs []ExternalGenre{Name, QID}` and `Genres` (names) from the same pairs. An invalid or missing QID keeps the name with an empty QID.
- [x] Open Library records leave `GenreRefs` empty. Harvest still never uses OL subjects as genres.
- [x] A new harvested genre is created `pending` with its QID.
- [x] A QID match wins over a slug match (a renamed genre is still matched).
- [x] A QID-matched `pending` genre is reused, not duplicated.
- [x] A QID-matched or slug-matched `rejected` genre gets no link and nothing is created. The book is still created hidden.
- [x] A slug match without a QID gets the QID backfilled, with status unchanged.
- [x] A slug match with a different QID links the existing genre, untouched, and creates no new genre.
- [x] Re-running the same harvest creates no duplicate genres.

Import
- [x] File import creates missing genres as `pending`, matched by slug only.

Admin API
- [x] `POST /admin/genres` creates the genre as `published` and ignores `status` in the body. An optional `wikidata_qid` is validated and checked for uniqueness.
- [x] `GET /admin/genres` items include `status` and `wikidata_qid`. It accepts an optional `?status=pending|published|rejected`; an invalid value returns 400.
- [x] `PATCH /admin/genres/:id` accepts an optional `status` (invalid: 400) and `wikidata_qid` (rule 4).
- [x] Genres embedded in admin book responses include `status` and `wikidata_qid`.
- [x] `GET /admin/dashboard` includes `pending_genres` (count of `status='pending'`). `genres` still counts all statuses.

Public
- [x] `GET /genres` returns only `published` genres.
- [x] `GET /books?genre_id=<non-published>` returns 200 with an empty list and total 0. Mixed IDs filter by the published ones only.
- [x] `GET /home/random?genre_id=<non-published>` returns `{book: null}`.
- [x] No public book payload contains a non-published genre. This covers book detail, catalog list, author page books, similar books, every home block, public and own shelves, public and own lists, and user profile shelves/lists.
- [x] Similar books ignore non-published genres (rule 8), live and precomputed.

Admin UI
- [x] `/catalog/genres` shows a status badge column, a status filter (`?status=` URL param) and a QID column (linked to wikidata.org).
- [x] The genre form has a QID input on both create and edit (client-side `^Q[0-9]+$` hint; the server stays authoritative). The status select appears on edit only.
- [x] The book form genre picker marks non-published genres, like the author picker does.
- [x] The Overview "Модерация" group has a "Жанры на модерации" tile = `pending_genres`, linking to `/catalog/genres?status=pending`.
- [x] The sidebar moderation badge is **unchanged** in this feature (see Frontend).

## API

Public: **no shape changes.** `GenreResponse {id, name, slug}` stays the same, and no `apps/web` type changes are needed.

Admin (`admin/transport/http/handler.go`):
- `genreResponse` gains `status: "pending"|"published"|"rejected"` and `wikidata_qid: string|null` (not omitempty). It is also used for genres embedded in admin books.
- `GET /api/v1/admin/genres?status=`: validate in the handler (400) and again in the service (`ErrInvalidGenreStatus`), like authors.
- `createUpdateGenreRequest` gains `status *string` (ignored on create) and `wikidata_qid *string`. Errors: `ErrInvalidGenreStatus` gives 400, `ErrInvalidQID` gives 400, `ErrDuplicateGenreQID` gives 409.
- `dashboardResponse` gains `pending_genres`.
- Authorization is unchanged: the `adminRole` group plus `requireBearer` on mutations. Moderators get 403. Do not widen.

## Database

`backend/migrations/000017_genres_status_wikidata_qid.up.sql`:
```sql
ALTER TABLE genres ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE genres ADD CONSTRAINT chk_genres_status CHECK (status IN ('pending','published','rejected'));
ALTER TABLE genres ADD COLUMN wikidata_qid VARCHAR(32);
CREATE UNIQUE INDEX idx_genres_wikidata_qid ON genres(wikidata_qid) WHERE wikidata_qid IS NOT NULL;
CREATE INDEX idx_genres_status_not_published ON genres(status) WHERE status <> 'published';
```
`.down.sql` drops the indexes, the constraint and the columns in reverse order.

**Agents must NEVER run `migrate down`, drop/truncate tables, reset volumes or run any other destructive DB command.** `migrate down` without a count rolls back ALL migrations and wipes the local DB. Apply with `migrate up` only, or leave applying to the human.

Domain `catalog/domain/genre.go` gains:
- `Status string` with `gorm:"type:varchar(20);not null;default:'pending'"`
- `WikidataQID *string` with `gorm:"column:wikidata_qid"`. The explicit column tag is required because GORM naming would otherwise produce `wikidata_q_id`.
- constants `GenreStatusPending/Published/Rejected`
- `func (g Genre) IsPublished() bool`
- `func IsValidGenreStatus(string) bool`
- `func PublishedGenres([]Genre) []Genre`, a filter helper for mappers

Seed: set `Status: published` on the seed genres. `FirstOrCreate` does not touch existing rows, so a local DB that was already seeded keeps them `pending` per rule 1.

## Backend changes

`pkg/bookdata`
- `bookdata.go`: add `type ExternalGenre struct { Name, QID string }` and `ExternalBookRecord.GenreRefs []ExternalGenre`. Keep `Genres []string`, which admin lookup, import and Open Library use.
- `wikidata.go`: in all three builders, replace `GROUP_CONCAT(DISTINCT ?genreLabel; ...)` with the author-style paired aggregate `(GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?genre), "/entity/"), "\t", ?genreLabel); separator="|") AS ?genres)`. Keep the `?genre rdfs:label ?genreLabel . FILTER(LANG(?genreLabel) = "ru")` lines. Parse with the existing `parseAuthorPairs` logic: generalize it to a shared pair parser and fill `GenreRefs`, with `Genres` = the names of `GenreRefs`.

`admin/service`
- `harvest_resolve.go`:
  - `ResolveOrCreateGenreIDsByNames` (import): create missing genres as `pending`; link slug matches as-is (rule 6).
  - Add `GenreRef{Name, QID}`, `ResolveOrCreateHarvestGenres(ctx, []GenreRef) ([]string, error)`, plus a single-ref `resolveHarvestGenre` that returns `GenreResolution{Genre, Outcome, Match, QIDBackfilled}`. Use `GenreOutcomeCreated/Existing/Rejected` and `GenreMatchQID/Slug`, mirroring `AuthorResolution`, so #303 can reuse it.
- `service.go`:
  - `GenreInput` gains `Status *string` and `WikidataQID *string`.
  - Add an internal `createGenre(ctx, input, genreIdentity{status, qid})`. It keeps the existing validation, slugify and `lowercaseFirstRune` logic. `CreateGenre` forces `published`, and the resolvers pass `pending`.
  - QID normalization reuses `importQIDPattern` and `ErrInvalidQID` from `author_import.go`. Move them to a shared helper if convenient.
  - New errors `ErrInvalidGenreStatus` and `ErrDuplicateGenreQID`. A unique-index violation on `wikidata_qid` maps to `ErrDuplicateGenreQID`, not `ErrDuplicateSlug`: inspect the constraint name or pre-check with `GetGenreByQID`.
  - `ListGenres(ctx, search, status, page, limit)`.
  - `DashboardStats.PendingGenres`.
- `admin/repository`:
  - add `GetGenreByQID(ctx, qid)` and `SetGenreQID(ctx, id, qid)`
  - `ListGenres` gains a status filter
  - `CreateGenre`/`UpdateGenre` persist `Status` and `WikidataQID` (`Save` already writes all fields)
  - `DashboardCounts` counts pending genres
  - Update every fake of the repository/service interfaces: `admin/transport/http/handler_test.go`, `admin/service/service_test.go`, `harvest/service/handler_test.go`.

`harvest/service/handler.go` `processRecord`: call `ResolveOrCreateHarvestGenres(ctx, harvestGenreRefs(enriched))`. `harvestGenreRefs` mirrors `harvestAuthorRefs`: use `GenreRefs`, falling back to `Genres` as QID-less refs. Enrichment must not touch genres. Update the package doc comment.

### Public read paths (every one must be covered)

| # | Path | File | Change |
|---|---|---|---|
| 1 | `GET /genres` (home chips, footer, catalog filter, random picker) | `catalog/repository/repository.go` `ListGenres` (~180) | `WHERE status='published'` |
| 2 | `GET /books?genre_id=` | `catalog/repository/repository.go` `ListBooks` (~90) | add `EXISTS (SELECT 1 FROM genres g WHERE g.id = book_genres.genre_id AND g.status = 'published')`, same style as the author filter at ~100 (avoid ambiguous columns) |
| 3 | `GET /home/random?genre_id=` | `home/repository/repository.go` (~171) | add `JOIN genres` / `status='published'` to the EXISTS subquery |
| 4 | Book embeds: catalog list, book detail, author page `Books.Genres`, similar books, all home blocks (`GetBooksByIDs` -> `ToBookListItemResponse`) | `catalog/transport/http/handler.go` (`ToBookListItemResponse` ~184, detail mapper ~208) | map `domain.PublishedGenres(b.Genres)` |
| 5 | Shelves (own, public, profile) | `shelves/transport/http/handler.go` (~142) | same helper |
| 6 | Lists (own, public, profile) | `lists/transport/http/handler.go` (~170) | same helper |
| 7 | Similar source genres (live + precomputed job) | `catalog/repository/repository.go` `GetSimilarSource` (~254) | join `genres`, `status='published'` |
| 8 | Similar candidate genre count | `ListSimilarCandidates` (~325) `genre_count` subquery | count only published genres (join `genres`) |

Filtering is done at the mapper level (rows 4-6), so every current `Preload("Genres")` site (catalog x4, shelves, lists) is covered by one choke point, including future preload sites. The home review feed has no genres. Search (`ListBooks` FTS) does not touch genres. Admin repository reads stay unfiltered. Before finishing, the developer must re-grep `Genres`, `book_genres`, `FROM genres` and `genre_id` in `backend/internal/modules` to confirm no other public path exists.

## Frontend

`packages/api-client/src/index.ts`
- Add `export type AdminGenreStatus = 'pending' | 'published' | 'rejected'`.
- `AdminGenre` gains `status: AdminGenreStatus` and `wikidata_qid: string | null`.
- `AdminGenreInput` gains `status?: AdminGenreStatus` and `wikidata_qid?: string | null`. Send `""` to clear.
- The admin genres list params gain `status?`.
- `AdminDashboard` gains `pending_genres: number`. Update every test fixture of `AdminDashboard`: dashboard-view, nav-counters, pending-moderation.
- The public `Genre` type is unchanged.

`apps/admin`
- `entities/genre/ui/genre-status-badge.tsx` (`GenreStatusBadge`, `GENRE_STATUS_LABELS`): copy the `AuthorStatusBadge` styling, with amber outline for pending and destructive for rejected. Export it from `entities/genre`.
- `views/genres/ui/genres-view.tsx`:
  - add Status and QID columns
  - add a status filter select bound to the `?status=` URL param; include it in the query key and reset `page` when it changes
  - `startEdit` prefills `status` and `wikidata_qid`
  - after create, show "Жанр создан" (it is published)
- `features/manage-genre/ui/genre-form.tsx`: add a QID input on both create and edit, and a status select on edit only. Show server 400/409 errors through the existing error banner.
- `features/manage-book/ui/book-form.tsx`: mark non-published genres in the genre picker the same way authors are marked ("Название · На модерации").
- `views/dashboard/ui/dashboard-view.tsx`: add the tile "Жанры на модерации" -> `/catalog/genres?status=pending` in the Moderation group, and update the skeleton count to 4.
- **Nav badge:** `entities/dashboard/lib/pending-moderation.ts` and `features/nav-counters/lib/build-nav-counters.ts` stay unchanged. `pending_genres` joins the sidebar moderation badge and gets a `moderation.genres` tab badge in **#302**, when `/moderation/genres` becomes a real queue. A badge now would point to a placeholder page and could not be cleared from that section.

`apps/web`: no code changes. The genre list simply returns fewer items. Spot-check that the home chips, footer and random-picker render fine with an empty list (they already handle `[]`).

## Hand-off to #302 / #303

- #302 merge: move `book_genres` from the duplicate to the canonical genre (dedupe books that already have it) and carry the QID if the canonical has none. **Superseded during #302 planning:** the duplicate is *not* deleted; it becomes a `rejected` alias row with `merged_into_id` (migration 000018) so both its slug and QID keep resolving to the canonical genre, and the resolvers follow the alias. See [genre-moderation-queue.md](genre-moderation-queue.md). Bulk status endpoint. Moderation queue with book counts. Nav badge and dashboard tile retarget to `/moderation/genres`.
- #303 import: reuse `GetGenreByQID`, `createGenre` (pending plus QID) and `resolveHarvestGenre`/`GenreResolution` outcomes for the single-genre import result. Validate QIDs with the same normalization.

## Existing Patterns

- `docs/features/author-status.md`, migration `000015_authors_status_external_id`, `catalog/domain/author.go` (`IsPublished`, `IsValidAuthorStatus`).
- `admin/service/harvest_resolve.go` `ResolveHarvestAuthorRefs` / `resolveHarvestAuthor` (structure and logging).
- `admin/service/author_import.go` QID normalization (`importQIDPattern`, `ErrInvalidQID`).
- Public `author_id` EXISTS filter in `catalog/repository/repository.go` `ListBooks`.
- Admin `entities/author/ui/author-status-badge.tsx`, `views/authors/ui/authors-view.tsx` status filter, the author picker marking in `book-form.tsx`.
- `DashboardStats` struct plus the `pending_authors` tile (author-moderation-queue.md).

## Tests

Backend: run via `docker compose run`, only the changed packages. No migration is applied by tests. Run gofmt only on changed files.
- `bookdata`:
  - genre pair parsing: several genres, labels with spaces/punctuation, missing/invalid QID, empty aggregate
  - all three builders contain the paired genre aggregate and still the ru FILTER
  - `Genres` equals the names of `GenreRefs`
  - OL records have empty `GenreRefs`
- `admin/service` resolver unit tests (fakes):
  - new genre -> pending + QID
  - a QID match beats a slug match
  - QID-matched pending genre -> reused
  - QID-matched rejected genre -> skipped, nothing created
  - slug-matched rejected genre -> skipped, QID backfilled
  - slug match without QID -> backfilled, status unchanged
  - slug match with a different QID -> linked, untouched, nothing created
  - no QID -> slug path
  - dedupe
  - all skipped -> empty result
- `admin/service`:
  - `CreateGenre` forces published and accepts/validates a QID
  - import creates pending genres and links a rejected genre on slug match
  - update status: valid / invalid / omitted
  - QID: set / clear / invalid / duplicate (409 error)
  - list status filter
  - `DashboardCounts` `PendingGenres`
- `harvest/service` `processRecord`: uses `GenreRefs`, falls back to `Genres`; an all-rejected record still creates the hidden book.
- `catalog/repository`: real-SQL SQLite tests (new `genre_status_sqlite_test.go`, style of `author_status_sqlite_test.go`):
  - `ListGenres` published-only
  - `genre_id` filter: non-published / mixed
  - `GetSimilarSource` excludes non-published genres
  - `ListSimilarCandidates` genre_count counts published only
- `home/repository` random genre filter: SQLite if feasible, otherwise an SQL-text check (like `calendar_status_test.go`).
- Mappers (catalog list and detail, shelves, lists): a book with pending, rejected and published genres emits only the published one. Also unit-test `PublishedGenres`.
- Admin handler:
  - `status`/`wikidata_qid` in responses
  - `?status=` 400
  - PATCH status/QID 400/409
  - `pending_genres` in the dashboard

Frontend (admin vitest):
- genres view: badge, status filter updates the URL, QID column
- genre form: status select only on edit, QID sent on create and edit, `""` clears it
- dashboard tile `pending_genres`
- book-form picker marker
- update `AdminDashboard` fixtures

The CI frontend build is the gate for typedRoutes. Do not run a native `next build`.

## Documentation

- This contract: set `Status: done` and add Implementation Notes when finished.
- `docs/features/admin-navigation-restructure.md` phase 7 -> links here (done during planning).
- `docs/ROADMAP.md` #290 line -> links here (done during planning). Tick it when #301-#303 ship.
- `docs/features/harvest-review-queue.md`: note that harvested genres are now `pending`, QID-matched and tombstoned.
- Admin import docs/README section (if any): imported genres are `pending`.
- Regenerate Swagger: `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`.

## Risks

- **Public site shows no genres right after migrating** (rule 1): the home chips, footer and catalog genre filter are empty, and similar books lose the genre signal until genres are published and the next daily similarity recompute runs. This is intended. There is no prod. The admin publishes the curated genres in `/catalog/genres` (bulk publish arrives in #302).
- **A missed public path leaks genres.** This is mitigated by the enumerated table, mapper-level filtering and the final grep.
- **Cached home selections / precomputed similarities** reflect status changes only after the cache TTL or the daily recompute. Book embeds are re-hydrated per request, so the genre chips on cards update immediately.
- **SPARQL aggregate change**: if the paired encoding misbehaves, the parser degrades to QID-less names and slug matching, without failing. Verify once manually against live Wikidata, as was done for authors.
- **Slug match with a different QID** links books to a possibly different concept with the same ru label. This is accepted (rule 5) and resolved by #302 merge.
- **File import into a rejected genre** creates links that stay invisible publicly. This is intentional (rule 6).
- **Concurrent harvests** may race on the unique slug/QID: that record fails and is reported, and the next run resolves it. A race on the backfill unique violation is swallowed (rule 5.2).
- **The local DB was already seeded**, so its seed genres stay `pending` (`FirstOrCreate` does not update them). Publish them manually.

## Implementation Notes

- Migration `000017_genres_status_wikidata_qid` is created but **not applied** by the developer agent; apply it with `migrate up` (never `migrate down`). After applying, every existing genre is `pending` and the public site shows no genres until some are published in `/catalog/genres`.
- `domain.Genre` gained `Status`, `WikidataQID` (`gorm:"column:wikidata_qid"`, verified by a real-SQL test), `IsPublished()`, `IsValidGenreStatus()` and `PublishedGenres()` (always returns a non-nil slice).
- `bookdata`: the author pair parser was generalized to `parseQIDPairs`; `parseAuthorPairs`/`parseGenrePairs` wrap it. A plain label aggregate (no tab) still parses as QID-less names.
- Service: all genre creation goes through `createGenre(ctx, input, genreIdentity{status, qid})`. `CreateGenre` validates/normalizes the optional QID (reusing `importQIDPattern`/`ErrInvalidQID`) and forces `published`; file import passes `pending`; harvest passes `pending` + QID. QID uniqueness is pre-checked with `GetGenreByQID` (own QID allowed on update), and a Postgres unique violation whose constraint name contains `wikidata_qid` maps to `ErrDuplicateGenreQID` (anything else stays `ErrDuplicateSlug`). `UpdateGenre` keeps its existing behavior of not lowercasing the name.
- Harvest: `ResolveOrCreateHarvestGenres` / `resolveHarvestGenre` return `GenreResolution{Genre, Outcome, Match, QIDBackfilled}`. A unique violation on the slug-match backfill is swallowed (logged, still linked). The different-QID slug match and the tombstone skips are logged at info level.
- Public filtering: `ListGenres` (`status = 'published'`), `ListBooks?genre_id=` (`EXISTS` on `genres g`, like the author filter), home random pool (`poolGenreFilter` constant with `JOIN genres`), `GetSimilarSource` (join `genres`) and the `ListSimilarCandidates` `genre_count` subquery (join `genres`, named param `@published_genre`). Book embeds are filtered in the catalog (`ToBookListItemResponse`, `toBookResponse`; author page books reuse the former), shelves and lists mappers. Final grep of `Genres`, `book_genres`, `FROM genres`, `JOIN genres`, `genre_id` outside `admin` found no other public read path (user profile shelves/lists reuse the shelves/lists mappers; home blocks hydrate via `GetBooksByIDs` + `ToBookListItemResponse`; the review feed and search have no genres).
- Admin UI: `entities/genre/ui/genre-status-badge.tsx`; `/catalog/genres` has Status and QID columns, a `?status=` filter (query key + page reset) and a QID input (client-side hint only, never blocks submit) plus the status select on edit. The edit form always sends the current QID input value (`""` clears it). The Overview tile "Жанры на модерации" links to `/catalog/genres?status=pending`; the sidebar badge and `pending-moderation.ts` are unchanged (fixtures now carry a non-zero `pending_genres` to pin that it is not counted).
- Swagger regenerated (additive only).
- Tests: `bookdata/wikidata_genres_test.go`; `admin/service/genre_status_test.go` (resolver, create/update/list/dashboard); `admin/repository/genre_status_sqlite_test.go`; `admin/transport/http/handler_genre_status_test.go`; `catalog/repository/genre_status_sqlite_test.go` (the shared `catalogStatusDDL` gained `status`/`wikidata_qid`); `home/repository/random_genre_sqlite_test.go` (real SQLite); mapper tests in catalog/shelves/lists; `catalog/domain/genre_test.go`; harvest `genre_status_test.go`. Admin vitest: genres view, genre form, dashboard tile, book-form picker.
- Not verified: live SPARQL against Wikidata and a manual check on the local stack (migration not applied by the agent).
