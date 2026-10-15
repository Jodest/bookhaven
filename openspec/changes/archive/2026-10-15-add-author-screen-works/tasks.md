# Tasks

## 1. Backend

- [x] 1.1 Add migration `backend/migrations/000030_author_screen_works.{up,down}.sql` exactly as in design.md Database. Apply it forward only with the project's normal `migrate up` flow. Never run `migrate down`, drop or reset. Verify that the up migration applies on the local DB and that `\d author_screen_works` shows the PK, the relation CHECK and the work index.
- [x] 1.2 `bookdata`: add `FetchAuthorFilms`, `ExternalAuthorFilm` and the pure `classifyAuthorFilm` in `wikidata_author_films.go`, reusing F1 helpers and exclusions without changing `classifyAdaptation`.
  - Verify live (label and description) Q93204 and Q645928, and look up "television documentary" and "documentary television series"; record the verified list in a comment. Check that Q42689885 (Tolkien Q892) and Q1343175 (Rowling Q34660) are returned with the expected kind and relation.
  - Generate the SPARQL `VALUES ?allowed` list from the Go whitelist.
  - Unit tests with a fake SPARQL server (`httptest`), following `wikidata_adaptations_test.go`: query contains the P921 pattern and the allowed-class filter; biographical film → `film`/`biopic`; TV film → `tv_film`/`biopic`; P31 Q93204 → `film`/`documentary`; film with P136 Q93204 → `documentary`; film without genre → `biopic`; TV series episode, film series and list article excluded; several P577 values give the minimum year, one item; English and P1476 title fallback; no title drops the item; invalid QID gives `ErrInvalidQuery` without a call; more than 50 QIDs rejected; 5xx gives `ErrSourceUnavailable`.
- [x] 1.3 `screenworks` module: domain additions, repository (`GetAuthorsForSync`, `AuthorStatus`, `SyncAuthor`, `ListPublicByAuthor`, `ListAdminByAuthor`, `SetAuthorLinkHidden`) and service (`SyncAuthors`, `SyncCreatedAuthors`, `HasAuthorSyncSource`, lists, set hidden) as in design.md Backend layout. F1 behavior and tests stay unchanged.
  - Service tests with a fake source and the test DB, following `service_test.go`: re-sync is `unchanged` with no duplicates; a work already linked to a book is reused (one `screen_works` row, linked to both); a documentary shared by two authors; a hidden link stays hidden after re-sync and is not counted; relation is not changed by a re-sync; an empty year is filled while title and kind stay; a `manually_edited_at` work is untouched; no QID → `skipped`/`no_qid`; unknown author → `error`; source failure → `wikidata_unavailable` and the 502 error; 51 ids → too-large error; hiding an author link does not touch the book link of the same work.
- [x] 1.4 Transport and routes: public `GET /authors/:id/screen-works` next to `/authors/:slug`; admin `POST /admin/authors/sync-screen-works` (admin, bearer, registered before `/:id`), `GET /admin/authors/:id/screen-works` (admin), `PATCH /admin/authors/:id/screen-works/:screen_work_id` (admin, bearer, `hidden` via `*bool`). Russian swagger annotations; regenerate `backend/docs`.
  - Handler tests: public 200 shape with `relation` and order; hidden excluded; 400 for a slug/invalid id; 404 for a pending author; 200 with an empty list; sync 400 (empty, 51, malformed), 403 for a moderator, 502 and 503; patch 200, 400 without `hidden`, 404 unknown link, 403 moderator.
  - Router test: the anonymous public route is not 401/403, and `/authors/:slug` still resolves.
- [x] 1.5 Author import: `WithAuthorFilmSyncer` option and the best-effort call after `enrichImportedAuthor` for `created` only; wire it in `cmd/api` (construct `screenWorksService` before `adminService`). Tests following `author_enrich_import_test.go`: created author triggers one sync with its id; sync failure still returns `created` with an unchanged response; `existing` and `rejected` do not trigger it; nil syncer is a no-op.
- [x] 1.6 Harvest: `AuthorFilmSyncer` interface, `WithAuthorFilmSyncer`, `author_films_added`/`author_film_sync_errors` on `HarvestResult` (`*int`, omitempty), the step after the adaptation sync over the created authors; wire in `cmd/worker`. Tests following the F1 adaptation-sync harvest tests: counters set; failure only counts and the job succeeds; no created authors → no call and counters 0; nil syncer leaves counters nil.
- [x] 1.7 Author merge: move film links in `MergeAuthors` as in design.md Database. Repository/service tests: duplicate's hidden link appears hidden on the canonical author; a film linked to both ends up as one hidden link when either was hidden; `books_reassigned` unchanged.
- [x] 1.8 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./...`. Run gofmt only on the changed files, never on the whole tree.

## 2. Frontend

- [x] 2.1 `packages/api-client`: types and methods from design.md Frontend. Verify with `tsc --noEmit` for the package.
- [x] 2.2 `apps/web`: `screenWorksApi.forAuthor`, `AuthorFilms` component in `entities/screen-work`, and the block in `views/author-detail` (fetch with `.catch(() => null)`, render after «Книги автора»).
  - Tests: the block lists title, year, type, «Документальный» only for documentaries, «Режиссёр: …», the links «Кинопоиск» and «IMDb» with correct URLs and a new tab; an item without ids has no links; an empty list renders nothing; in the view test, a failed films request renders the page without the block.
  - Verify with `tsc --noEmit` and `next lint` for `apps/web`, and the app's test runner.
- [x] 2.3 `apps/admin`:
  - New feature `features/sync-author-films` with the mutation and `sync-text.ts`; tests cover every single-result line from the spec, the bulk summary with «Без QID пропущено: N.» and at most three details, and the 502, 503 and fallback error texts.
  - `views/authors`: row action only for QID authors. `views/author-moderation`: row action and bulk «Найти фильмы об авторе в Wikidata (N)» counting QID rows only.
  - New widget `widgets/author-films-admin`, rendered in `views/author-edit`: list, «Скрыт», «Документальный», «Скрыть»/«Показать», sync button only with a QID, empty and error texts.
  - View tests: no button without a QID; bulk sends only QID ids; hiding switches the button to «Показать»; the section's empty and error states.
  - Respect FSD. Verify with `tsc --noEmit` and `next lint` for `apps/admin`, and the app's test runner. Never run a native `next build`; CI is the typedRoutes gate.

## 3. Docs

- [x] 3.1 `docs/ROADMAP.md`: mark F2 «Фильмы об авторе» done with the OpenSpec change `add-author-screen-works` and migration 000030; keep F3 (manual editing) as next. Do not edit `openspec/specs/` directly; the orchestrator archives the change.
