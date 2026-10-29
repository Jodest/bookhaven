# Design

## Context

- `enrichRecord` (`backend/internal/modules/harvest/service/handler.go`, about line 548) does: ISBN lookup (`enrich.FindByISBN`), then, only if that found nothing, the strict `FindMatchByTitleAuthor` (#687). Wikidata books have no ISBN, and the Russian title search rarely matches, so they end up without covers.
- `BuildAuthorListQuery` (`backend/internal/pkg/bookdata/wikidata.go`, line 287) and `parseSparqlBindings` already carry the English labels (`TitleEn`, `ExternalAuthor.NameEn`), using `SAMPLE`/`GROUP_CONCAT DISTINCT` under `GROUP BY ?item ?itemLabel`.
- `OpenLibraryClient` (`backend/internal/pkg/bookdata/openlibrary.go`) already calls `/works/{id}.json` (`FetchWorkDescription`) through the shared `doRequest`, which handles the User-Agent, the ~1 req/s limiter and retries. Cover links are built with `CoverURL(id, CoverIDTypeID, CoverSizeLarge)`.
- `cmd/olcoveraudit` + `harvest/service/cover_audit.go` (#687) are the pattern for a report-first maintenance command. The logic sits behind small interfaces, and writes go through `adminservice.UpdateBook` with the full stored input.
- `WikidataClient.FetchAdaptations` (`wikidata_adaptations.go`) is the pattern for a batched `VALUES ?book { wd:Q… }` lookup that validates QIDs and caps the batch size.

## Decisions

### D1. P648 in the harvest query, not persisted

- `BuildAuthorListQuery`: add `OPTIONAL { ?item wdt:P648 ?olid . }` and `(GROUP_CONCAT(DISTINCT ?olid; separator="|") AS ?olids)`.
- `ExternalBookRecord` gets `OpenLibraryIDs []string`. The parser splits `olids` on `|`, trims the values and drops empty ones. It does not filter or order them (D2 does). Other queries leave the field empty.
- No migration and no OLID column. The ID is needed only at harvest time and for the one-off backfill, which reads it again from Wikidata. Storing it would add a column nobody reads afterwards.
- Update the query-shape tests in `wikidata_test.go` and add a parser test with two IDs.

### D2. Deterministic ID choice: an exported helper in `bookdata`

`func OrderOpenLibraryIDs(ids []string) []string` in a new `backend/internal/pkg/bookdata/openlibrary_ids.go`:
- keeps values matching `^OL(\d+)([WM])$` (trimmed, upper-case as given; no case folding);
- removes duplicates;
- puts works (`W`) before editions (`M`), each in ascending numeric order (compare the parsed integer, not the string);
- cuts the list to `MaxOpenLibraryIDsPerWork = 3`.

The issue's example Q332581 (`OL27479W`, `OL27516W`, `OL27455W`) gives `OL27455W, OL27479W, OL27516W`. Pure function, table-tested.

### D3. `OpenLibraryClient.FindCoverByOpenLibraryIDs`

```go
// Returns the cover link and the OL ID that produced it, or ErrNotFound.
func (c *OpenLibraryClient) FindCoverByOpenLibraryIDs(ctx context.Context, ids []string) (coverURL string, usedID string, err error)
```
- It applies `OrderOpenLibraryIDs` itself, so callers cannot get the order wrong.
- Per ID: `GET {baseURL}/works/{id}.json` for `W`, `GET {baseURL}/books/{id}.json` for `M`, through `doRequest`. Parse `{"covers": [int64...]}` and take the first value `> 0`, then build `CoverURL(strconv.FormatInt(v,10), CoverIDTypeID, CoverSizeLarge)`.
- A 404, an empty or all-non-positive list, a parse error, or a transport error on one ID moves on to the next ID. When every ID was tried and none gave a cover, return `ErrNotFound`. If the last error seen was a transport or source error and nothing was found, return that error instead, so the backfill can report `error:` rather than "no cover". The harvest treats both the same way.
- The context is checked between IDs.
- Do not reuse `FetchWorkDescription` and do not read `description`: cover only (owner's scope decision). A shared private helper `getJSON(ctx, path)` is fine if it keeps both methods short.
- Fixture-server tests in `openlibrary_test.go`: work hit, edition path, `[-1, 0, 123]` → 123, the first ID 404 and the second hit, all empty → `ErrNotFound`, 5xx on the only ID → error, invalid IDs dropped without a request.

### D4. Harvest enrichment order

In `enrichRecord`, keep the early return. Then:
1. ISBN lookup, unchanged. Its cover fills `rec.CoverURL` as today.
2. **New:** if the cover is still empty after step 1 (Wikidata nil and the ISBN result nil or without a cover) and `len(rec.OpenLibraryIDs) > 0`, and `enrich` implements a local interface `openLibraryCoverFinder { FindCoverByOpenLibraryIDs(...) }` (type assertion, same style as `titleAuthorMatcher`), call it. On success set `rec.CoverURL`. Any error is ignored.
3. Title search, unchanged condition: only when the ISBN lookup found nothing. The existing gap-filling (`if rec.CoverURL == nil`) means the title-search cover is used only when step 2 found nothing.

Implementation note: apply the ISBN-step cover to `rec` before step 2, then merge the title-search result after. The simplest way is to merge after each lookup (a small `mergeEnrichment(rec, found)` helper holding the existing field rules, including the Russian-description filter). The rest of the behavior stays exactly as it is.

Wire nothing new in `cmd/worker`: `enrich` is already the `*OpenLibraryClient`. Import-triggered and scheduled harvests use the same handler and get the step automatically. Existing books are never touched by a harvest (create-only rule unchanged).

### D5. Backfill command `cmd/olcoverbackfill`

Follows D5 of the archived `guard-harvest-cover-matching` design.

- **Flags:** `-apply` (default false); `-ids` (optional comma-separated UUIDs, valid only with `-apply`; `-ids` without `-apply` exits 2 with usage).
- **Service setup:** `config.Load`, `database.Open`, context of 30 minutes. `bookdata.NewWikidataClient()` and `bookdata.NewOpenLibraryClient()`, the latter without the work-description lookup. When `cfg.CoversMirrorEnabled`, an `asynq.Client` on `cfg.RedisURL` is passed via `adminservice.WithCoverEnqueuer(coversservice.NewEnqueuer(client))`, the same as `cmd/worker`, and closed on exit. Without it the save still succeeds and the sweep makes the copy (spec "Queue unavailable").
- **Logic:** `backend/internal/modules/harvest/service/cover_backfill.go`, `RunCoverBackfill(ctx, books CoverBackfillBooks, ids OpenLibraryIDSource, covers CoverByIDFinder, opts CoverBackfillOptions) (*CoverBackfillReport, error)`, with `Write(io.Writer)` like `CoverAuditReport`. The interfaces:
  - `CoverBackfillBooks`: `ListWikidataBooksWithoutCover`, `GetBook`, `UpdateBook` (adminservice.Service);
  - `OpenLibraryIDSource`: `FetchOpenLibraryIDs` (WikidataClient);
  - `CoverByIDFinder`: `FindCoverByOpenLibraryIDs` (OpenLibraryClient).
- **Wikidata:** new `WikidataClient.FetchOpenLibraryIDs(ctx, qids []string) (map[string][]string, error)` in `backend/internal/pkg/bookdata/wikidata_openlibrary_ids.go`. It takes 1..`MaxOpenLibraryIDBooks` (=50) QIDs, each validated with `IsWikidataQID`, otherwise `ErrInvalidQuery` without a call. Query: `SELECT ?item (GROUP_CONCAT(DISTINCT ?olid; separator="|") AS ?olids) WHERE { VALUES ?item { wd:Q… } ?item wdt:P648 ?olid . } GROUP BY ?item`. Keys are the QIDs; items without P648 are absent. It goes through `sparqlBindings` (shared limiter).
- **Candidates:** new admin repository method `ListWikidataBooksWithoutCover(ctx) ([]catalogDomain.Book, error)`: `external_source = 'wikidata' AND (cover_url IS NULL OR cover_url = '') AND status <> 'rejected'`, ordered by `created_at, id`, preloading Authors and Genres like `ListBooksWithOpenLibraryCover`. Exposed on `adminservice.Service`. Candidates whose `ExternalID` fails `IsWikidataQID` are counted as `skipped: invalid qid` in the report, not sent to Wikidata. Add the method to the three admin repository fakes (`service_test.go`, `merge_author_test.go`, `import_test.go`) and an SQLite repository test next to `book_openlibrary_cover_sqlite_test.go`.
- **Run:** batch the candidates by 50 → `FetchOpenLibraryIDs`. A batch error marks every book of the batch `error: wikidata: …`. For each candidate with IDs, call `FindCoverByOpenLibraryIDs`:
  - `ErrNotFound` → "no cover" line;
  - other error → `error:`;
  - success → finding with `usedID` and the cover link.
- **Apply:** for each finding, filtered by `-ids` when given (listed IDs that are not findings or candidates → `skipped: not a candidate` / `skipped: unknown id`):
  1. `GetBook` again; if `CoverURL` is now non-empty → `skipped: has cover`.
  2. Build the full `BookInput` from the re-read book, the same as `clearCover` in `cover_audit.go`, with `CoverURL: &link`, the ISBN kept as stored and `Status: nil`. Consider extracting a shared `bookInputFromStored(b)` helper used by both.
  3. `UpdateBook`; an error → `error: …` (for example, a stored description that fails validation); otherwise `updated`.
- **Output:** one line per finding, `found <id> <status> <qid> <olid> <cover> <title>`; per no-cover line, `nocover <id> <qid> <ids> <title>`; per error line, `error <id> <qid> <message>`; then `candidates=N with_ids=M found=K nocover=L errors=E`. With `-apply`, one line per write.
- **Dockerfile:** build and copy `olcoverbackfill` into the worker image the same way as `olcoveraudit`. Run: `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/olcoverbackfill worker [-apply]`; dev: `go run ./cmd/olcoverbackfill`.

### D6. Out of scope (explicit)

- **Admin book lookup** (`GET /admin/books/lookup`): no change. It has no Wikidata QID to start from.
- **CSV/JSON import:** no change.
- **Re-harvesting existing books:** stays create-only. The backfill is the only path for existing books.
- **Description from the OL work record:** not used. OL work descriptions are almost always English and would be dropped by the Russian filter anyway. This can be revisited separately.
- **Books with an `absent` copy** (a link with no image): not candidates, because they have a cover link.

## API

No HTTP changes.

## Database

No migration. The backfill is one read query plus normal `UpdateBook` saves that only set a cover link that was empty. No deletes, no bulk SQL updates, no `migrate down`.

## Existing Patterns

- `backend/cmd/olcoveraudit/main.go` + `backend/internal/modules/harvest/service/cover_audit.go` (+ its test): the CLI shape, report writer, interfaces and full-input `UpdateBook`.
- `backend/internal/pkg/bookdata/wikidata_adaptations.go` `FetchAdaptations`: the batched QID SPARQL lookup and its tests.
- `backend/internal/modules/harvest/service/handler_test.go` `fakeEnrichClient`: extend it with `FindCoverByOpenLibraryIDs` and record the call order (ISBN → IDs → title).
- `backend/cmd/worker/main.go` lines ~208-213: `WithCoverEnqueuer` wiring.

## Risks

- **Wrong-language covers.** An English (or other) edition cover can be shown for a Russian book. The owner accepted this.
- **Wrong P648 in Wikidata.** P648 is curated data and much safer than the title search. A wrong ID is a Wikidata data error, and an admin can fix the cover by hand. `olcoveraudit` will also list these covers as candidates (they are OL cover-ID links). Its `no_match` reason can then be a false positive for them, and the operator reviews before clearing.
- **SPARQL cost.** One more OPTIONAL multiplies the rows inside each group. `GROUP_CONCAT DISTINCT` keeps the output correct, and the limit stays at most 5000 works.
- **Request volume.** At most 3 extra Open Library requests per harvested work without a cover, and only when it has P648. The backfill makes about 100 Open Library requests and 14 Wikidata batches for the current 659 candidates, all through the ~1 req/s limiters.
- **Lost update on apply:** narrowed by re-reading each book right before its save. A concurrent admin edit within that sub-second window could still be overwritten with the stored values. This is acceptable for a one-off operator command.
- **Agents must not run `olcoverbackfill -apply`** against the shared dev database and must not start a second compose stack. Running it is the maintainer's step after merge.
