# Author Status (pending / published / rejected) + Wikidata QID

Status: done

Epic: "Author moderation & import" — feature 1 of 4. Feature 4 ([author-wikidata-enrichment.md](author-wikidata-enrichment.md)) shipped: harvest-created QID authors are enriched with dates and a Commons photo at the end of the job.

## Goal

Stop unvetted authors from leaking to the public site. Harvest currently auto-creates authors that are immediately public (empty / junk author pages). Introduce an author `status` lifecycle and an optional Wikidata identity (QID) so harvested authors land in `pending`, rejected authors act as tombstones, and every public read shows only `published` authors — while books stay publishable independently of their authors' status.

## Out of scope (other features of the epic)

- **Feature 2** — admin author moderation queue: publish / reject / merge-with-existing actions UI, similar-name hints, dashboard counter, link from the harvest report. (Feature 1 only adds the `status` field, a list badge/filter and a status select in the existing edit form.) Shipped: see [author-moderation-queue.md](author-moderation-queue.md).
- **Feature 3** — direct author import from admin (Wikidata `SearchAuthors` → "import author" / "import author + books"). Shipped: see [author-direct-import.md](author-direct-import.md).
- **Feature 4** — author enrichment from Wikidata on creation (dates, Commons photo with attribution, biography).
- Anything that changes book publication (`books.is_hidden`) based on author status.

## User Stories

- As a reader, I never see an author page, author list entry, calendar entry or link for an author an admin has not published.
- As a reader, on a published book whose author is not yet published, I still see the author's name (plain text, no link).
- As an admin, harvested authors do not go live until I publish them; I can see each author's status in the admin authors list and change it in the edit form.
- As an admin, an author I rejected is never recreated or re-linked by future harvests (tombstone by QID).

## Business Rules

1. `authors.status` ∈ `pending` | `published` | `rejected`. NOT NULL. Existing rows migrate to `published`.
2. Creation paths and resulting status:
   | Path | Status |
   |---|---|
   | `POST /admin/authors` (manual) | `published` (request cannot set status) |
   | Admin JSON/CSV import (`admin/service/import.go` → resolver) | `published` — admin-curated input, same trust level as manual creation |
   | Harvest worker (`harvest/service/handler.go` `processRecord`) | `pending` |
   | Seed / raw SQL inserts without a status | `published` (DB default) |
   The admin book form only takes `author_ids` (existing authors), so it creates no authors.
3. `authors.external_source` / `authors.external_id` are optional (manual/seed/legacy/contemporary authors have none). `(external_source, external_id)` is unique when `external_source IS NOT NULL`. Only source value used now: `wikidata`; `external_id` is a bare QID matching `^Q[0-9]+$`.
4. **Harvest author resolution** (per author ref `{name, qid?}`, in order; replaces name-only resolution for harvest only):
   1. If `qid` present: look up by `(wikidata, qid)`.
      - found & `rejected` → **tombstone: skip** (no `book_authors` row, no new author).
      - found & `pending`/`published` → link it.
   2. Else / not found: look up by `slugify(name)`.
      - found & `rejected` → **skip** (tombstone applies on any match path); if it has no QID and `qid` present, backfill the QID so future matches are by QID.
      - found, no QID, `qid` present → **backfill** `external_source='wikidata', external_id=qid` and link. Status is NOT changed by backfill.
      - found, same QID or no `qid` in input → link.
      - found with a **different** QID (namesake) → do not touch it; create a new `pending` author with slug `slugify(name) + "-" + lowercase(qid)` (e.g. `ivan-petrov-q12345`) and that QID; link the new one.
   3. Not found → create `{name, slug: slugify(name), status: pending, external_source/external_id: qid or NULL}` and link.
   - Duplicates by resolved ID are collapsed (existing behavior).
   - If every author of a record is skipped, the book is still created (it is hidden anyway, in the harvest review queue) with no authors; log at info level with the QID(s). No new harvest-report counters in this feature.
   - **Recommendation rationale for skip (vs. linking to the rejected record):** a rejected record is a "not a valid catalog author" decision (junk label, wrong entity); linking would re-attach new books to it and surface its name on public book pages. Skipping keeps the tombstone inert and the admin fixes authors during book review.
