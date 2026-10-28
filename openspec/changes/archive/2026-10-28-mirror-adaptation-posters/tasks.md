# Tasks

## 1. Backend: schema and download defaults
- [x] 1.1 Add `backend/migrations/000044_screen_work_poster_mirror.up.sql` and `.down.sql` exactly as in design «Database». It is forward-only and must not edit 000043. Verify with `go build ./...` and the migration test harness if one exists. Never run `migrate down`, and never drop or truncate anything.
- [x] 1.2 Add `image.tmdb.org` to `coverfetch.DefaultAllowedHosts`. Update `fetch_test.go`: `image.tmdb.org` is allowed, `x.image.tmdb.org` is refused. Update the config default and the compose/env examples if they list the hosts.
- [x] 1.3 Set `tmdb.ImageBaseURL` to `w500`, and update the `pkg/tmdb` tests that assert the URL.

## 2. Backend: generalise the covers mirror (owners)
- [x] 2.1 Add `covers/domain.Owner` and the owner descriptors in the repository (design D1). Make the repository methods owner-aware: `GetSubject`, `SaveState`, `DueCandidates` (UNION, single LIMIT), `VerifyBatch`, `MarkMissing`, `TouchChecked`, `ResetAllStored` (both tables, one transaction), `PathReferenced` and `ReferencedPaths` (both tables).
- [x] 2.2 Make `MirrorOne(ctx, owner, id, reason)` owner-aware. The `absent` branch applies to books only. Update `Sweep` and `collect`.
- [x] 2.3 Tasks: the `{owner, id, reason}` payload, with the legacy `book_id` decode; `Enqueuer.EnqueuePosterMirror`.
- [x] 2.4 Tests in `covers/service/service_test.go` and the repository tests:
  - a poster is copied, and its URL guard is respected after a removal or an upload while the download runs;
  - orphan GC keeps a file referenced only by `screen_works.poster_copy_path`;
  - the marker reset covers posters;
  - `DueCandidates` interleaves owners by wait time;
  - a legacy `book_id` payload still works;
  - the existing book tests still pass.

  Verify with `go build ./... && go vet ./... && go test ./internal/modules/covers/... ./internal/pkg/coverfetch/... ./internal/pkg/tmdb/...`.

## 3. Backend: screen works integration
- [x] 3.1 Domain: add the copy columns to `PosterColumns`. `Poster()` returns `/covers/<path>` only for a stored copy of the current `poster_url` (design D2). Make sure every query that selects poster columns includes the new ones. Table tests: stored and matching; stored but stale source; pending; failed; manual.
- [x] 3.2 Sync:
  - add `PosterInput.TMDBRef`, filled in `tmdbPosters`;
  - the fill UPDATE and the insert reset `poster_copy_*` and set `poster_tmdb_ref`, `poster_verified_at` and `poster_refresh_at` (150 d + jitter) for TMDB;
  - add `SyncCounts.PosterWorkIDs`;
  - add the service option `WithPosterMirror`, which enqueues after `SyncBook` and logs and swallows errors.

  Tests: TMDB fill stores the ref and a `refresh_at` in [150 d, 164 d); an enqueue happens for each filled work; an enqueue error leaves the outcome and counters unchanged.
- [x] 3.3 `SetManualPoster`/`RemovePoster` also clear the copy and TMDB columns. Test: after the removal, `Poster()` is nil and `poster_copy_path` is NULL.
- [x] 3.4 Wiring: `cmd/api/main.go` (`taskClient`) and `cmd/worker/main.go` (`coversClient`) pass `WithPosterMirror` when `CoversMirrorEnabled`. Update the swagger description of `poster.url`, and regenerate swagger if the repo does that in CI.

## 4. Backend: TMDB re-check and purge
- [x] 4.1 Covers service: `WithTMDB(PosterLookup)`, `WithRand`, and the sweep step `refreshTMDB`: the hard limit at 175 days, the batch of 20, and the outcomes same / changed / gone / unavailable / no key / no ref (design D5). Wire the TMDB client into the worker covers block when `TMDB_API_KEY` is set. Tests with a fake lookup and a fixed clock cover each outcome, plus the hard-limit drop; a drop never sets `removed` or `manually_edited_at`.
- [x] 4.2 `PurgeTMDB(ctx, dryRun)` in the covers service plus the repository methods. Add the thin `cmd/tmdbpurge/main.go` with `-dry-run` and the key warning, and the `backend/Dockerfile` build and copy lines. Tests: commons, manual and removed rows are untouched; files are deleted only when no longer referenced; a dry run changes nothing. Do not run the command against the dev database.
- [x] 4.3 Run `go build ./... && go vet ./... && go test ./...` from `backend/`. gofmt only the changed files.

## 5. Frontend (apps/web)
- [x] 5.1 Add the official TMDB logo SVG at `apps/web/public/brands/tmdb.svg`, downloaded from TMDB's logos page and not redrawn. If it cannot be downloaded, stop and report it.
- [x] 5.2 Footer (design D7): «Постеры фильмов — Wikimedia Commons и TMDB.», the logo link (name «TMDB», about 12 px high) and «BookHaven uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise approved by TMDB.» Update `footer.test.tsx`: the new notice, the old notice absent, and the link named «TMDB» to `https://www.themoviedb.org` containing an image.
- [x] 5.3 Verify with `pnpm --filter web exec tsc --noEmit`, `pnpm --filter web lint` and the web unit tests. No native `next build`: CI is the typedRoutes gate.

## 6. Docs
- [x] 6.1 `docs/DEPLOYMENT.md`:
  - «Self-hosted covers» now covers posters: the owners, the poster columns, the shared queue and batch, the marker reset of posters, `image.tmdb.org` in the default allowlist table, and a state query for posters (`SELECT poster_source, poster_copy_status, count(*) FROM screen_works WHERE poster_source IN ('commons','tmdb') GROUP BY 1,2;`).
  - «Screen work posters»: Commons and TMDB posters are copied into `covers-data`, not hotlinked; TMDB at `w500`; the TMDB re-check (150–164 days, dropped at 175 days, dropped without a key); the purge procedure (remove `TMDB_API_KEY` from api and worker, restart, run `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/tmdbpurge worker -dry-run`, then without `-dry-run`).
  - Backups: `posters-data` (uploads) still needs a backup in prod (parked); `covers-data` does not.
- [x] 6.2 Leave the uncommitted `docs/ROADMAP.md` line for #676 as it is. Do not edit `openspec/specs/`.
