# Tasks

## 1. Backend: FantLab source (pkg/bookdata)

- [x] 1.1 Confirm the FantLab response shape with one manual request each to `https://api.fantlab.ru/work/7193/extended` and `https://api.fantlab.ru/edition/16469`, and check the P7439 value format on one Wikidata item. Record the verified field names in a comment in `fantlab.go`; tests use inline JSON (no testdata fixtures saved). Verify: the field names are recorded in a comment in `fantlab.go`.
- [x] 1.2 Add `OrderFantLabWorkIDs` (D1) with table tests. Verify: `go test ./internal/pkg/bookdata/ -run FantLab`.
- [x] 1.3 Add P7439 to `BuildAuthorListQuery` and parse it into `ExternalBookRecord.FantLabWorkIDs`. Add `WikidataClient.FetchFantLabWorkIDs` (batch ≤50, QID validation). Update the query-shape tests and add parser and fixture tests. Verify: `go test ./internal/pkg/bookdata/`.
- [x] 1.4 Add `FantLabClient` with a 1.2 s limiter, `MaxRetries` 1, the per-call 4-request cap, an optional request budget and `FindSelfTitledEditionCover` (D2). Cover every case listed in D2 with fixture-server tests. Verify: `go test ./internal/pkg/bookdata/`.

## 2. Backend: harvest and backfill

- [x] 2.1 Add `WithFantLabCovers` and the FantLab step in `enrichRecord`, between the OL-ID step and the title search, with a 250-request budget per job (D3). Wire it in `cmd/worker/main.go`. Extend `handler_test.go` with the call order, the case where the FantLab cover blocks the title-search cover while the description still merges, the case where an OL-ID cover skips FantLab, error fall-through, and the cap. Verify: `go test ./internal/modules/harvest/...`.
- [x] 2.2 Extend `RunCoverBackfill` and `cmd/olcoverbackfill` (D4): fetch FantLab IDs per batch, try FantLab after OL, use `fantlab:<work>/<edition>` source IDs, add the new totals, and set the 90-minute context. Extend `cover_backfill_test.go`: FantLab finding, OL wins, FantLab error, Wikidata FantLab-batch error, and apply of a FantLab finding. Verify: `go test ./internal/modules/harvest/... ./cmd/olcoverbackfill/...`.

## 3. Backend: link-only covers in the mirror

- [x] 3.1 Add the forward migration `000045_book_cover_linked` (up and down, D5 Database). Confirm the constraint name with a read-only `\d books` against the dev DB. Do not run migrate down. Verify: the migration applies with the normal `migrate up` used by the stack, or the repo's migration test.
- [x] 3.2 Add `CoverStatusLinked`, `COVERS_LINK_ONLY_HOSTS` config (default `fantlab.ru,*.fantlab.ru`) with config tests, reuse the existing `coverfetch.MatchHost`, and add `WithLinkOnlyHosts` wired in `cmd/worker` (and in `cmd/api` if it builds the covers service). Verify: `go test ./internal/config/ ./internal/pkg/coverfetch/`.
- [x] 3.3 `MirrorOne` link-only branch before `pace()`; `DueCandidates` includes `linked`. Tests: no fetch is called, the state is `linked`, next attempt is +7 d, the old stored file is collected, the weekly re-check makes no fetch, a host taken off the list fetches normally, posters are unaffected, and `DisplayCoverURL` shows the source for `linked`. Verify: `go test ./internal/modules/covers/... ./internal/modules/catalog/...`.
- [x] 3.4 Add `linked` to the admin `coverCopyResponse` enum and regenerate Swagger per the repo's documented step. Add a handler test for `cover_copy` = linked. Verify: `go build ./... && go vet ./... && go test ./internal/modules/admin/...`, gofmt on changed files only.

## 4. Frontend

- [x] 4.1 `packages/api-client`: add `'linked'` to `AdminCoverCopyStatus`. `apps/admin` `cover-copy-hint.ts`: add the `linked` case with the exact spec text and a test. Verify: `pnpm --filter admin exec tsc --noEmit`, `pnpm --filter admin lint`, the admin unit tests for the hint.
- [x] 4.2 `apps/web/src/shared/lib/hotlink-cover.ts` + test. Apply `unoptimized={isHotlinkCoverUrl(...)}` to every cover `<Image>` listed in the design Context. Add the `next.config.js` comment, but no remotePatterns entry. Add a `book-card` test that checks the FantLab `src` is rendered directly. Verify: `pnpm --filter web exec tsc --noEmit`, `pnpm --filter web lint`, the web unit tests. No native `next build`.

## 5. DevOps and docs

- [x] 5.1 Add `COVERS_LINK_ONLY_HOSTS` to `docker-compose.yml` and `docker-compose.prod.yml` next to `COVERS_ALLOWED_HOSTS`. Verify: `docker compose config` parses (do not start a second stack).
- [x] 5.2 `docs/DEPLOYMENT.md`: the env var, the backfill's FantLab step and its new totals, "run while no harvest is active", and how to switch FantLab to copies once permission arrives (move it between the two host lists and empty the web helper). `docs/ROADMAP.md`: mark #686 and add the follow-up for the attribution link to the FantLab edition page. Verify: the docs mention every new flag and variable.
- [x] 5.3 Validate the change: `npx -y @fission-ai/openspec@1.13.2 validate add-fantlab-covers --strict`.

## 6. Maintainer (after merge, not for agents)

- [ ] 6.1 Run `olcoverbackfill` in report mode, review the FantLab findings, then run with `-apply` (optionally with `-ids`). Check that the covers show on the site as FantLab links and that the admin edit page shows the `linked` hint.
