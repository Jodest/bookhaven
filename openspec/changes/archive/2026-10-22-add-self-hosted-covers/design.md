# Design

## Context

Covers today are raw `books.cover_url` links, mostly `covers.openlibrary.org/b/id/…-L.jpg`. They go through 2 redirects to archive.org, take 2.3–2.9 s and are not cacheable. The Next optimizer cache is per width, has a 60 s TTL and lives in the container. 24 of 683 books have a cover today, and harvest will add more.

The 3 write paths all funnel into `admin/service.Service.CreateBook` / `UpdateBook`:
- the admin HTTP handlers;
- `ImportBooks`, which calls `tx.CreateBook` inside `WithinTransaction` per row;
- the harvest handler (`admin.CreateBook` in the worker, with `enrichRecord` filling the cover from Open Library).

`UpdateBook` loads the full `catalogDomain.Book` and the repository saves the whole row.

## Decisions

1. **Copy state lives on `books`, read-only for GORM.** The new columns map to `catalogDomain.Book` fields tagged `gorm:"->"`: GORM reads them on every `Book` load and never writes them on Create/Save/Updates. So the admin full-row save cannot overwrite a copy the worker stored meanwhile, and every module that loads `Book` gets the fields for free. Only the covers repository writes them, with raw `UPDATE books SET cover_… WHERE id = ? AND cover_url IS NOT DISTINCT FROM ?`. The raw `UPDATE` does not touch `updated_at`. The guard on the source makes a copy of a link the admin has already replaced a no-op, and that orphan file is left for GC. Verify that the SQLite `AutoMigrate` used in the repository tests still creates `->` columns. If it does not, add them in the test schema setup.
2. **A copy is valid only for its source.** `cover_source_url` records the link a copy was made from. The displayed link is computed by a pure function, `catalogDomain.Book.DisplayCoverURL() *string`, plus a free `DisplayCoverURL(coverURL, sourceURL, path, status)` for column-select queries:
   - `cover_url` empty → nil;
   - `cover_source_url == cover_url` and status `stored` → `"/covers/" + path`;
   - the same match and status `absent` → nil;
   - otherwise → `cover_url`.

   No write on the save path: changing `cover_url` makes the old copy invisible by itself. The admin `cover_copy.status` is `pending` whenever the source does not match.
3. **One hook in the admin service.** Add `WithCoverEnqueuer(CoverEnqueuer)`, where `CoverEnqueuer` is `EnqueueMirror(ctx, bookID uuid.UUID, reason string) error`; nil means no-op:
   - `CreateBook` enqueues when the normalized cover is non-nil;
   - `UpdateBook` enqueues when the cover is non-nil and the book (before the update) has no stored copy of the new link;
   - the copy returned by `withRepo` (transaction scope) sets `suppressCoverEnqueue`, so `ImportBooks` collects the created ids and enqueues each after its `WithinTransaction` returns nil.

   An enqueue error is logged at warn level and never returned. Wire it in `cmd/api` (existing asynq client) and in `cmd/worker` for the harvest (`asynq.NewClient`, closed on shutdown).
4. **Retries are owned by the DB, not asynq.** `covers:mirror` runs with `MaxRetry(0)`, `Timeout(2m)`, `Unique(10m)` and queue `covers`. The payload is `{book_id, reason: "save"|"sweep"}`. The handler:
   - loads the book; when it is gone or has no cover, it returns nil;
   - with `reason=save`, resets the attempts;
   - downloads, and stores the file on success;
   - updates the row as a guarded update;
   - garbage-collects the previous path when no row references it.

   Failures set `cover_status`, `cover_error`, `cover_attempts` and `cover_next_attempt_at` per the backoff table, and the handler returns nil (no asynq retry). `covers:sweep` is a cron task with `MaxRetry(0)`, `Timeout(30m)` and `Unique(30m)` on the same queue, which runs its batch inline.
5. **Own asynq server.** Queue `covers` gets its own server with `Concurrency: 1`, like `ai_descriptions`. The handler paces downloads at least 1 s apart, with an injectable clock and sleep, the same pattern as `aidescriptions.Handlers.pace`.
6. **Statuses and backoff** (`covers/domain`):
   - `pending`: never tried, or a transient retry is scheduled;
   - `stored`;
   - `failed`: permanent, or 6 transient failures;
   - `absent`: an Open Library 404.

   Transient errors are `timeout`, `network` and `http_error` for 408/429/5xx. Backoff: `next = now + 10m·2^(attempts-1)`, capped at 24h. Once the attempts reach 6, the copy becomes `failed` with `next = now + 7d` and the attempts reset to 0, which the sweep picks up again. Permanent errors (`host_not_allowed`, `blocked_address`, `too_many_redirects`, `too_large`, `not_image`, `http_error` for other 4xx) get `next = NULL`.
