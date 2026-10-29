# Design

## Context

- `enrichRecord` (`backend/internal/modules/harvest/service/handler.go`, around line 548) runs these steps: the ISBN lookup, then the Open Library ID cover (`openLibraryCoverFinder`, #688), then the strict title search (`titleAuthorMatcher`, #687). `enrich` is the `*OpenLibraryClient`. FantLab is a different source, so it has to be a separate handler dependency.
- `BuildAuthorListQuery` (`pkg/bookdata/wikidata.go`) already adds `OPTIONAL { ?item wdt:P648 ?olid }` plus `GROUP_CONCAT(DISTINCT …)` → `ExternalBookRecord.OpenLibraryIDs`. `WikidataClient.FetchOpenLibraryIDs` (`wikidata_openlibrary_ids.go`) is the batched lookup of 50 QIDs.
- The backfill is `cmd/olcoverbackfill` + `harvest/service/cover_backfill.go` (`RunCoverBackfill`, interfaces `CoverBackfillBooks`, `OpenLibraryIDSource`, `CoverByIDFinder`).
- The mirror: `covers/service/service.go` `MirrorOne` calls `pace()` and then `fetcher.Fetch`. The fetcher (`pkg/coverfetch`) rejects hosts off the allowlist with the permanent error `host_not_allowed` → `failed`. `covers/repository.DueCandidates` picks rows where `cover_source_url <> cover_url` or (`status IN ('pending','failed')` and `next_attempt_at <= now`). Without a recorded state, a FantLab row would be picked by every sweep, ahead of the others (`NULLS FIRST`).
- `books.cover_status` has an inline CHECK `('pending','stored','failed','absent')` (migration 000037). Postgres names it `books_cover_status_check`. The latest migration is 000044.
- Web covers are rendered with `next/image` (optimized) in `entities/book/ui/book-card.tsx`, `widgets/{similar-books,book-of-the-day,personal-home/want-to-read-pick}`, `features/pick-random-book`, `entities/curated-collection/ui/collection-card.tsx`, `views/{book-detail,collection-detail,shelf-detail,public-shelf-detail}`. `recent-reviews` and `literary-calendar` use a plain `<img>`, which needs no change. The CSP `img-src 'self' data: blob: https:` already allows fantlab.ru. `apps/admin` renders no book cover images (the admin record has the source link and the hint only).

## Decisions

### D1. FantLab work IDs in the harvest query, not persisted

- `BuildAuthorListQuery`: add `OPTIONAL { ?item wdt:P7439 ?fantlab . }` and `(GROUP_CONCAT(DISTINCT ?fantlab; separator="|") AS ?fantlabIds)`. `ExternalBookRecord.FantLabWorkIDs []string` is parsed like `OpenLibraryIDs`: split on `|`, trimmed, no filtering.
- New `WikidataClient.FetchFantLabWorkIDs(ctx, qids) (map[string][]string, error)` in `pkg/bookdata/wikidata_fantlab_ids.go`. It is a copy of `FetchOpenLibraryIDs` with P7439: 1..50 QIDs, validated, `ErrInvalidQuery` without a call. Optionally, one shared private helper takes the property ID. The two queries stay separate so the existing P648 tests remain untouched.
- No column. The data is read again from Wikidata when needed, the same as D1 of #688.
- P7439 values are plain digits (verified live, e.g. `363346`). `OrderFantLabWorkIDs(ids []string) []int` accepts `^(?:work)?(\d+)$` defensively (trimmed, case-insensitive prefix), dedupes, sorts ascending and cuts to `MaxFantLabWorkIDsPerWork = 2`. Table-tested.

### D2. `FantLabClient` in `pkg/bookdata/fantlab.go`

```go
func NewFantLabClient(opts ...Option) *FantLabClient // base https://api.fantlab.ru, DefaultUserAgent
// Returns the cover link and "<workID>/<editionID>", or ErrNotFound.
func (c *FantLabClient) FindSelfTitledEditionCover(ctx context.Context, title string, workIDs []string) (coverURL, usedID string, err error)
```
- Reuse `newConfig`/`doRequest` (User-Agent, retries on 429/5xx with `Retry-After`). Default limiter: `rate.NewLimiter(rate.Every(1200*time.Millisecond), 1)`. Pass it through a FantLab-specific default and not `WithRateLimit` with a float, so 1.2 s is exact. Use a small retry count (`WithMaxRetries(1)`): a 30-minute job must not sleep through repeated backoffs.
- **Request budget.** The client has `SetRequestBudget(n)` (negative = unlimited) that the handler calls at the start of each job (see D3). It is spent in a wrapper around the limiter, so every outgoing HTTP attempt counts, retries included. When the budget is spent, the client returns `ErrBudgetExhausted` without calling. The client also stops after 4 requests for one `FindSelfTitledEditionCover` call.
- **Work.** `GET /work/{id}/extended` → decode only the fields we need: `work_name` and `editions_blocks`. Verified live shape: `editions_blocks` is an object whose KEYS are numeric strings; each value has `block` (`paper`, `audio`, `magazin`, `books_plan`, …) and `list[]` with `edition_id`, `name`, `lang_code`, `year`, `pic_num`, `type`. The shape is recorded in a comment in `fantlab.go`; tests use inline JSON. 404 → next ID.
- **Self-titled rule.** `NormalizeMatchText(work_name) == NormalizeMatchText(title)`, and the same for the edition `name`. An empty normalized value never matches. No subtitle cut (do not use `TitleMatches`). Only blocks whose `block` field is `paper` (skip `audio`, `magazin`, `books_plan` and anything else). Only `lang_code == "ru"`.
- **Order.** Year descending (0/missing last), then edition ID ascending. At most 3 editions.
- **Image check.** List entries have no image field, only `pic_num`. An entry with `pic_num` = 0 has no image (skipped with no request). Otherwise (`pic_num` > 0 or missing) `GET /edition/{edition_id}` confirms a non-empty `image`. Never request `fantlab.ru/images/...` itself. The cover link is `https://fantlab.ru/images/editions/big/{edition_id}`, without the `?r=` cache-buster, so it stays stable and deduplicates.
- **Errors.** As in D3 of #688: per ID, move on. Return `ErrNotFound` when nothing matched. Return the last transport error when one occurred and nothing was found, so the backfill can tell `error:` from "no cover". `ErrBudgetExhausted` is returned as is.
- Fixture-server tests (`fantlab_test.go`): the self-titled hit, the work-name mismatch, a collection name, audio and English editions skipped, newest-with-image ordering (2015 no image → 2008), a list entry with an image needing no edition request, the 404 on the first ID and the hit on the second, 503 → error, the 4-request cap per call, budget exhaustion with no request, and the limiter spacing (fake limiter).

### D3. Harvest step

- `HandlerOption WithFantLabCovers(f FantLabCoverFinder)`, where `FantLabCoverFinder` is a local interface for `FindSelfTitledEditionCover`. Nil means the step is off (tests, CLI harvest tools).
- In `enrichRecord`, between the Open Library ID step and the title search: `if rec.CoverURL == nil && len(rec.FantLabWorkIDs) > 0 && fantlab != nil` → call it with `rec.Title`; on success set `rec.CoverURL`; ignore every error. `enrichRecord` gets the finder as a parameter, or becomes a method. Keep the signature change small and update the callers and tests.
- **Per-job cap of 250 requests.** The handler creates the budget at the start of each `ProcessTask` and passes it via context or a per-job wrapper, so concurrent jobs cannot share state (jobs are serialized anyway). Log once when the cap is hit (`zap.Info`, count of skipped works). No new result field.
- Wiring in `cmd/worker/main.go` (near line 232): `bookdata.NewFantLabClient()` → `harvestservice.WithFantLabCovers(...)`. A new env toggle `HARVEST_FANTLAB_COVERS_ENABLED` (default true) is optional and not required. Leave it out unless trivial.
- `handler_test.go`: extend the fakes to record the call order: ISBN → OL IDs → FantLab → title. Cover the cases where the FantLab cover blocks the title-search cover but the title-search description is still merged, an OL ID cover skips FantLab, and a FantLab error falls through to the title search.

### D4. Backfill extension

- `CoverBackfillOptions` gets `FantLab` (`FantLabCoverFinder`, nil = skip) and `FantLabIDs` (`FantLabIDSource`, `FetchFantLabWorkIDs`), so the `RunCoverBackfill` signature is unchanged. Per batch of 50: call `FetchOpenLibraryIDs` as today, then `FetchFantLabWorkIDs` only for the books of the batch without an OL finding. A FantLab-IDs batch error adds `error: wikidata: …` lines only for books that have no OL finding.
- For each book without an OL finding but with FantLab IDs: `FindSelfTitledEditionCover(ctx, b.Title, ids)`. `ErrNotFound` → a `nocover` line with `fantlab:<ids>`; other error → `error`; success → a finding with `OLID` (rename the field `SourceID`) = `fantlab:<work>/<edition>`. No request budget in the CLI (pass none). About 365 works at ≤4 requests × 1.2 s is up to ~30 minutes for FantLab alone, on top of the Wikidata and Open Library calls. That does not fit the current 30-minute context, so **raise the command context to 90 minutes**.
- A book whose OL step returned an error is still tried on FantLab. If FantLab finds a cover, the finding replaces the error line; otherwise both lines are kept.
- Totals line: `candidates=N with_ids=M with_fantlab_ids=F found=K found_fantlab=J nocover=L errors=E`. `found` keeps counting OL findings, so existing tests keep their meaning.
- Apply is unchanged (`setCover`). The FantLab link then goes through `UpdateBook` → the copy is queued → `linked` (D5).
- `cmd/olcoverbackfill/main.go`: build `bookdata.NewFantLabClient()` and pass the Wikidata client as both ID sources. The command name stays (renaming would break the documented run line). The usage text mentions FantLab.

### D5. Link-only hosts in the mirror

- Config: `COVERS_LINK_ONLY_HOSTS` (default `coverfetch.DefaultLinkOnlyHosts = "fantlab.ru,*.fantlab.ru"`), parsed with `coverfetch.ParseHosts`. An explicitly empty value means no link-only hosts. Reuse the existing exported allowlist matcher `coverfetch.MatchHost` (no new function).
- `covers/service`: `WithLinkOnlyHosts(hosts []string)` option. In `MirrorOne`, after the `HasStoredCopy` check and **before `pace()`**, when `owner == domain.OwnerBook` and the parsed link's host matches: save `StateUpdate{SourceURL: link, Status: "linked", NextAttemptAt: now+7d, Attempts: 0, Error: nil, Path: nil}`, then run the usual `prev` collection, and return. No fetch and no `lastCall` update. Unparseable links fall through to the normal path, where the fetcher rejects them.
- `catalogDomain.CoverStatusLinked = "linked"`. `DisplayCoverURL` needs no change: anything but stored/absent shows the source link. Add a test case anyway.
- `DueCandidates`: the status condition becomes `IN ('pending','failed','linked')` for every owner. The posters column never holds `linked`, so this is harmless. Because of `next_attempt_at`, linked rows come back weekly. If the host is still link-only, the row is re-recorded without a request; if it is not, the normal fetch runs.
- Make sure `ResetAllStored`, `VerifyBatch` and `MarkMissing` ignore `linked` (they filter on `stored`; confirm with a test).
- The home "books with cover" pool (`home/repository` around line 168) excludes only `absent`, so `linked` counts as having a cover. Confirm, no code change expected.

### D6. Frontend

- `apps/web/src/shared/lib/hotlink-cover.ts`: `isHotlinkCoverUrl(url)` → true for `https:` URLs whose host is `fantlab.ru` or ends with `.fantlab.ru` (same style as `isWikimediaUrl`), with unit tests. Its comment says it must stay in step with `COVERS_LINK_ONLY_HOSTS`. It is a frontend constant, because the API gives no flag. This is acceptable for one host. If more hosts are added later, consider a `cover_hotlink` flag in the API.
- Every cover `<Image>` listed in Context gets `unoptimized={isHotlinkCoverUrl(src)}`. Do not add `fantlab.ru` to `images.remotePatterns`: the optimizer would then fetch the image and cache it for 30 days, which is a stored copy. With `unoptimized`, `next/image` skips the loader and its remotePatterns check. Add a comment in `next.config.js` that explains why it is left out.
- Add a component test (for example `book-card`) that checks that a FantLab `coverUrl` renders an `<img>` whose `src` is the FantLab link itself, not `/_next/image?...`.
- `packages/api-client`: `AdminCoverCopyStatus` gets `'linked'`. `apps/admin/src/features/manage-book/lib/cover-copy-hint.ts` gets a `linked` case with the exact text from the spec, plus a test.
- Swagger: the `enums` tag on `coverCopyResponse.Status` gets `linked`. Regenerate `backend/docs` the way the repo does it (check the Makefile/README; `swag init` only if that is the documented step).
- FSD: the helper is in `shared/lib`, so entities, widgets and views may import it.

## API

- No new endpoints. `cover_copy.status` in the admin book record (`GET/POST/PATCH /api/v1/admin/books[/{id}]`, the admin list) can be `linked`. Auth is unchanged: admin/moderator role and bearer as today.
- The public `cover_url` can now be a `https://fantlab.ru/images/editions/big/{id}` link.

## Database

- Forward migration `000045_book_cover_linked.up.sql`:
  ```sql
  ALTER TABLE books DROP CONSTRAINT IF EXISTS books_cover_status_check;
  ALTER TABLE books ADD CONSTRAINT books_cover_status_check
      CHECK (cover_status IN ('pending', 'stored', 'failed', 'absent', 'linked'));
  ```
  The down file maps `linked` rows to `pending` and restores the old CHECK. It is written for completeness and never run by agents. Verify the real constraint name in the dev DB with a read-only query (`\d books`) before you rely on `IF EXISTS`. If the name differs, use the real name. Never edit 000037.
- No data migration. Existing rows are unaffected. The backfill writes only through `UpdateBook`, sets only cover links that were empty, and deletes nothing.

## Existing Patterns

- `pkg/bookdata/openlibrary_cover.go` + `openlibrary_ids.go` (#688): the ID ordering helper, the per-ID fall-through and error semantics, and fixture tests.
- `pkg/bookdata/wikidata_openlibrary_ids.go`: the batched P-property lookup.
- `harvest/service/cover_backfill.go` + `cmd/olcoverbackfill`: the report-first CLI, re-read before save, and the `-ids` filter.
- `apps/web/src/entities/author/lib/is-wikimedia-url.ts`: the unoptimized hotlink helper.
- `covers/service/service.go` `MirrorOne` and the `absent` handling: the precedent for a non-download terminal state.

## Risks

- **FantLab API shape is assumed** from the owner's probe (`editions_blocks.*.list[]`, `edition_id`, `name`, `lang_code`, `type`; edition card `image`). The developer must confirm the field names and the block key on one live response (a single manual request, within the crawl delay) and pin it as a fixture. If the shape differs materially, adapt the decoding, not the rules.
- **Self-titled collections.** A collection named exactly like its title story (for example «Пикник на обочине» with extra stories) passes the rule. Its cover still carries the work's title, which the owner accepted as a "self-titled edition". Expected yield is about 25-30 books.
- **Hotlinking.** FantLab may block hotlinks by `Referer` or change URLs. Covers would then appear broken. Before merge, a manual check in the browser on local compose shows one FantLab cover rendering. Before going to prod, ask the owner whether to set `referrerPolicy="no-referrer"` on hotlinked images. It is not specced now. Permission for copies is still pending. When it is granted, remove `fantlab.ru` from `COVERS_LINK_ONLY_HOSTS` and add it to `COVERS_ALLOWED_HOSTS`: the weekly re-check then copies everything with no code change. The frontend helper also has to be emptied then.
- **Frontend/backend host lists can drift** (D6). There is one host for now, and comments link the two.
- **Job duration.** The cap of 250 requests × 1.2 s adds at most ~5 minutes to a 30-minute job. Larger authors are covered by the backfill.
- **Two processes.** The backfill and a harvest running at the same time would each keep 1.2 s, so FantLab would see ~0.6 s spacing. The operator runs the backfill while no harvest is active (document this in DEPLOYMENT.md).
- **The admin `pending` blip.** Right after a save, a FantLab link shows `pending` («Обложка копируется…») until the queued task records `linked` (seconds). This is accepted.
- **Agents must not run `olcoverbackfill -apply`** or start a second compose stack. Running it is the maintainer's step after merge. No `migrate down`.
- **Follow-up (out of scope):** an attribution link to the FantLab edition page on the book page. Note it in the ROADMAP.
