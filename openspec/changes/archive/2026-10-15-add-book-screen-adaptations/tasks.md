# Tasks

## 1. Backend

- [x] 1.1 Add migration `backend/migrations/000029_screen_works.{up,down}.sql` exactly as in design.md Database. Apply it forward only with the project's normal `migrate up` flow. Never run `migrate down`, drop or reset. Verify that the up migration applies on the local DB and that `\d screen_works` shows the partial unique index.
- [x] 1.2 `bookdata`: add `FetchAdaptations` and `ExternalScreenWork` in `wikidata_adaptations.go`.
  - Verify every whitelist and exclusion class QID from design.md against Wikidata (label and description) and resolve the «телеспектакль» item. Record the verified list in a comment.
  - Write a pure function `classifyAdaptation(classes) (kind, ok)` for the kind priority and the exclusions.
  - Parse the title fallback (Russian, English, P1476), the year range, `tt\d+` and digits-only ids, and the directors (at most 3, sorted, joined with «, »).
  - Unit tests with a fake SPARQL server (`httptest`), following `wikidata_test.go`:
    - film series, video game and list article excluded;
    - an item that is both film and film series excluded;
    - animated series gets `animated_series`;
    - several P577 values give the minimum year, one item;
    - English fallback and P1476 fallback;
    - no title drops the item;
    - malformed IMDb or Kinopoisk ids become empty;
    - an invalid QID gives `ErrInvalidQuery` without a call;
    - more than 50 QIDs is rejected;
    - a 5xx response gives `ErrSourceUnavailable`.
- [x] 1.3 Module `backend/internal/modules/screenworks` (domain, repository, service), as in design.md Backend layout.
  - The sync covers per-book transactions, the upsert by QID, the fill-empty-only update that skips `manually_edited_at`, and `ON CONFLICT DO NOTHING` links that count only inserted rows.
  - Outcomes and notes, the 1 to 50 cap with dedupe, 503 when no source is configured, and 502 when every chunk failed.
  - Service tests with a fake source and a test DB, following the existing module tests. Cover:
    - re-sync is `unchanged` with no duplicates;
    - a work shared by two books;
    - a hidden link stays hidden after re-sync and is not counted;
    - an empty year is filled while the title and kind stay;
    - a `manually_edited_at` work is untouched;
    - a book without a QID is `skipped` with `no_qid`;
    - an unknown book is `error`;
    - a source failure gives `wikidata_unavailable` and the 502 error;
    - 51 ids is the too-large error.
- [x] 1.4 Transport and routes. Add the public `GET /books/:id/adaptations`, next to `/books/:id/collections`. Add the admin routes `POST /admin/books/sync-adaptations` (admin and bearer), `GET /admin/books/:id/adaptations` (admin), and `PATCH /admin/books/:id/adaptations/:screen_work_id` (admin and bearer; `hidden` required through `*bool`). Add `external_id` to the admin book DTO. Write Russian swagger annotations and regenerate `backend/docs`.
  - Handler tests:
    - public 200 shape and order (year ascending, nulls last, then title);
    - a hidden link is excluded;
    - 400 for an invalid id;
    - 404 for a pending book;
    - 200 with an empty list;
    - sync 400, 403 for a moderator, 502 and 503;
    - patch 200, 400 without `hidden`, 404 for an unknown link.
  - Router test: an anonymous public route is not 401 or 403, and `/books/:slug` still resolves.
- [x] 1.5 Harvest:
  - `processHarvestRecord` also returns the created book id;
  - the handler collects the ids and, after author enrichment, calls the injected `AdaptationSyncer`;
  - `adaptations_added` and `adaptation_sync_errors` go on `HarvestResult` (`*int`, omitempty);
  - wire the syncer in `cmd/worker` (and in `cmd/api` for the admin endpoint) with the shared `WikidataClient`.

  Tests, following `handler_enrich_test.go`:
  - the counters are set;
  - a sync failure only counts errors and the job succeeds;
  - skipped books are not synced;
  - a nil syncer leaves the counters nil.
- [x] 1.6 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./...`. Run gofmt only on the changed files, never on the whole tree.

## 2. Frontend

- [x] 2.1 Add the types and methods from design.md Frontend to `packages/api-client`, including `external_id` on `AdminBook`. Run `tsc --noEmit` for the package.
- [x] 2.2 `apps/web`:
  - New entity `entities/screen-work` with the API, the kind labels and the `BookAdaptations` component.
  - `views/book-detail` fetches the adaptations with `.catch(() => null)` and renders the «Экранизации» block only when there are items.
  - Tests:
    - the block lists title, year, type, «Режиссёр: …», and the links «Кинопоиск» and «IMDb» with the correct URLs and a new tab;
    - an item without ids has no links;
    - an empty list or an error renders no block;
    - the kind labels.
  - Verify with `tsc --noEmit` and `next lint` for `apps/web`, and the app's test runner.
- [x] 2.3 `apps/admin`:
  - New feature `features/sync-adaptations`, with the mutation and `sync-text.ts`. Its tests cover every single-result line from the spec, the bulk summary with «Без QID пропущено: N.» and at most three details, and the 502, 503 and fallback error texts.
  - `views/books-list`: a row action only for QID books.
  - `views/book-moderation`: a row action and the bulk «Найти экранизации в Wikidata (N)» counting QID rows only.
  - `views/book-edit`: the «Экранизации» section with list, «Скрыта», «Скрыть»/«Показать», the sync button, the empty text and the error texts.
  - View tests cover:
    - no button without a QID;
    - the bulk sends only QID ids;
    - hiding switches the button to «Показать»;
    - the section's empty and error states.
  - Respect FSD: no cross-feature imports, and no view imports another view. Verify with `tsc --noEmit` and `next lint` for `apps/admin`, and the app's test runner. Never run a native `next build`; CI is the typedRoutes gate.

## 3. Docs

- [x] 3.1 Add the feature to `docs/ROADMAP.md` as done. Note:
  - book adaptations from Wikidata (F1 of 3) and the OpenSpec change `add-book-screen-adaptations`;
  - migration 000029;
  - F2 (films about an author) and F3 (manual editing of adaptations) as the next items.
- [x] 3.2 Add the new domain `screen-works` to the domain list in `openspec/config.yaml` (`context`) and in `docs/PIPELINE.md` ("OpenSpec"). The orchestrator archives the delta and decides on the feature-planner agent prompt. Do not edit `openspec/specs/` directly.