7. **Safe downloader (`pkg/coverfetch`).**
   - **Client.** A `Fetcher` built from an `http.Client` with a custom `Transport`: `DialContext` uses a `net.Dialer` whose `Control` rejects non-public addresses (`netip.Addr`: `IsLoopback`, `IsPrivate`, `IsLinkLocalUnicast/Multicast`, `IsMulticast`, `IsUnspecified`, IPv4-mapped forms, `100.64.0.0/10`, `0.0.0.0/8`). It also uses `TLSHandshakeTimeout 5s`, `ResponseHeaderTimeout 15s` and `Proxy: nil`.
   - **Redirects.** `CheckRedirect` caps at 5 and re-checks the scheme, port and host allowlist of each hop. The dial guard covers the address of every hop.
   - **Limits.** The overall `context.WithTimeout` is 30s. The body goes through `io.LimitReader(5 MiB + 1)`.
   - **Type.** Magic bytes: JPEG `FF D8 FF`, PNG `89 50 4E 47 0D 0A 1A 0A`, WebP `RIFF????WEBP`, giving `.jpg/.png/.webp`.
   - **Open Library.** For `covers.openlibrary.org`, add `default=false` to the query when it is missing.
   - **Allowlist.** `COVERS_ALLOWED_HOSTS` takes comma-separated entries; a leading `*.` allows subdomains. The default is `covers.openlibrary.org,archive.org,*.archive.org,upload.wikimedia.org`.
   - **Tests.** The guard and the allowlist are pure functions. Tests inject a fake `RoundTripper`/dialer: no sockets, no `httptest` server, no network.
8. **Storage interface (`pkg/coverstore`).**
   ```go
   type Store interface {
       Put(ctx context.Context, name string, data []byte) error   // atomic (temp file + rename for FS)
       Open(ctx context.Context, name string) (io.ReadSeekCloser, Info, error) // ErrNotFound
       Delete(ctx context.Context, name string) error             // missing = nil
       Exists(ctx context.Context, name string) (bool, error)
       List(ctx context.Context, fn func(name string, modTime time.Time) error) error
       EnsureMarker(ctx context.Context) (created bool, err error) // ".bookhaven-covers"
   }
   ```
   - `FSStore(root)` creates `root` on demand in the worker; the backend only reads. `MemStore` is for tests.
   - Names are `{sha256[0:2]}/{sha256}.{ext}`. A strict validator, `^[0-9a-f]{2}/[0-9a-f]{64}\.(jpg|png|webp)$`, is shared by the store and the HTTP handler, so traversal is impossible by construction.
   - An S3 implementation later only has to satisfy this interface.
9. **Sweep steps** (service `Sweep(ctx, batch)`):
   1. `EnsureMarker`. If the marker was just created and any row is `stored`, run `UPDATE … SET cover_status='pending', cover_path=NULL, cover_next_attempt_at=now WHERE cover_status='stored'`. This handles a lost or new volume.
   2. Verify up to 100 stored rows, oldest `cover_checked_at` first. A missing file sets `pending`, `path NULL`. Others get `cover_checked_at = now`.
   3. Pick up to `batch` due candidates and mirror them inline, paced. Candidates have `cover_url <> ''` and either `cover_source_url IS DISTINCT FROM cover_url`, or (`status IN ('pending','failed')` AND `next_attempt_at <= now`). Order: `next_attempt_at NULLS FIRST`, then `created_at`.
   4. Orphan GC: `List` the store and delete files whose name no `books.cover_path` references and whose `modTime` is older than 1h.
10. **Config** (`internal/config`):
    - `COVERS_DIR` (default `./storage/covers`);
    - `COVERS_MIRROR_ENABLED` (default `true`; off means no enqueue, no sweep, but files are still served);
    - `COVERS_SWEEP_CRON` (default `*/15 * * * *`; empty means off);
    - `COVERS_SWEEP_BATCH` (default 50);
    - `COVERS_ALLOWED_HOSTS`.

    The worker registers the sweep cron in the existing scheduler block in `cmd/worker/main.go`.
11. **Web absolute URL for SEO.** A server-only `SITE_URL` env, default `http://localhost:3000`, is read in `apps/web` shared config. `book-seo.ts` maps a cover starting with `/` to `${SITE_URL}${path}` before the `^https?://` check, and uses the same absolute value in JSON-LD.

## API

- `GET|HEAD /covers/:shard/:file` is new, at the root and not under `/api/v1`. It is public: no auth middleware, no CSRF. Behaviour:
  - a name that fails the validator → 404;
  - `Store.Open` → `ErrNotFound` → 404;
  - otherwise 200 with `Content-Type` by extension (`image/jpeg|png|webp`), `Cache-Control: public, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff`, `ETag: "<sha256>"`, `Content-Length`;
  - a matching `If-None-Match` → 304;
  - other methods → 405 (Echo default).

  Register it in `cmd/api` with the covers transport handler. Make sure any global rate limiter or body-logging middleware does not choke on images.