5. Admin manual import keeps name-only matching (no QID), but creates missing authors as `published`; a slug-matched `rejected` author is linked as-is (admin's explicit input; it renders name-only publicly). Harvest's slug-match-rejected skip (rule 4.2) is harvest-only.
6. **Public visibility:** every public read path returns only `published` authors as first-class entities (pages, lists, search, calendar). See "Public read paths" below.
7. **Book publication is independent of authors (variant c).** A published book is shown even if all its authors are non-published. In any public book payload, a non-published author is embedded **by name only**: `slug: null`, and `photo_url`/`birth_date` omitted. `id` stays (stable list key; it reveals nothing navigable).
8. Similar-books ranking counts **shared published authors only** (pending/rejected author links are ignored as a signal; genre signal unaffected).
9. Admin reads see all statuses everywhere; admin payloads always include `status`.
10. Admin may change status via `PATCH /admin/authors/:id` (any → any of the three values). This is the stop-gap so pending authors are not stuck until feature 2.
11. Admin merge (`POST /admin/authors/:id/merge`): if the canonical author has no QID and the duplicate has one, the QID moves to the canonical (in the same transaction, after the duplicate row is deleted, to satisfy the unique index). If both have QIDs, the canonical keeps its own (duplicate's QID is dropped; log it). Canonical status is unchanged. Without this, a merged-away QID would make the next harvest recreate the author as `pending`.

## Acceptance Criteria

DB
- [x] Migration `000015_authors_status_external_id` adds `status` (default `published`, CHECK constraint), `external_source`, `external_id`, partial unique index; all existing authors are `published` after `migrate up`. Down migration drops them (provided for completeness; never instructed to be run).

Harvest / Wikidata
- [x] Wikidata queries (`buildTitleAuthorQuery`, `buildISBNQuery`, `BuildAuthorListQuery`) return author QID paired with label; parser yields `[]ExternalAuthor{Name, QID}` with pairs never misaligned.
- [x] `ExternalBookRecord.Authors []string` still populated (names) for all sources; Open Library records have `AuthorRefs` empty/QID-less and harvest still works with names only.
- [x] New harvested author → `status=pending`, QID stored.
- [x] QID match wins over slug match (author renamed in DB still matched by QID).
- [x] QID-matched `rejected` author → no new author created, no `book_authors` row for it; book still created hidden.
- [x] Slug-matched author without QID gets QID backfilled; status unchanged.
- [x] Slug-matched author with a different QID → new pending author with `-q<id>` slug suffix.
- [x] Re-running the same harvest creates no duplicate authors.

Admin
- [x] `POST /admin/authors` creates `published`; a `status` in the body is ignored.
- [x] Admin import creates missing authors as `published`.
- [x] `GET /admin/authors` items include `status`, `external_source`, `external_id`; supports optional `?status=pending|published|rejected` (invalid value → 400).
- [x] `PATCH /admin/authors/:id` accepts optional `status`; invalid → 400; omitted → unchanged.
- [x] Admin book responses' embedded authors include `status`.
- [x] Merge carries the QID to the canonical per rule 11.
- [x] Admin UI: status badge in the authors table, status filter, status select in the edit form; author pickers in the book form show a badge for non-published authors.

Public
- [x] `GET /authors/:slug` → 404 for `pending`/`rejected`.
- [x] `GET /authors` (list + search) returns only `published`; `total` counts only `published`.
- [x] `GET /books?author_id=<non-published id>` → empty result (200, total 0).
- [x] Every public book payload (book detail, book lists, author page co-authors, similar books, home blocks incl. book of the day / calendar books / new / top-rated / recommendations / want-to-read pick / random, review feed books, public & own shelves, public & own lists) renders non-published authors as `{id, name, slug: null}`.
- [x] Literary calendar never selects non-published authors.
- [x] Similar books ignore non-published authors as a shared-author signal (both live query and precomputed `book_similarities` job).
- [x] Web: non-published author names render as plain text (no `<Link>`) on book detail, book-of-the-day, and every book card that shows authors; no link to `/authors/null`.

## API

Public (`catalog/transport/http/handler.go`) — `AuthorResponse.Slug` becomes `*string` (`json:"slug"`, emits `null`, NOT omitempty so clients get an explicit null). Introduce one shared mapper rule: `if !a.IsPublished() { return {ID, Name, Slug: nil} }`. Apply it in every embed mapper:
- `catalog` `toAuthorResponse` (used by `ToBookListItemResponse`, book detail, author list).
- `shelves/transport/http/handler.go:104` `toAuthorResponse`.
- `lists/transport/http/handler.go:132` `toAuthorResponse`.
- `home/transport/http/handler.go:192` `reviewBookAuthorResponse` (Slug → `*string`).
- Home calendar `calendarAuthorResponse` needs no nullable slug (calendar is filtered to published), but keep its type unchanged.

The `GET /authors` list and `GET /authors/:slug` detail only ever contain published authors, so their top-level `slug` is always non-null in practice; the type is shared, so it is nullable in TS.

No new public endpoints. No `status` field in public responses (null slug is the signal; keeps moderation state private).

Admin (`admin/transport/http/handler.go`):
- `authorResponse` += `status: "pending"|"published"|"rejected"`, `external_source: string|null`, `external_id: string|null` (embedded in book responses too).
- `GET /api/v1/admin/authors?status=` optional filter.
- `PATCH /api/v1/admin/authors/:id` body += optional `status`. `AuthorInput` += `Status *string` (nil = unchanged on update; ignored on create). New `ErrInvalidAuthorStatus` → 400.
- Authorization unchanged: `adminRole` group + `requireBearer` on mutations (moderators get 403). Do not widen to moderators in this feature.

## Database

`backend/migrations/000015_authors_status_external_id.up.sql` (modeled on 000009):
```sql
ALTER TABLE authors ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'published';
ALTER TABLE authors ADD CONSTRAINT chk_authors_status CHECK (status IN ('pending','published','rejected'));
ALTER TABLE authors ADD COLUMN external_source VARCHAR(50);
ALTER TABLE authors ADD COLUMN external_id VARCHAR(255);
CREATE UNIQUE INDEX idx_authors_external_source_id ON authors(external_source, external_id) WHERE external_source IS NOT NULL;
CREATE INDEX idx_authors_status_not_published ON authors(status) WHERE status <> 'published';
```
`.down.sql` drops the indexes, constraint and columns (reverse order). Do not run `migrate down` — apply with `migrate up` only.

Domain `catalog/domain/author.go`: add `Status string` (`gorm:"type:varchar(20);not null;default:published"`), `ExternalSource *string`, `ExternalID *string`, constants `AuthorStatusPending/Published/Rejected`, and `func (a Author) IsPublished() bool`. Service code must set `Status` explicitly on every create (published for admin paths, pending for harvest) — do not rely on the DB default in Go code.

## Backend changes

`pkg/bookdata`
- `bookdata.go`: add `type ExternalAuthor struct { Name string; QID string }` and `ExternalBookRecord.AuthorRefs []ExternalAuthor`. Keep `Authors []string` (names) — used by admin lookup, import, Open Library, enrichment author hint. Wikidata parser fills both from the same parsed pairs (`Authors` = names of `AuthorRefs`), so they cannot diverge.
- `wikidata.go`: in all three query builders replace the `?authors` aggregate with a paired encoding:
  `(GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?author), "/entity/"), "\t", ?authorLabel); separator="|") AS ?authors)`.
  Parser: split on `|`, then each part on the first `\t` → `{QID, Name}`; validate QID with `wikidataQIDPattern` (invalid/missing QID → keep name, empty QID); skip empty names. Leave genres unchanged. Note: in `buildTitleAuthorQuery` the optional author filter limits authors to matching labels — existing behavior, unchanged.

`admin/service`
- `harvest_resolve.go`: keep `ResolveOrCreateAuthorIDsByNames(ctx, names)` for import, but make it create authors as `published`. Add `ResolveOrCreateHarvestAuthors(ctx, refs []AuthorRef) ([]string, error)` (`AuthorRef{Name, QID string}`, defined in the admin service package) implementing rule 4. Creation goes through a shared internal create helper that takes status + external ids (reuses `CreateAuthor` validation/slug logic).
- `admin/repository`: add `GetAuthorByExternalID(ctx, source, id)`, `SetAuthorExternalID(ctx, id, source, extID)`; `ListAuthors` gains a status filter; `MergeAuthors` tx carries QID (rule 11). `CreateAuthor`/`UpdateAuthor` persist `Status`.
- `service.go`: `AuthorInput.Status`, validation, create forces `published`.

`harvest/service/handler.go` `processRecord`: call `ResolveOrCreateHarvestAuthors` with refs mapped from `enriched.AuthorRefs` (fallback: `enriched.Authors` as QID-less refs when `AuthorRefs` is empty). Enrichment never changes authors (current behavior) — keep it so.

Public read paths (every one must be covered):
| # | Path | File | Change |
|---|---|---|---|
| 1 | `GET /authors/:slug` | `catalog/repository/repository.go:162` `GetAuthorBySlug` | `AND status='published'` → 404 otherwise |
| 2 | `GET /authors` list/search | `catalog/repository/repository.go:185` `ListAuthors` | `WHERE status='published'` (count too) |
| 3 | `GET /books?author_id=` | `repository.go:95` | join `authors` and require `authors.status='published'` |
| 4 | Book embeds (detail, list, author page co-authors `Books.Authors`, similar, home blocks via `ToBookListItemResponse`) | catalog handler mappers | name-only mapping (rule 7) |
| 5 | Shelves (own + `GET /shelves/public/:id`), user profile shelves | `shelves/transport/http/handler.go:104` | name-only mapping |
| 6 | Lists (own + `GET /lists/public/:id`), user profile lists | `lists/transport/http/handler.go:132` | name-only mapping |
| 7 | Home review feed book authors | `home/transport/http/handler.go:192` | name-only mapping |
| 8 | Literary calendar | `home/repository/repository.go` ~201-220 (both UNION branches) | `AND a.status='published'` |
| 9 | Similar books (live) | `catalog/repository/repository.go` `GetSimilarSource` (~243) and per-candidate author IDs (~341) | join `authors`, `status='published'` |
| 10 | Similar books (precomputed job) | `catalog/repository/similarity.go:128` `ListBookAuthorIDs` | join `authors`, `status='published'` |
`home` `ListTopBooksByAuthors` is only fed calendar (published) author IDs — no change. Admin repository reads stay unfiltered. Developer must re-grep `Preload("Authors")`, `book_authors`, `FROM authors` before finishing to confirm no new path was added.

## Frontend

`packages/api-client/src/index.ts`
- `Author.slug: string | null` (public). `HomeCalendarAuthor` unchanged. Any home review-feed author type: `slug: string | null`.
- `AdminAuthor` += `status: AdminAuthorStatus` (`'pending' | 'published' | 'rejected'`), `external_source: string | null`, `external_id: string | null`. `AdminAuthorInput` += `status?: AdminAuthorStatus`. Admin authors list params += `status?`.
- The type change makes `tsc` flag every link site; fix all of them.

`apps/web` — render name as plain text when `slug === null`, `<Link>` otherwise. Extract one small shared component (e.g. `entities/author/ui/author-name-link.tsx` or the closest existing author entity slice) and use it in:
- `views/book-detail/ui/book-detail-view.tsx:59`
- `widgets/book-of-the-day/ui/book-of-the-day.tsx:66`
- any book card / list / shelf / review-feed component that renders author names (grep `authors` usages in `apps/web/src`). `widgets/literary-calendar` stays as is (always published).

`apps/admin`
- `views/authors/ui/authors-view.tsx`: status column with badge (pending = warning, rejected = destructive, published = default), status filter select bound to `?status=` URL param.
- `features/manage-author/ui/author-form.tsx`: status select on edit only (hidden on create).
- Book form author picker: show a status badge next to non-published authors (data already in `AdminAuthor`).

## Existing Patterns

- Migration 000009 (`books.external_source/external_id` partial unique index) — mirror exactly.
- `books.is_hidden` public filtering in `catalog/repository/repository.go` (`ListBooks`, `GetBookBySlug`) — same style for `status='published'`.
- `admin/service/harvest_resolve.go` resolver + `FindBookByExternalID` idempotency in `harvest/service/handler.go`.
- `docs/features/duplicate-author-merge.md` + `admin/repository` `MergeAuthors` tx for rule 11.
- `docs/features/harvest-review-queue.md` (hidden-by-default harvested content).

## Tests

Backend (run via `docker compose run`; never `migrate down`):
- `bookdata`: parser for paired encoding — multiple authors, label containing spaces/punctuation, missing/invalid QID, empty aggregate; query builders contain the paired `CONCAT` aggregate for all three builders; Open Library record unaffected.
- `admin/service` resolver unit tests (fakes): new → pending + QID; QID match beats slug; QID-matched rejected → skipped, nothing created; slug-matched rejected → skipped + QID backfilled; slug match without QID → backfill; slug match with different QID → new `-q…` author; no QID → slug path; dedupe.
- `admin/service`: create forces published; import creates published; update status valid/invalid/omitted; list status filter; merge QID carry-over (canonical without QID / both with QID).
- `harvest/service` `processRecord`: uses `AuthorRefs`, falls back to `Authors`; all-authors-rejected still creates hidden book.
- `catalog/repository` (integration, existing `repository_test.go` style): author page 404 for pending/rejected; list/search excludes them; `author_id` filter; similar candidates ignore non-published shared authors; `ListBookAuthorIDs` filtered.
- `home/repository`: calendar excludes non-published authors.
- Transport mappers (catalog, shelves, lists, home review feed): non-published author → `slug: null`, no photo/birth date.
- Admin handler: `status` in responses, `?status=` validation, PATCH status validation.

Frontend:
- web: author-name component renders link vs plain text; book-detail and book-of-the-day tests with a `slug: null` author.
- admin: authors view shows badge + filter; author form sends `status` on edit only.
- CI frontend build is the gate for typedRoutes (do not run native `next build`).

## Documentation

- This contract → `Status: done` on completion.
- `docs/features/harvest-review-queue.md`: note harvested authors are now `pending` and QID-matched.
- `docs/features/duplicate-author-merge.md`: note QID carry-over.
- API docs / README section for public author endpoints if one exists (null `slug` in book embeds).
- `docs/ROADMAP.md`: add/tick the epic item.

## Risks

- **Missed public path** → leak. Mitigated by the enumerated table + final grep; nullable TS type forces compile errors on web link sites.
- **Breaking change to public `slug` type** (string → nullable): any consumer that builds `/authors/${slug}` without the guard yields `/authors/null`. All in-repo consumers are fixed here; there are no external API consumers yet.
- **Existing pending-looking junk**: all legacy authors (including earlier harvest-created junk) migrate to `published`; cleaning them up is admin work (feature 2 queue). Acceptable per product decision.
- **Home cache staleness**: the cached daily home selection (calendar events/books) may show an author until the cache refreshes after an admin changes status to non-published. Acceptable (bounded by the cache TTL); book embeds are re-hydrated per request so they reflect status immediately.
- **Precomputed similarities** reflect status changes only after the next daily recompute.
- **Namesake slug suffix** (`-q123`) produces less pretty slugs; admin can edit the slug.
- **Concurrent harvest jobs** may race on create (unique slug / QID index violation) → that record fails and is reported; the next run resolves it. Same as today's slug behavior; no locking added.
- **SPARQL aggregate change**: if `STRAFTER` or the tab separator misbehaves on the endpoint, parsing falls back to name-only refs (QID empty) — harvest degrades to slug matching rather than failing. Verify against live Wikidata once manually.
- **Skipping rejected authors** can produce hidden harvested books with no authors; admin fixes them during review.
- **Admin import linking rejected authors by slug** (rule 5) renders name-only publicly; intentional (admin's explicit input).

## Implementation Notes

- Migration `000015_authors_status_external_id` applied locally with `migrate up` (all existing authors -> `published`).
- `domain.Author` gained `Status`, `ExternalSource`, `ExternalID`, `IsPublished()` and `IsValidAuthorStatus()`; the GORM default tag is quoted (`default:'published'`) like `Book.Language`.
- Wikidata aggregate uses the SPARQL escape `"\t"` as the pair separator; the parser splits each `|` entry on its first tab. An entry without a tab is accepted as a QID-less name (so older fixtures/plain aggregates still parse).
- Harvest resolution lives in `admin/service/harvest_resolve.go` (`ResolveOrCreateHarvestAuthors`, `AuthorRef`); all creation goes through the internal `createAuthor` helper (status + external identity). A namesake whose `-q<id>` slug is already taken by a QID-less author fails that record with a duplicate-slug error (rare; resolved by admin), same as other slug races.
- Admin service gained an optional `WithLogger(*zap.Logger)` option (no-op by default), wired in `cmd/api` and `cmd/worker`, for the tombstone-skip and merge QID-drop info logs.
- `GET /admin/authors?status=` validation happens in the handler (400) and again in the service (`ErrInvalidAuthorStatus`).
- Merge: the repository carries the QID inside the merge transaction (reads the duplicate identity first, `UPDATE ... WHERE external_source IS NULL` after the delete); the service mirrors it on the returned canonical author and logs a dropped duplicate QID.
- Public `ListBooks?author_id=` keeps the existing `book_authors` join and adds an `EXISTS` on `authors.status = 'published'` (a plain join on `authors` would make unqualified `created_at` ambiguous).
- Web: `entities/author/ui/author-name-link.tsx` (`AuthorNameLink`) is used on book detail and book-of-the-day; every other web book card/list/shelf/review-feed renders author names as plain text already (no links), and the literary calendar is published-only.
- Admin: `entities/author/ui/author-status-badge.tsx` (`AuthorStatusBadge`, `AUTHOR_STATUS_LABELS`; pending uses an amber outline badge since `@bookhaven/ui` has no warning variant). The book-form picker is a native multi-select, so non-published authors are marked in the option text ("Имя · На модерации") and the selected non-published ones are listed with a badge under the picker.
- Tests: real-SQL SQLite tests for the catalog public filters (`catalog/repository/author_status_sqlite_test.go`) and the merge QID carry-over (`admin/repository/merge_qid_sqlite_test.go`); the Postgres-only calendar query is checked by SQL text (`home/repository/calendar_status_test.go`). Live check against the local stack: a temporarily pending author returned 404 on its page, an empty `?author_id=` list and `slug: null` in a book payload (status restored afterwards).
- Swagger regenerated (`swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`).
