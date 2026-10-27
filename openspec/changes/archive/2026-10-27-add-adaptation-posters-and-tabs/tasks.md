# Tasks

Verification for backend groups: from `backend/` run `go build ./...`, `go vet ./...`, `go test ./...`. For each touched app run `pnpm --filter <app> exec tsc --noEmit`, `pnpm --filter <app> lint` and its vitest suite. Never run a native `next build`, because CI is the typedRoutes gate. Never run destructive DB commands (`migrate down`, drop), and never bring up a second compose stack. Format only the changed Go files.

## 1. Backend: verdict threshold
- [x] 1.1 Set `screenworks/domain.MinVerdictVotes = 1`. Replace the "keep equal" comments in `screenworks/domain/vote.go` and `curated/domain/auto.go` with "intentionally independent; curated floor stays 5". Update `vote_test.go`: 0 votes give nil counts, 1 vote reveals them. Update the verdicts handler test so it expects `min_votes: 1`. Verify with `go test ./internal/modules/screenworks/...`.

## 2. Backend: poster storage, model and admin endpoints
- [x] 2.1 Add migration `000043_screen_work_posters.up.sql`/`.down.sql` (design § Database). Never edit an older migration. Verify by applying with the normal `migrate up` only.
- [x] 2.2 Config: add `POSTERS_DIR` (default `./storage/posters`) and `TMDB_API_KEY` (default empty) to `internal/config`, with config tests.
- [x] 2.3 Domain:
  - add the poster fields to `LinkedWork` and to the work detail model, plus `PosterInput`;
  - `Poster()` builds the API object (nil for NULL and `removed`, `/posters/<path>` for `manual`), and `PosterRemoved()`;
  - unit tests for every source.
- [x] 2.4 Repository:
  - select the poster columns in `linkedSelect`, the author film lists and the work detail;
  - `SetManualPoster(ctx, id, path) (prevPath *string, err)`, `RemovePoster(ctx, id) (prevPath *string, err)` with the manual→NULL and commons/tmdb→`removed` rule, and `PosterPathReferenced(ctx, path)`;
  - the work delete returns the deleted poster path for garbage collection.
- [x] 2.5 Service:
  - `UploadPoster(ctx, id, data)`: sniff the type (`coverfetch.SniffExt`), name the file `{sha[0:2]}/{sha}.{ext}`, `store.Put`, update the DB, delete the previous file if it is no longer referenced. A DB failure deletes the new file if unreferenced.
  - `RemovePoster` and garbage collection after `DeleteWork`.
  - Neither path touches `manually_edited_at`.
  - Inject the store through a service option. Unit tests use an in-memory store.
- [x] 2.6 Transport:
  - `PUT` and `DELETE /api/v1/admin/screen-works/:id/poster` with the admin role, the bearer header and the error mapping from the admin-editing spec;
  - add the body-limit override in `transport/http/limits.go`;
  - add `poster` to the public and admin items and `poster`/`poster_removed` to the detail;
  - swagger annotations;
  - handler tests for 200, 400, 413, 422, 404 and 403, and that a PDF named `.jpg` gives 422.
- [x] 2.7 File route: generalise the covers handler with `RegisterAt(e, prefix)` and register `/posters` in `cmd/api/main.go` with `coverstore.NewFS(cfg.PostersDir)`. Add handler tests for `/posters` covering 200, 304, 404 and path traversal.

## 3. Backend: posters in the Wikidata adaptation sync
- [x] 3.1 `bookdata`:
  - extend `buildAdaptationsQuery` with P3383, P18, P4947 and P4983 (`SAMPLE`);
  - add `commonsFileName` (Special:FilePath URL → file name);
  - add `PosterFile`, `ImageFile`, `TMDBMovieID` and `TMDBTVID` to `ExternalScreenWork`, validating the TMDB ids;
  - parser tests with fixtures, including an encoded file name and a malformed TMDB id.
- [x] 3.2 New `internal/pkg/tmdb`:
  - client per design D7, with base URL, limiter and timeouts;
  - `httptest` tests: poster found, `poster_path` null, 404, 401, 429 and an invalid `poster_path`;
  - the error strings never contain the key.
- [x] 3.3 Service:
  - the poster resolution step in `syncChunk` (design D6): candidates, then Commons batches with the free-licence and author rules, then TMDB within the 100-lookup cap, TV id first for series;
  - add `WithTMDB`;
  - `Posters` counters on `SyncItem`, `SyncReport` and `CatalogSyncResult`;
  - tests: Commons P3383 wins over P18, a non-free image falls back to TMDB, no TMDB client means no calls, a TMDB or Commons error leaves the outcome unchanged, an existing work is backfilled with `outcome: unchanged` and `posters` > 0.
