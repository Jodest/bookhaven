# Tasks

Safety rules for every group:
- Never run destructive DB commands: no `migrate down`, no `down-all`, no `docker compose down -v`, no DROP/TRUNCATE against a database.
- Apply migrations only with `docker compose exec backend go run ./cmd/migrate up`.
- Run `gofmt -w` only on the files you changed, never on the whole backend.
- Never run a native `next build`: CI is the typedRoutes gate.
- Unit tests MUST NOT touch the network: use `coverstore.MemStore` or `t.TempDir()`, and fake `RoundTripper`/dialers instead of real or `httptest` servers.

## 1. Backend: schema, storage, downloader

- [x] 1.1 Add the forward-only `backend/migrations/000037_book_cover_mirror.{up,down}.sql` as in design.md. Check that `000037` is still free on `origin/dev` first. Do not edit earlier migrations. Verify with `migrate up` (command above) and `\d books` read-only.
- [x] 1.2 Add the cover fields to `catalog/domain.Book` with `gorm:"->"`. Add the status/error constants and the pure `DisplayCoverURL` (method and free function). Unit-test every branch of the display rule: no link, stored match, stored mismatch, absent match, absent mismatch, pending, failed. Add a SQLite repository test proving that an admin `UpdateBook` full save does not overwrite cover columns set by raw SQL.
- [x] 1.3 `internal/pkg/coverstore`:
  - the `Store` interface, the name validator, `FSStore` (atomic temp file + rename, creates dirs, `List` walks shards) and `MemStore`;
  - the same table-driven contract test run against both (`FSStore` on `t.TempDir()`): Put/Open/Exists/Delete/List, the marker, and rejection of invalid names (`../x`, uppercase hex, wrong extension).
- [x] 1.4 `internal/pkg/coverfetch`: allowlist matching, the public-address guard, scheme/port checks, the redirect cap with a per-hop re-check, the 30 s timeout, the 5 MiB limit, magic-byte sniffing, the Open Library `default=false` rewrite, and error classification into the `cover_error` codes with transient/permanent flags. Tests use a fake `RoundTripper` and call the guard directly on `netip.Addr` values. Cover: redirect to `10.0.0.5`, 127.0.0.1, `::1`, `169.254.169.254`, IPv4-mapped IPv6, 6 redirects, 5 MiB + 1 bytes, a GIF, HTML with `image/jpeg`, OL 404 → absent, 503 → transient, 403 → permanent.

## 2. Backend: covers module, hooks, serving

- [x] 2.1 `internal/modules/covers`:
  - `domain` (backoff: 10m·2^(n-1) capped at 24h, 6 attempts, 7-day re-try after exhaustion);
  - `repository` (GORM + raw guarded updates; due-candidate, verify-batch and referenced-path queries);
  - `service` (`MirrorOne`, `Sweep` with the 4 steps of design.md, GC of the previous path, injectable clock/sleep pacing ≥ 1 s);
  - tasks `covers:mirror` / `covers:sweep` with the options from design.md.

  Tests (MemStore, fake fetcher, SQLite):
  - success stores and sets `/covers/…`;
  - a link changed mid-download → guarded update is a no-op;
  - transient failure → backoff;
  - the 6th failure → failed + 7 days;
  - permanent → no next attempt;
  - `reason=save` resets attempts;
  - missing file → pending;
  - new marker with stored rows → all pending;
  - orphan GC keeps referenced and young files;
  - a shared file survives one book's cover change.
- [x] 2.2 `admin/service`: `WithCoverEnqueuer`, enqueue in `CreateBook`/`UpdateBook` per design.md, suppression in the `withRepo` copy, and enqueue after commit in `ImportBooks`. Tests with a fake enqueuer:
  - create with a cover → 1 call;
  - update with the same stored link → 0;
  - update with a new link → 1;
  - an import of 3 rows with covers → 3 calls, made after commit, none for a failed row;
  - an enqueuer error does not fail the save.
- [x] 2.3 Display URL in responses:
  - replace `CoverURL` mappings with `DisplayCoverURL()` in the catalog, shelves, home and curated handlers (grep all `transport/http`);
  - fix `curated/repository.Covers` and the `home/repository` `RequireCover` filter per design.md;
  - admin `bookResponse` keeps the source and adds `cover_copy`.

  Extend the existing handler tests: a stored copy gives `/covers/…` publicly but the source on the admin record, and absent gives `null`. Extend the home repository SQLite test: an absent copy is excluded from the cover pools.
