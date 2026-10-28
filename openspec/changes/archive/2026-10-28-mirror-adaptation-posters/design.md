# Design

## Context
- **Book cover mirror (#489).** `backend/internal/modules/covers`:
  - `service.Service` has `MirrorOne`, `Sweep`, `collect` and `collectOrphans`. `tasks.go` defines `covers:mirror`/`covers:sweep`, the `covers` queue with Concurrency 1, and the `Enqueuer`.
  - `repository.GormRepository` is hard-wired to `books.cover_*`, including the guarded `SaveState`, `DueCandidates`, `VerifyBatch`, `MarkMissing`, `ResetAllStored`, `PathReferenced` and `ReferencedPaths`.
  - The store is `pkg/coverstore` (FS, marker `.bookhaven-covers`). The download is `pkg/coverfetch` (`DefaultAllowedHosts`, SSRF rules, sniffing).
  - The worker builds the service at `cmd/worker/main.go:539`. The api serves `/covers` read-only.
- **Posters (archived change `2026-10-27-add-adaptation-posters-and-tabs`, D4–D7).**
  - Migration 000043 added `screen_works.poster_*` with `chk_screen_works_poster`. `domain.PosterColumns.Poster()` builds the API object.
  - In the repository, `SyncBook` fills posters with a guarded UPDATE in `repository.go:~267`. `SetManualPoster` and `RemovePoster` are in `works.go`.
  - `service/poster_sync.go` resolves Commons, then TMDB. `pkg/tmdb` returns `ImageBaseURL` (`w342`) + `poster_path`.
  - Uploads live in `posters-data` (api read-write) and are untouched by this change.
- There is no prod deployment yet, so TMDB posters exist only in local databases, and none of them has a kept TMDB id.

## Decisions

### D1. Owners in the covers module (generalise, don't fork)
- Add `covers/domain.Owner` (string kind) with `OwnerBook = "book"` and `OwnerScreenWorkPoster = "screen_work_poster"`. The repository maps each owner to a descriptor that lists, explicitly, the table, the source-link column and the eight mirror columns, plus an optional eligibility SQL. Author photos (#676) will be one more descriptor:
  - book: `books`, `cover_url`, `cover_source_url`/`cover_path`/`cover_status`/`cover_error`/`cover_attempts`/`cover_next_attempt_at`/`cover_stored_at`/`cover_checked_at`;
  - screen work poster: `screen_works`, `poster_url`, `poster_copy_*` (see Database), eligibility `poster_source IN ('commons','tmdb')`.
- The repository API becomes owner-aware:
  - `GetSubject(ctx, owner, id) (*Subject{Link, CopySourceURL, CopyPath, CopyStatus, Attempts}, error)` replaces `GetBook`. The book "has stored cover" check moves to `Subject.HasStoredCopy()`, with the same semantics as `catalogDomain.Book.HasStoredCover`.
  - `SaveState(ctx, owner, id, st)`. The guard is `WHERE id = ? AND <link col> = ? [AND eligibility]`.
  - `DueCandidates(ctx, now, limit) ([]Ref{Owner, ID})` is a `UNION ALL` over owners, ordered by `next_attempt_at ASC NULLS FIRST, created_at ASC`, with one `LIMIT`.
  - `VerifyBatch` returns `[]StoredRef{Owner, ID, Path}` as a `UNION ALL`. `MarkMissing(owner, …)` and `TouchChecked(owner, ids)` take the owner.
  - `ResetAllStored` updates both tables in one transaction and returns the sum.
  - `PathReferenced(path)` and `ReferencedPaths()` look at `books.cover_path` **and** `screen_works.poster_copy_path`. This is the orphan-GC fix: without it, the sweep would delete every poster copy.
- `MirrorOne(ctx, owner, id, reason)`. The book `absent` branch (Open Library `default=false`, 404) stays book-only. A poster 404 is a permanent 4xx and becomes `failed`.
- Task payload `{owner, id, reason}`. A payload with only `book_id` (tasks already queued at deploy) decodes as the `book` owner. Uniqueness is per payload, so it covers the owner and the id. `Enqueuer.EnqueueMirror(ctx, bookID, reason)` keeps its signature for the admin service. Add `EnqueuePosterMirror(ctx, workID)`, which uses reason `save`.
- Log fields: replace `book_id` with `owner` + `id`.

### D2. Displayed poster URL
- `PosterColumns` gains `PosterCopyPath`, `PosterCopyStatus` and `PosterCopySourceURL`, read-only (`->`). For `commons`/`tmdb`, `Poster()` returns `"/covers/" + path` when the status is `stored`, the path is set and `copy_source_url == poster_url`. Otherwise it returns `poster_url`. `manual` is unchanged (`/posters/`). Every read model that embeds `PosterColumns` (public adaptations, author films, admin lists, admin detail) gets this automatically. Their SELECTs must include the new columns: check every query that lists columns explicitly.
- The `/covers/` prefix is shared with books. Reuse the catalog constant if one is exported; otherwise add `CoversURLPrefix` next to `PostersURLPrefix`.

### D3. Writers of poster state
- **Sync fill** (`SyncBook` UPDATE and the insert of a new work with a poster):
  - set the `poster_copy_*` columns to their reset state: everything NULL and attempts 0;
  - for `tmdb`, set `poster_tmdb_ref` (`movie/<id>` or `tv/<id>`), `poster_verified_at = now` and `poster_refresh_at = now + 150d + rand[0,14d)`. For `commons` these three stay NULL.

  `PosterInput` gains `TMDBRef string`, set in `tmdbPosters` from `t.kind + "/" + t.id`. `SyncCounts` gains `PosterWorkIDs []uuid.UUID`, the works that got a poster in this call. For an existing work use `UPDATE … RETURNING id`, or select the id first, since the WHERE clause is by id.
- **Queue after commit.** In `screenworks/service`, after `SyncBook` returns, call `posterMirror.EnqueuePosterMirror(ctx, id)` for each id in `PosterWorkIDs`. Errors are logged at Warn and swallowed. Add the option `WithPosterMirror(PosterMirrorEnqueuer)`; the interface is defined in screenworks so it does not import covers. Wire it in `cmd/api/main.go` (with `taskClient`) and in `cmd/worker/main.go` (with `coversClient`), only when `cfg.CoversMirrorEnabled`.
- **Admin `SetManualPoster` / `RemovePoster`:** extend their UPDATEs to clear `poster_copy_*`, `poster_tmdb_ref`, `poster_verified_at` and `poster_refresh_at`. They do not delete the copy file: the api mounts `covers-data` read-only, and the sweep's orphan GC (files older than 1 h with no reference) removes it.
- **A mirror attempt in flight** while an admin removes or replaces the poster: the `SaveState` guard on `poster_url` plus the eligibility clause fails, so the new file is left for GC. This matches books.

### D4. Allowlist and size
- Set `coverfetch.DefaultAllowedHosts` to `covers.openlibrary.org,archive.org,*.archive.org,upload.wikimedia.org,image.tmdb.org`, and update the config default and the compose/env examples if they spell out the list.
- Set `tmdb.ImageBaseURL` to `https://image.tmdb.org/t/p/w500`. TMDB's path is size-independent, so the migration rewrites the existing `w342` links in place. Only local databases have them, and the copies are not made yet, so nothing has to be invalidated. `thumb.wikimedia.org` (allowed as a Commons source link by the spec) is **not** added. Commons returns `upload.wikimedia.org` thumbnails in practice; a thumb-host link would end `failed` and keep showing its source link.

### D5. TMDB re-check (inside the covers sweep)
- New step `refreshTMDB` in `Sweep`, after the mirror candidates and before the orphan GC. It handles at most `TMDBRefreshBatch = 20` rows per run, ordered by `poster_refresh_at`. All rows are `poster_source = 'tmdb'`.
  1. **Hard limit first.** One UPDATE drops every TMDB poster whose `COALESCE(poster_verified_at, poster_updated_at) < now - 175d`, guarded on `poster_source = 'tmdb'`. The drop sets `poster_source`, `poster_url`, the attribution columns, `poster_copy_*`, `poster_tmdb_ref`, `poster_verified_at` and `poster_refresh_at` to NULL, and `poster_updated_at` and `updated_at` to now. The freed files go to the GC.
  2. **Due rows.** These have `poster_refresh_at <= now`; a NULL `poster_refresh_at` counts as due, which covers rows created before this change.
     - With no TMDB client configured, or `poster_tmdb_ref` NULL: drop the poster (same UPDATE, guarded on `id` and `poster_url`).
     - Otherwise call `tmdb.Poster(kind, id)`:
       - The same URL: set `poster_verified_at = now` and `poster_refresh_at = now + 150d + jitter`.
       - Another URL: in one guarded UPDATE, set `poster_url = new`, reset `poster_copy_*`, set `poster_verified_at = now` and a new `refresh_at`. Then run `MirrorOne(OwnerScreenWorkPoster, id, ReasonSave)` inline; it goes through the same pacing. The old file goes to the GC.
       - `""` (no poster or 404): drop.
       - `ErrUnavailable` or `ErrUnauthorized`: set `poster_refresh_at = now + 24h` and keep the poster. Log `ErrUnauthorized` at Error once per sweep. The hard limit bounds the total age.
  - The copy is not re-downloaded when the URL is unchanged. That is the approved behaviour. The 175-day limit applies to the confirmation of the TMDB data. Re-downloading identical bytes would produce the same content-addressed file anyway.
- Covers `Service` gains an option `WithTMDB(PosterLookup)` (`Poster(ctx, kind, id) (string, error)`) and an injectable random source for the jitter in tests (`WithRand`). Wire it in the worker covers block when `cfg.TMDBAPIKey != ""`. The client is constructed separately from the screenworks one; its limiter is per client, and both clients stay under 10 rps.
- Jitter is drawn once when `refresh_at` is set. It is not deterministic, which is acceptable.

### D6. Purge command (`cmd/tmdbpurge`), not a flag
- A cmd follows the `cmd/similarity` pattern: config, `database.Open` and a 10-minute context. It runs in the **worker** container, the only read-write mount of `covers-data`, with `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/tmdbpurge worker`. For dev, document `go run ./cmd/tmdbpurge` from inside the existing dev worker/backend container, with `COVERS_DIR` set to the worker's storage. Never use a second compose project.
- Flags: `-dry-run`.
- Steps:
  1. In one transaction, select the `poster_copy_path` of `poster_source = 'tmdb'` rows, then run the drop UPDATE from D5 for all of them.
  2. After the commit, for each distinct path, call `PathReferenced`, which checks both owners, and `store.Delete` when it is not referenced.
  3. Print `posters_removed=N files_deleted=M`.
- With `-dry-run`, only the counts are printed. If `TMDB_API_KEY` is set, print a warning that the sync can refill TMDB posters, and that the key should be removed from api and worker first.
- Put the logic in the covers service (`PurgeTMDB(ctx, dryRun) (PurgeStats, error)`) with repository methods, so it is unit-testable. The cmd is thin. Add the build and copy lines to `backend/Dockerfile`.
- Why not an HTTP endpoint or a flag on the worker: a licence end is a one-off operator action, it needs the read-write volume, and it must not be one click away in the admin.

### D7. Footer logo and notice
- Asset: `apps/web/public/brands/tmdb.svg`, the **official** logo from `https://www.themoviedb.org/about/logos-attribution` (the "alt short" blue variant reads well on the dark footer). It must not be redrawn. If the developer environment cannot download it, stop and report it, so the orchestrator can add the file.
- Footer markup (`widgets/footer/ui/footer.tsx`, bottom bar):
  ```
  <p>Постеры фильмов — Wikimedia Commons и TMDB.</p>
  <p>
    <a href="https://www.themoviedb.org" target="_blank" rel="noopener noreferrer">
      <Image src="/brands/tmdb.svg" alt="TMDB" width={…} height={12} unoptimized className="inline-block h-3 w-auto" />
    </a>
    BookHaven uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise approved by TMDB.
  </p>
  ```
  The logo is about 12 px high, smaller than the header logo. The width follows the SVG's aspect ratio. Use `next/image` with `unoptimized` (an SVG through the optimizer needs `dangerouslyAllowSVG`), or a plain `<img>` with the lint rule disabled for that line, whichever matches the existing repo convention. CSP `img-src 'self'` already allows it.

## Database
Forward-only migration `backend/migrations/000044_screen_work_poster_mirror.up.sql` and `.down.sql`. The down migration drops the indexes and columns and rewrites `w500` back to `w342`. Agents never run it.
```sql
ALTER TABLE screen_works
    ADD COLUMN poster_copy_source_url      TEXT,
    ADD COLUMN poster_copy_path            VARCHAR(80),
    ADD COLUMN poster_copy_status          VARCHAR(16)
        CHECK (poster_copy_status IN ('pending', 'stored', 'failed')),
    ADD COLUMN poster_copy_error           VARCHAR(32),
    ADD COLUMN poster_copy_attempts        INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN poster_copy_next_attempt_at TIMESTAMPTZ,
    ADD COLUMN poster_copy_fetched_at      TIMESTAMPTZ,
    ADD COLUMN poster_copy_checked_at      TIMESTAMPTZ,
    ADD COLUMN poster_tmdb_ref             VARCHAR(24)
        CHECK (poster_tmdb_ref ~ '^(movie|tv)/[0-9]{1,10}$'),
    ADD COLUMN poster_verified_at          TIMESTAMPTZ,
    ADD COLUMN poster_refresh_at           TIMESTAMPTZ;

CREATE INDEX idx_screen_works_poster_copy_due ON screen_works (poster_copy_next_attempt_at)
    WHERE poster_source IN ('commons', 'tmdb');
CREATE INDEX idx_screen_works_poster_copy_path ON screen_works (poster_copy_path)
    WHERE poster_copy_path IS NOT NULL;
CREATE INDEX idx_screen_works_poster_refresh ON screen_works (poster_refresh_at)
    WHERE poster_source = 'tmdb';

UPDATE screen_works
   SET poster_url = 'https://image.tmdb.org/t/p/w500/' || substr(poster_url, length('https://image.tmdb.org/t/p/w342/') + 1)
 WHERE poster_source = 'tmdb' AND poster_url LIKE 'https://image.tmdb.org/t/p/w342/%';
```
- `poster_copy_fetched_at` is the counterpart of `books.cover_stored_at`: the time the stored file was downloaded. The repository descriptor maps it as the "stored at" column.
- `chk_screen_works_poster` is unchanged. The copy columns are independent of it, and every writer that clears `poster_url` also clears them.
- Existing TMDB rows get `poster_refresh_at` NULL, so they are due at once and are dropped (no `poster_tmdb_ref`). A later sync, with the key set, refills them with a ref. This only affects local data.

## API
No new HTTP routes. `poster.url` in `GET /books/{id}/adaptations`, `GET /authors/{id}/screen-works`, the admin lists and `GET /admin/screen-works/{id}` can now be `/covers/...`. Update the swagger description of the poster `url` field.

## Frontend
- `apps/web/src/widgets/footer/ui/footer.tsx` and `footer.test.tsx` (D7), plus `public/brands/tmdb.svg`.
- No change to `entities/screen-work/ui/screen-work-poster.tsx`. A relative `/covers/` URL is already optimised by `next/image` (only `^https?://` is `unoptimized`), and `/covers/*` is already rewritten in web and admin. The admin `screen-work-poster-section` uses a plain `<img>`, which works through the admin `/covers` rewrite.

## Existing Patterns
- Guarded state UPDATE and GC: `covers/repository.SaveState` and `covers/service.collect`/`collectOrphans`.
- After-commit enqueue that never fails the save: `adminservice.WithCoverEnqueuer`.
- Thin maintenance cmd: `cmd/similarity/main.go` plus the `backend/Dockerfile` build/copy lines.
- Optional capability via an option: `screenworksservice.WithTMDB`.
- Key-safe TMDB errors: `pkg/tmdb`.

## Risks
- **Orphan GC regression.** If `ReferencedPaths` misses `screen_works`, every poster copy is deleted an hour after it is stored. Cover it with a test.
- **Shared queue throughput.** Covers and posters share 1 download/s and the 50-item sweep batch. A few thousand posters backfill in hours, which is fine. Covers can be delayed by a large poster backlog; the oldest-waiting-first ordering keeps it fair.
- **TMDB rejection.** Dropping the poster when the key is unset is stricter than strictly necessary, because hotlinking is allowed. It is chosen so that no unverifiable TMDB data outlives the terms. An operator who removes the key loses all TMDB posters within one re-check cycle, at most 175 days.
- **Purge vs. sync.** If the key is still set, the next sync refills TMDB posters. The command warns, and `DEPLOYMENT.md` orders the steps.
- **Logo asset licence/availability.** The asset must be the official file. The developer must not draw it.
- **Backups.** `covers-data` (covers and poster copies) is a rebuildable cache and needs no backup. `posters-data` (uploads) still does; this stays a parked prod item.
- **Local compose.** No new volume. Agents must not bring up a second compose stack or run `tmdbpurge` against the shared dev database.
