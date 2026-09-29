# Open Library Works Description Enrichment

Status: implemented

Backend-only. Fixes a structural gap in `backend/internal/pkg/bookdata/openlibrary.go`: `FindByTitleAuthor` (the Open Library Search API path, `/search.json?title=...&author=...`) can never populate `ExternalBookRecord.Description`, because the Search API response it parses (`olSearchDoc`) has no description field at all — unlike `FindByISBN` (Books API), which already gets `Description` via `olBookData.Excerpts`. Since most Wikidata-harvested classics predate ISBN (pre-1970s), `FindByTitleAuthor` is the majority harvest-enrichment code path today, so harvested books are almost always created with no `Description`, not just occasionally. This is "Option A" from the business analysis in this session: give the search-match path a follow-up call to Open Library's Works API (`GET /works/{id}.json`), which reliably returns a work-level `description`, and wire that value onto the record — best-effort, only when actually needed.

## Scope boundaries (read first)

- **`PageCount` is explicitly out of scope.** It's an edition-level attribute (Works API has no page count; picking a specific edition from a work's editions list is a materially different, riskier feature). Do not attempt it here.
- **No change to `FindByISBN`** — it already gets `Description` correctly via the Books API's `Excerpts`.
- **No change to the `russian-only-content` Description-language filter** (`enrichRecord`'s `adminservice.IsRussianText` check in `backend/internal/modules/harvest/service/handler.go`) — this contract only gives that filter something to actually evaluate for search-matched records; today there's usually nothing there at all.
- **No change to Wikidata/SPARQL query building** (`wikidata.go`'s `parseSparqlBindings` correctly never sets `Description` — that's by design, Open Library is the enrichment source per `bookdata-clients.md`).
- **No change to search relevance/matching quality** (Option C in the business analysis) — this is purely about filling a field the current search path structurally can never populate, not about picking better search matches.
- **The admin lookup-assistant endpoint (`GET /admin/books/lookup`, see `docs/features/catalog-manual-import.md`) opts out.** That contract's Acceptance Criteria explicitly states lookup "makes exactly one outbound Open Library call per request" and is a synchronous, human-blocking action in the create-book form. Adding an unconditional second round-trip to every title/author lookup would silently violate that existing contract and slow down interactive admin UX for a field the lookup form doesn't currently promise reliably anyway. This contract does not touch `admin/service/lookup.go`, and the admin lookup client (`backend/cmd/api/main.go`'s `bookdata.NewOpenLibraryClient()`) must NOT opt into the new behavior. If the product owner later wants lookup-assistant description enrichment too, that's a follow-up, sized and scoped separately (flagged in Risks).

## Goal

When harvest enrichment matches a book via Open Library's Search API (the common case, since most harvested classics have no ISBN), also populate `Description` from the matched work's Works API record, so the `russian-only-content` description filter and the review queue actually have something to evaluate instead of `nil` almost every time.

## User Stories

- As a content admin reviewing a harvested book in the pending-review queue, I see a filled-in Description for books Open Library actually has a summary for, instead of an empty field for nearly every search-matched (non-ISBN) harvest result.

## Acceptance Criteria

**`backend/internal/pkg/bookdata/openlibrary.go` — new Works API method**
- [ ] Add `func (c *OpenLibraryClient) FetchWorkDescription(ctx context.Context, workKey string) (*string, error)` (naming/signature is the developer's call; follow this client's existing method-naming conventions). `workKey` is the bare key as already produced by `lastPathSegment(doc.Key)` (e.g. `"OL262461W"`), not the full `/works/...` path — mirrors how `SourceID` is already stored.
- [ ] Builds `GET {baseURL}/works/{workKey}.json` and sends it through `c.cfg.doRequest` — the same User-Agent/rate-limit/retry plumbing as every other method in this client. Do not build a separate HTTP path.
- [ ] Parses the response's `description` field, which Open Library inconsistently returns as either a plain JSON string or an object `{"type": "/type/text", "value": "..."}`. Handle both shapes (e.g. via `json.RawMessage` and a fallback unmarshal attempt — there is no existing reusable helper for this specific quirk; `mapOLBookData`'s `Description` handling reads from `olBookData.Excerpts`, a different Books-API-only shape, and is not reusable here).
- [ ] Returns `(nil, nil)` (not an error) when the work has no `description` field or it's an empty string, and returns `(nil, err)` (a non-nil error, no partial value) on a transport/parse/non-2xx failure — callers use these two "no description" outcomes identically (see next section) but the distinction matters for testability.
- [ ] Maps HTTP 404 to `ErrNotFound`, mirroring `FindByISBN`'s existing 404 handling convention; other 4xx to `ErrInvalidQuery`; 5xx/transport errors surface via `doRequest`'s existing `ErrSourceUnavailable`/`ErrRateLimited` (no new sentinel errors needed).

**Wiring into `FindByTitleAuthor` — opt-in via constructor option**
- [ ] Add a new unexported `config` field (`transport.go`) and an exported functional `Option`, e.g. `WithWorkDescriptionLookup()` (or `WithFetchWorkDescription(bool)` — developer's call), defaulting to **off**. This keeps the existing `Client` interface (`FindByISBN`/`FindByTitleAuthor`/`SourceName`) completely unchanged — no signature break, `WikidataClient` is unaffected, and each call site's `OpenLibraryClient` instance opts in or out independently at construction time, which is also the natural seam for sharing the same rate limiter/throttle (same `cfg`, same client instance — no separate HTTP path, no separate throttle to reason about).
- [ ] In `FindByTitleAuthor`, after `mapOLSearchDoc(parsed.Docs[0])` produces `record`: only when `c.cfg` has the option enabled, `record.Description == nil` (i.e. not already present — defensive, since `olSearchDoc` today has no description field so this is always true in practice, but keep the guard so this doesn't silently redo work if the doc shape ever changes), and `record.SourceID` is non-empty and looks like an Open Library **work** key (matches `^OL\d+W$`, e.g. `"OL262461W"` — not an edition key like `OL123M` or an author key like `OL1A`) — call `FetchWorkDescription(ctx, record.SourceID)` and, on success with a non-nil result, set `record.Description`.
- [ ] A `FetchWorkDescription` failure (any error, or a nil result) is **not fatal** to `FindByTitleAuthor` — the already-successfully-matched `record` (title, authors, cover, ISBN, etc.) is still returned as-is, with `Description` simply left `nil`. Matches the existing pattern where enrichment failures elsewhere in this pipeline (`handler.go`'s `enrichRecord`) are already non-fatal — log at most, never return an error from `FindByTitleAuthor` because of this follow-up call failing.

**Wiring into harvest (`backend/cmd/worker/main.go`)**
- [ ] The `bookdata.NewOpenLibraryClient()` construction used for harvest enrichment (currently `openLibraryClient := bookdata.NewOpenLibraryClient()`) passes the new option, e.g. `bookdata.NewOpenLibraryClient(bookdata.WithWorkDescriptionLookup())`. This is the only production call site that opts in.
- [ ] `backend/cmd/api/main.go`'s lookup-client construction (`bookdata.NewOpenLibraryClient()` passed to `adminservice.WithLookupClient`) is **unchanged** — does not opt in (see Scope boundaries).
- [ ] No change to `backend/internal/modules/harvest/service/handler.go`'s `enrichRecord`/`processRecord` — the existing `rec.Description = enrichedRec.Description` copy-then-Russian-filter logic already handles whatever `enrichedRec.Description` now contains; no new code needed there since the fix lives entirely inside the Open Library client.

**Throttle/performance tradeoff (explicit, not silent)**
- [ ] Document in code comments (on `FetchWorkDescription` and/or `WithWorkDescriptionLookup`) and in this contract's Risks that this adds one extra outbound request per search-matched, no-description book during harvest enrichment, sharing the same client instance's ~1 req/sec throttle as every other call — i.e. it roughly doubles the Open Library round-trips (and therefore wall-clock time spent on Open Library calls) for the common no-ISBN harvest case, since harvest already processes one record fully before starting the next (`docs/features/async-harvest-queue.md`, `harvest-progress.md`). This is an accepted, explicit tradeoff (harvest throughput for enrichment completeness), not a bug to fix later.

## Business Rules

- A work-key match is identified structurally (`^OL\d+W$` on `SourceID`, which is `lastPathSegment(doc.Key)`) — Open Library's `/search.json` `key` field is documented to return a work key (`/works/OL...W`) for the vast majority of results; if it ever doesn't match this shape (e.g. unexpected key type), the follow-up call is simply skipped, not attempted against a malformed URL.
- Best-effort only: a missing/failed work description never fails the overall lookup. The base record Wikidata/Open Library already successfully matched must always be returned.
- The `russian-only-content` Description filter (`enrichRecord` in harvest) is unchanged and still applies after this fix — a non-Russian work description is still silently dropped before `CreateBook`, exactly as today. This contract only increases how often there's something there for that filter to evaluate.
- Opt-in via constructor option, not a per-call parameter — matches this package's existing `Option func(*config)` pattern (`WithUserAgent`, `WithRateLimit`, etc. in `transport.go`) rather than adding a new interface method or changing `Client`'s shared signature.

## Database

None. No schema change.

## API

None. No new or changed HTTP routes/response shapes. `GET /admin/books/lookup` (`catalog-manual-import.md`) is unaffected — its client does not opt into this behavior (see Scope boundaries).

## Frontend

None. Harvested books' `Description` (already a field the review queue and book-form display today) will simply be populated more often for search-matched records; no UI change needed.

## Existing Patterns

- `backend/internal/pkg/bookdata/transport.go` — `Option func(*config)` pattern (`WithHTTPClient`, `WithUserAgent`, `WithRateLimit`, `WithMaxRetries`) — add the new option the same way, defaulting to off.
- `backend/internal/pkg/bookdata/openlibrary.go`'s `FindByISBN` — existing `doRequest` call shape, 404→`ErrNotFound` mapping, and `mapOLBookData`'s `Excerpts`-based `Description` assignment (`record.Description = &desc`) — same assignment idiom, different source field/shape.
- `backend/internal/modules/harvest/service/handler.go`'s `enrichRecord` — the established "enrichment failure is never fatal to the record" convention this contract's `FetchWorkDescription` failure-handling must match.
- `backend/cmd/worker/main.go` / `backend/cmd/api/main.go` — the existing pattern of two independently-constructed `bookdata.NewOpenLibraryClient()` instances for two different call sites (harvest vs. admin lookup) — this is exactly the seam the new option uses to differ per call site without touching shared code.

## Tests

- `backend/internal/pkg/bookdata/openlibrary_test.go`:
  - `FetchWorkDescription` parses a canned Works API fixture with a plain-string `description` → returns the string.
  - `FetchWorkDescription` parses a canned fixture with an object-shaped `description` (`{"type":"/type/text","value":"..."}`) → returns `.value`.
  - `FetchWorkDescription` on a fixture with no `description` field → returns `(nil, nil)`.
  - `FetchWorkDescription` on HTTP 404 → returns `ErrNotFound`.
  - `FindByTitleAuthor` with `WithWorkDescriptionLookup()` enabled and a fixture whose `key` is a work key → asserts a second request is made to `/works/{id}.json` and the returned record's `Description` is populated from it.
  - `FindByTitleAuthor` with the option **not** set (default) → asserts no second request is made (existing single-request behavior/fixture tests must keep passing unmodified — this is the regression guard that the admin lookup path's "exactly one outbound call" behavior is preserved by default).
  - `FindByTitleAuthor` with the option enabled but `FetchWorkDescription` failing (e.g. fixture responder returns 500 for the works call) → `FindByTitleAuthor` still succeeds, returns the base record with `Description == nil`.
- `backend/internal/modules/harvest/service/handler_test.go`: extend an existing `enrichRecord`/`processRecord` fake-client test to confirm a `Description` returned by the fake `FindByTitleAuthor` still flows through the existing Russian-filter logic unchanged (this is a regression check, not new behavior — the fake client can just return a non-nil `Description` directly, no need to fake the two-step Works API call inside the harvest test since that's fully covered at the `bookdata` package level above).
- `go build ./...`, `go vet ./...`, `go test ./backend/...` (or `go test ./...` from `backend/`) clean.

## Documentation

- Mark this file `Status: implemented` when done.
- Doc comments on `FetchWorkDescription` and `WithWorkDescriptionLookup` explaining: the plain-string-vs-object description-shape quirk, the "opt-in per client instance" design and why (admin lookup vs. harvest), and the throttle/performance tradeoff.
- No `docs/ROADMAP.md` entry required (consistent with how prior harvest-pipeline increments in this series were not separately tracked there, per `russian-only-content.md` precedent).

## Risks

- **Roughly doubles Open Library round-trip count (and wall-clock time) for harvest's common no-ISBN, search-matched case.** Accepted tradeoff, stated explicitly above — not a bug. Revisit only if harvest job duration becomes a real operational problem (the option can be turned off, or a future increment could parallelize/batch it, without changing this contract's core design).
- **Key-shape heuristic (`^OL\d+W$`) could occasionally skip a legitimate work whose key doesn't match** if Open Library's Search API ever returns a non-work key in `doc.key` for some result types. Low risk (documented Open Library behavior is work-key-by-default for `/search.json`), and the failure mode is simply "no extra description fetched," not incorrect data.
- **Admin lookup-assistant still never gets a Description via the search path**, since it deliberately doesn't opt in (see Scope boundaries). This is an intentional scope cut, not an oversight — flagged here as a follow-up if the product owner later wants lookup-assistant parity, which would need its own UX tradeoff discussion (an extra round-trip on a synchronous, human-blocking form action) rather than inheriting this contract's harvest-oriented tradeoff by default.
- **Open Library's Works API response shape is only unit-tested against canned fixtures** (both description shapes), matching this package's existing testing convention (`bookdata-clients.md`'s Risks already flags this same limitation for SPARQL correctness) — semantic correctness against the live endpoint isn't verified by unit tests; flag for manual/exploratory verification if harvested descriptions look wrong in practice.