- [x] 2.4 Covers HTTP handler: `GET|HEAD /covers/:shard/:file`, public, in `cmd/api`. Test against a MemStore: 200 with headers, 304 on `If-None-Match`, 404 for an unknown name, traversal and an empty name, HEAD without a body.
- [x] 2.5 Config `COVERS_DIR`, `COVERS_MIRROR_ENABLED`, `COVERS_SWEEP_CRON`, `COVERS_SWEEP_BATCH`, `COVERS_ALLOWED_HOSTS` with defaults and `config_test.go` cases. Wiring:
  - `cmd/api`: store (read-only use), handler, enqueuer on the admin service;
  - `cmd/worker`: an asynq client as the enqueuer for the worker's admin service (harvest), a `covers` server with `Concurrency: 1`, and the sweep cron in the scheduler, skipped when disabled.
- [x] 2.6 Update the swagger annotations (`cover_copy`; `/covers` is not documented there because it lives outside the `/api/v1` base path) and regenerate `backend/docs`. Verify from `backend/`: `go build ./...`, `go vet ./...` and `go test ./...`.

## 3. DevOps

- [x] 3.1 `docker-compose.yml`: named volume `covers-data`. Mount it at `/data/covers` in `worker` (rw) and `backend` (`:ro`), with `COVERS_DIR=/data/covers` on both and the `COVERS_*` envs with their defaults next to the other worker envs.
- [x] 3.2 `docker-compose.prod.yml`: the same volume and mounts (worker rw, backend `:ro`) and `COVERS_*` envs. Add `SITE_URL: https://${WEB_DOMAIN}` to `web`, and `covers-data` under `volumes:`.
- [x] 3.3 `backend/Dockerfile` production stage: pre-create `/data/covers` owned by `nonroot:nonroot` (an empty dir from the builder, copied with `--chown`), so a fresh volume is writable by the worker. Add `backend/storage/` to `.gitignore`. Verify with `docker compose config` (no `up`/`down -v` needed).

## 4. Frontend

- [x] 4.1 `apps/web/next.config.js`: the `/covers/:path*` rewrite and `images.minimumCacheTTL: 2592000`. `apps/admin/next.config.js`: the `/covers/:path*` rewrite (`API_URL || NEXT_PUBLIC_API_URL || localhost`).
- [x] 4.2 `apps/web`: the `SITE_URL` server-side helper and `absoluteCoverUrl`. `book-seo.ts` uses it for the Open Graph and JSON-LD images. Extend the book-seo tests: an absolute Open Library cover is unchanged, `/covers/…` gets prefixed, and no cover means no image.
- [x] 4.3 `packages/api-client`: the `AdminCoverCopy` type and the optional `cover_copy` field on the admin book (the admin types are wire-shaped snake_case, so there is no mapping and no mapping test).
- [x] 4.4 `apps/admin` `features/manage-book`: `coverCopyHint` in `lib/` with tests for every status and reason text in the spec. Render it under «URL обложки» in edit mode only, and extend the book-form test (hint shown on edit, absent on create).
- [x] 4.5 Verify in `apps/web`, `apps/admin` and `packages/api-client`: `tsc --noEmit`, `next lint` (apps) and the unit tests. Manual dev check after `docker compose up -d` (never `down -v`): a book with an OL cover first shows the OL link, then after a sweep `/covers/…`. `curl -I http://localhost:3000/covers/<file>` returns `immutable`, and `/_next/image?url=%2Fcovers%2F…&w=384&q=75` returns 200.

## 5. Docs

- [x] 5.1 `docs/DEPLOYMENT.md`: covers section covering
  - the `covers-data` volume (worker rw, backend ro), the `COVERS_*` envs and `SITE_URL`;
  - no backup needed and why (the sweep restores from `cover_url`);
  - what happens on a fresh volume or DB;
  - native default `./storage/covers`, and the caveat about mixed native/docker runs;
  - the allowlist, and how to add a host.
- [x] 5.2 `docs/admin-guide/adding-content.md`: the «URL обложки» hint states and what to do on failure: use an allowed host, or re-save to retry.
- [x] 5.3 `docs/ROADMAP.md`: a `- [x]` done entry for #489 in the house style. Mention migration `000037_book_cover_mirror`, the `covers` queue and OpenSpec change `add-self-hosted-covers`.