- [x] 3.4 Repository:
  - `PosterCandidates`;
  - `SyncBook` returns `{Added, Posters}`, inserts new works with a poster and fills existing ones only `WHERE poster_source IS NULL AND manually_edited_at IS NULL`;
  - the author film sync path leaves the poster columns untouched;
  - tests or service fakes covering the manual, removed and edited-by-hand cases.
- [x] 3.5 Wire the TMDB client (only when `TMDB_API_KEY` is set) into the screenworks service in `cmd/api/main.go` and `cmd/worker/main.go`.

## 4. Shared API client
- [x] 4.1 `packages/api-client`:
  - `ScreenWorkPoster` and the `poster` fields;
  - `poster_removed`;
  - the `posters` counters;
  - `uploadPoster` (FormData) and `deletePoster`.

  Verify with tsc in web and admin.

## 5. Web (apps/web)
- [x] 5.1 `entities/screen-work`:
  - `ScreenWorkPoster` with the placeholder, the `onError` fallback and the kind icon map;
  - `PosterAttribution`;
  - `BookAdaptations` and `AuthorFilms` as poster card grids, plus `headingHidden` and `id="adaptations"`;
  - `verdictLabel` returns null below `min_votes`.

  Update the tests: the placeholder shows the icon and year, a broken image falls back, the Commons caption is shown, and «Пока мало голосов» is gone.
- [x] 5.2 `features/vote-adaptation/ui/adaptation-verdict.tsx`:
  - the dropdown (design D2) with «Не выбрано» and «Убрать голос» shown only in their states;
  - disabled while pending;
  - the previous value is restored on failure;
  - no `border-t`;
  - no verdict line at 0 votes.

  Rewrite `adaptation-verdict.test.tsx` for every scenario of «Voting on the book page» and «Verdict on the book page».
- [x] 5.3 `views/book-detail`:
  - `book-tabs.tsx` (ARIA tabs, keyboard, `?tab=` via `history.replaceState`, `#reviews` on mount);
  - `book-tab-link.tsx`;
  - the header link «N экранизаций»;
  - the `RatingSummary` links switched to tab links;
  - the page passes `searchParams.tab`;
  - `alternates.canonical`.

  Tests:
  - tab order and the default with and without adaptations;
  - `?tab=reviews` initial render, with the adaptations present but hidden;
  - an unknown tab;
  - the arrow, Home and End keys;
  - a click updates the URL without navigation;
  - the header link selects and scrolls (mock `scrollIntoView`);
  - the review-count link.
- [x] 5.4 `widgets/similar-books` (`emptyFallback`, `headingHidden`) and `widgets/reviews-section` (`headingHidden`), with tests for «Похожих книг пока нет.».
- [x] 5.5 `widgets/footer`: the TMDB notice with the link. Add a test.
- [x] 5.6 `apps/web/next.config.js`: the `/posters/:path*` rewrite and `(?!api/|covers/|posters/)` in the headers source. Keep the security-headers tests green.

## 6. Admin (apps/admin)
- [x] 6.1 New `features/screen-work-poster` with the «Постер» section, used on the work edit page:
  - the preview, the source label and the Commons attribution;
  - «Постера нет.» and the removed-poster hint;
  - upload with client checks, then the success or failure messages;
  - removal with the confirmation;
  - buttons disabled while pending.

  Tests for every scenario of «Poster section on the work edit page».
- [x] 6.2 Single and bulk adaptation sync banners: append «Добавлено постеров: <p>.» when `posters` > 0. Update the existing banner tests.
- [x] 6.3 `apps/admin/next.config.js`: the `/posters/:path*` rewrite and the headers exclusion.

## 7. DevOps and docs
- [x] 7.1 `.env.example`:
  - an empty `TMDB_API_KEY=` with a comment: optional, posters via TMDB, the attribution notice is in the footer;
  - `POSTERS_DIR` commented as optional.
- [x] 7.2 `docker-compose.yml` and `docker-compose.prod.yml`:
  - `TMDB_API_KEY: ${TMDB_API_KEY:-}` for the api and the worker;
  - `POSTERS_DIR: /data/posters` and volume `posters-data:/data/posters` (read-write) for the api only;
  - declare the `posters-data` volume.

  No prod secret is set. Do not start a second stack.
- [x] 7.3 `docs/DEPLOYMENT.md`: the posters volume (api read-write) and the fact that it is not covered by the Postgres backup (parked prod item), plus `TMDB_API_KEY` (optional, sync skips TMDB without it).
- [x] 7.4 Regenerate swagger the usual way (`swag init` per the Makefile) and commit the generated docs.