- **Every public or non-record response** keeps the field name and type of `cover_url` (`string|null`), but its value becomes `DisplayCoverURL()`. Grep `CoverURL` in all `transport/http` packages (catalog, shelves, home, curated, the admin curated/queue responses) and swap the mapping. `curated/repository.Covers` must select `cover_url, cover_source_url, cover_path, cover_status` and resolve, skipping nil results. In `home/repository`, the `RequireCover` filter becomes `cover_url IS NOT NULL AND cover_url <> '' AND NOT (cover_status = 'absent' AND cover_source_url = cover_url)`.
- **Admin book record** (`admin/transport/http.bookResponse`, used by list, get, create and update) keeps `cover_url` as the source and adds `cover_copy`: `{"status","error","stored_at"} | null`. Add a swagger annotation and regenerate `backend/docs`.
- **Authorization:** `/covers/*` is public. No other endpoint's auth changes.

## Database

Forward-only `backend/migrations/000037_book_cover_mirror.{up,down}.sql`. Check that no newer migration landed on `dev` first; if one did, renumber. Do not edit earlier migrations.

```sql
ALTER TABLE books
  ADD COLUMN cover_source_url      TEXT,
  ADD COLUMN cover_path            VARCHAR(80),
  ADD COLUMN cover_status          VARCHAR(16)
    CHECK (cover_status IN ('pending','stored','failed','absent')),
  ADD COLUMN cover_error           VARCHAR(32),
  ADD COLUMN cover_attempts        INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN cover_next_attempt_at TIMESTAMPTZ,
  ADD COLUMN cover_stored_at       TIMESTAMPTZ,
  ADD COLUMN cover_checked_at      TIMESTAMPTZ;
CREATE INDEX idx_books_cover_due  ON books (cover_next_attempt_at) WHERE cover_url IS NOT NULL AND cover_url <> '';
CREATE INDEX idx_books_cover_path ON books (cover_path) WHERE cover_path IS NOT NULL;
```

The down migration drops the indexes and columns. It exists by convention only: tasks never run it. There is no backfill in SQL; the first sweep does it.

## Frontend

- **`apps/web/next.config.js`.**
  - Add the rewrite `{ source: '/covers/:path*', destination: \`${process.env.API_URL || 'http://localhost:8080'}/covers/:path*\` }` next to `/api`.
  - Set `images.minimumCacheTTL: 2592000` (30 days). This is safe because copies are content-hashed and fallback Open Library links are stable per id.
  - Keep the `covers.openlibrary.org` remote pattern for the fallback.
  - Relative `/covers/…` is a local image for `next/image`. Its optimizer fetches it through the app's own router, so through the rewrite, over the internal network.
- **`apps/admin/next.config.js`.** Add the same rewrite with `process.env.API_URL || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'`. Admin renders no covers today; the rewrite only keeps `/covers/…` links working on the admin origin.
- **`apps/web` SEO.** In `views/book-detail/lib/book-seo.ts`, use an `absoluteCoverUrl(cover)` helper in `shared/lib` or `shared/config` (FSD: views may import shared). Extend its test.
- **`packages/api-client`.** Add `AdminCoverCopy { status: 'pending'|'stored'|'failed'|'absent'; error: string|null; stored_at: string|null }` and an optional `cover_copy: AdminCoverCopy|null` on `AdminBook`; the admin types are wire-shaped snake_case, so there is no mapping. Public types are unchanged.
- **`apps/admin` `features/manage-book`.** A pure `coverCopyHint(copy)` in `lib/`, with tests for the texts from the spec, rendered under «URL обложки» only in edit mode from the `initial` book. No cross-feature imports.

## Existing Patterns

- Worker queue with its own server, `Concurrency: 1`, pacing, a sweep task and a cron register: `aidescriptions/service/tasks.go` and its wiring in `cmd/worker/main.go`.
- Optional service dependency via a functional option: `admin/service` `With…` options (`WithSimilarInvalidator`).
- Transaction-scoped service copy: `admin/service.withRepo` and `import.go`.
- `/api` rewrite in both `next.config.js` files.

## Risks

- **`gorm:"->"` columns.** Confirm that Create and Save skip them and that SQLite tests create them. Map-based `Updates` silently drops read-only fields, which is why writes are raw SQL.
- **Next optimizer and rewrites.** Confirm in dev that `/_next/image?url=%2Fcovers%2F…&w=384&q=75` returns 200 through the rewrite, in the docker web container (where `API_URL=http://backend:8080`). Rewrites are resolved when the production image is built. Check that the production `/covers` rewrite resolves to `backend:8080` the same way `/api` does today (Dockerfile.node build env). If it does not, document the fix in DEPLOYMENT.md rather than changing the build in this change.
- **Volume ownership.** The production image is distroless `nonroot`. A fresh named volume inherits the ownership of the mount point from the image, so the production stage must contain `/data/covers` owned by `65532:65532`: `COPY --from=builder --chown=nonroot:nonroot` an empty dir created in the builder. The backend mounts the volume `:ro`.
- **Mixed native/docker runs** (for example a native backend with the docker worker) use different storages. The source fallback keeps pages working, and the doc should mention it.
- **Import enqueue timing.** Enqueuing inside the per-row transaction could race the commit, hence suppression in the tx copy and enqueue after commit.
- **`absent` changes visible behaviour.** A book whose Open Library link was a blank placeholder now shows «Нет обложки» and leaves the cover-required book-of-the-day pools. This is intended (#489).
- **Migration number collision** with a parallel branch: renumber before merge if needed.
