# Bookdata External Clients (`backend/internal/pkg/bookdata`)

Status: implemented

Backend-only. No database migration, no HTTP endpoint, no frontend change, no wiring into `backend/internal/modules/admin` or `backend/internal/modules/catalog` in this increment. This package ships as an internal library with no live caller yet — it is the foundation for two later, separately-planned increments: `cmd/harvest` (bulk CLI) and `GET /api/v1/admin/books/lookup` (admin single-lookup endpoint). See `docs/ai/features/catalog-data-import/business-analysis.md` for the full initiative context; this contract only covers the package itself.

## Goal

Provide Go HTTP clients for Wikidata (SPARQL) and Open Library (REST) that return a common, normalized `ExternalBookRecord`, so future callers don't need source-specific parsing logic, and so neither free public service gets hammered by tight-loop or unthrottled requests.

## User Stories

Internal/developer-facing only (no end user in this increment):

- As a future `cmd/harvest` CLI, I can run a Wikidata SPARQL query and get back a slice of normalized book records to feed into book creation.
- As a future admin lookup endpoint, I can look up a single book by ISBN or title+author against Open Library (and optionally Wikidata) and get back one normalized record.
- As a developer, I can unit-test code that depends on `bookdata` without making real network calls.

## Acceptance Criteria

- [x] `ExternalBookRecord` type exists with fields: `Title string`, `Authors []string`, `Genres []string`, `ISBN *string`, `Description *string`, `CoverURL *string`, `PublishedYear *int`, `PageCount *int`, `Language string`, `SourceName string`, `SourceID string` — field-compatible in spirit with `admin/service.BookInput` (same semantic fields; `Authors`/`Genres` are names here, not IDs, since name→ID resolution is a later increment's job).
- [x] `WikidataClient` can execute an arbitrary caller-supplied SPARQL query string against the configured endpoint and parse the SPARQL JSON results format into `[]ExternalBookRecord`.
- [x] `WikidataClient` supports a point lookup by title (+ optional author) built internally as a parameterized SPARQL query.
- [x] `OpenLibraryClient` supports point lookup by ISBN (Books API) and by title + author (Search API), returning a single best-match `ExternalBookRecord` or a not-found error.
- [x] `OpenLibraryClient` (or a package-level helper) can construct a Covers API URL from an ISBN/cover ID + size without a network call, matching the existing hotlink pattern already used in `backend/cmd/seed`.
- [x] Every outbound request from both clients sets a descriptive `User-Agent` header (project name, version, contact placeholder), overridable via constructor option.
- [x] Both clients throttle outbound requests via a shared rate limiter (default ~1 req/sec per client instance, configurable) and retry on HTTP 429 / 5xx with exponential backoff (bounded max attempts, honors `Retry-After` on 429 when present); non-retryable 4xx (400, 404, etc.) fail fast without retry.
- [x] All retry/rate-limit/User-Agent behavior is covered by unit tests using a fake `http.RoundTripper` — no real network calls in tests, no reliance on wall-clock sleep for correctness (sleep is injectable/stubbed in tests).
- [x] `go build ./...` and `go vet ./...` are clean; `go test ./internal/pkg/bookdata/...` passes.
- [x] Exported types/functions have doc comments (package-level doc comment included).
- [x] Nothing in the existing `admin` or `catalog` modules is modified; no migration is added; no route is registered.

## Business Rules

- Wikidata is the source of structured facts (title, authors, genre, published year); Open Library is the enrichment source (cover, description, page count). This package does not decide which source "wins" for a given field — that merging policy belongs to the future `cmd/harvest` increment. This package only returns what each source reports, one `ExternalBookRecord` per source call.
- Open Library must only be used for point lookups (single ISBN or single title+author query) from this package — no bulk crawling loops live here. Bulk retrieval is Wikidata's job (via SPARQL), consistent with Open Library's published etiquette (bulk belongs on their data dumps, not their live API).
- Google Books is explicitly out of scope for this package (and the whole initiative) — its ToS forbids persisting API results, which conflicts with the intended use. Do not add a Google Books client here.
- `ExternalBookRecord.ISBN` is normalized only structurally in this package (trim, strip spaces/dashes) — no checksum validation is performed here. Full ISBN-10/13 checksum validation is owned by `admin/service.isValidISBN` and applied later when a record is fed into `admin.Service.CreateBook`. Do not duplicate that validation in `bookdata` (avoids two independent implementations of the same business rule, matching NFR-001 in the business analysis).
- Retryable conditions: HTTP 429 and 5xx. Non-retryable: other 4xx (treated as immediate errors — e.g. 400 bad query, 404/empty-result treated as "not found"). Network/transport errors (timeouts, connection refused) are retryable the same as 5xx, up to the same max-attempts bound.
- Open Library's ISBN Books API returns HTTP 200 with an empty JSON object for an unknown ISBN (not a 404) — this must be explicitly mapped to `ErrNotFound`, not treated as a successful empty record.

## UX

None — no user-facing surface in this increment.

## API

None — no new or changed HTTP routes in this increment. (Future increment: `GET /api/v1/admin/books/lookup`, not part of this contract.)

## Database

None — no migration in this increment. (Future increment: nullable `external_source`/`external_id` on `books`, not part of this contract.)

## Frontend

None.

## Authorization

Not applicable — this is an internal Go package with no HTTP surface and no caller yet. No auth/role checks apply here; they belong to the future admin lookup endpoint increment, which must explicitly enforce `RoleAdmin` (reuse `middleware.Auth` + `RequireRole`, as flagged in the business analysis) when it is built.

## Package Design

Location: `backend/internal/pkg/bookdata/`.

Suggested file layout (developer may adjust names, not structure/responsibilities):

- `bookdata.go` — package doc comment, `ExternalBookRecord`, sentinel errors (`ErrNotFound`, `ErrRateLimited`, `ErrSourceUnavailable`, `ErrInvalidQuery`), the shared point-lookup interface:
  ```go
  type Client interface {
      SourceName() string
      FindByISBN(ctx context.Context, isbn string) (*ExternalBookRecord, error)
      FindByTitleAuthor(ctx context.Context, title, author string) (*ExternalBookRecord, error)
  }
  ```
  Both `WikidataClient` and `OpenLibraryClient` implement `Client`. `WikidataClient` additionally exposes a bulk method not on the common interface:
  ```go
  func (c *WikidataClient) Query(ctx context.Context, sparql string) ([]ExternalBookRecord, error)
  ```
- `transport.go` — shared, unexported HTTP plumbing used by both clients: a small config/options struct (`httpClient *http.Client`, `userAgent string`, `baseURL string`, rate limiter, `maxRetries int`, retry base/max delay, an injectable `sleep func(time.Duration)` defaulting to `time.Sleep`) and a `doRequest(ctx, req) (*http.Response, error)` helper implementing: set User-Agent → acquire rate limiter token (`limiter.Wait(ctx)`) → send → on 429/5xx retry with exponential backoff (honor `Retry-After` if present) up to `maxRetries` → return `ErrRateLimited`/`ErrSourceUnavailable` if attempts exhausted. Keep the rate limiter behind a small interface (e.g. `type limiter interface { Wait(ctx context.Context) error }`, satisfied by `*golang.org/x/time/rate.Limiter`) so tests can substitute a fast/no-op fake instead of a real timed limiter.
- `wikidata.go` — `NewWikidataClient(opts ...Option) *WikidataClient`, SPARQL query templates for title/author lookup, JSON result parsing (`results.bindings[]` → `ExternalBookRecord`, mapping `ru`-labeled values where available per the business analysis' language preference).
- `openlibrary.go` — `NewOpenLibraryClient(opts ...Option) *OpenLibraryClient`, Books API (`/api/books?bibkeys=ISBN:...&format=json&jscmd=data`) parsing, Search API (`/search.json?title=...&author=...`) parsing, and a pure helper `CoverURL(id string, idType CoverIDType, size CoverSize) string` (no network call) for the Covers API URL pattern already used in `cmd/seed`.
- `wikidata_test.go`, `openlibrary_test.go`, `transport_test.go` — unit tests (see Tests below).

Constructor pattern: functional options (`Option func(*config)`), e.g. `WithHTTPClient`, `WithUserAgent`, `WithBaseURL` (for pointing at a test server), `WithRateLimit(requestsPerSecond float64, burst int)`, `WithMaxRetries(n int)`. This keeps both clients testable without a real network and lets a later increment tune throttling without touching client internals.

`go.mod`: `golang.org/x/time` is already present as an **indirect** dependency (`v0.5.0`) — use `golang.org/x/time/rate` for the limiter and run `go mod tidy` to promote it to a direct requirement. No other new dependency is needed; stdlib `net/http` + `encoding/json` is sufficient for both SPARQL (plain HTTP GET with a query string, JSON response) and the Open Library REST endpoints.

### User-Agent

Default value should look like `BookHaven/0.1 (https://github.com/bookhaven; contact: <placeholder>)`, overridable via `WithUserAgent`. Flag explicitly in code comments that the placeholder contact must be replaced with a real, monitored contact (project URL or issue tracker — not a personal email) before this package is used against production traffic; do not hardcode any individual's personal email address into the User-Agent (this is a public, third-party-visible HTTP header, not an internal attribution field).

## Existing Patterns

- `backend/internal/modules/admin/service/service.go` — `BookInput` struct shape (field names/types this package's `ExternalBookRecord` should stay compatible with) and `isValidISBN`/`slugify` helpers (reference only — do not import; `bookdata` must not depend on `internal/modules/*` to avoid inverting the pkg→module layering, and must not re-implement checksum ISBN validation per NFR-001).
- `backend/internal/modules/admin/service/service_test.go` — repo-wide test convention: stdlib `testing`, table-driven tests, hand-rolled fakes (no mocking library, no testify — none is a dependency). Follow the same style for `bookdata`'s tests, substituting a fake `http.RoundTripper` for a fake repository.
- `backend/cmd/seed/main.go` — existing direct hotlink pattern to `covers.openlibrary.org` for cover URLs; `CoverURL` helper should produce URLs matching this same pattern.

## Tests

All tests are offline (fake `http.RoundTripper`, e.g. `type roundTripFunc func(*http.Request) (*http.Response, error)`), no `httptest` server required (acceptable either way, but no real network). Suggested cases:

- **transport_test.go**
  - Sets expected `User-Agent` header on every request.
  - Retries on 429 then succeeds on 2nd/3rd attempt; retries on 500/502/503 then succeeds; does not retry on 400/404 (fails immediately).
  - Exhausts `maxRetries` and returns `ErrRateLimited` (all 429s) / `ErrSourceUnavailable` (all 5xx) respectively.
  - Honors `Retry-After` header value when present on a 429 response (assert the injected sleep func was called with the expected duration, not a real wall-clock wait).
  - Rate limiter is consulted once per outbound attempt (fake limiter counts `Wait` calls); a real-limiter timing test is optional and, if included, must use small enough durations to stay fast and non-flaky.
- **wikidata_test.go**
  - `Query` parses a canned SPARQL JSON results fixture (multiple bindings, some with missing optional fields like ISBN/genre) into the expected `[]ExternalBookRecord`.
  - `FindByTitleAuthor` builds a request whose query string contains the (escaped) title/author and parses a canned single-result response into one `ExternalBookRecord` with `SourceName == "wikidata"`.
  - No results / malformed JSON → `ErrNotFound` / a wrapped parse error, respectively.
- **openlibrary_test.go**
  - `FindByISBN` parses a canned Books API JSON fixture into `ExternalBookRecord` with `SourceName == "openlibrary"`.
  - `FindByISBN` on an unknown ISBN (canned `{}` response body, HTTP 200) returns `ErrNotFound`.
  - `FindByTitleAuthor` parses a canned Search API JSON fixture (picks first/best match).
  - `CoverURL` produces the expected `covers.openlibrary.org` URL for a given ISBN and size, with no network call.

## Documentation

- Package-level doc comment on `bookdata.go` summarizing purpose, the two clients, and the etiquette requirements (User-Agent, throttling) so future increments (`cmd/harvest`, admin lookup endpoint) know how to use it without re-reading this contract.
- No `docs/ROADMAP.md` or `docs/features/admin.md` changes needed in this increment (this package isn't wired to anything user-facing yet).

## Risks

- **Placeholder contact info**: the default `User-Agent` contact value is a placeholder; if a later increment ships without replacing it, BookHaven risks violating the etiquette this package exists to satisfy. Mitigate with an explicit code comment/TODO and mention in the package doc comment.
- **Guessed default rate limits**: neither Wikidata nor Open Library publish a hard numeric rate limit for casual API use (beyond "be reasonable" / "don't bulk-crawl the live API"); the ~1 req/sec default is a reasonable but unverified starting point. Revisit if the later `cmd/harvest` increment observes 429s in practice.
- **SPARQL correctness is only unit-tested against canned fixtures**: parsing logic is verified, but the actual query templates' semantic correctness against the live Wikidata endpoint can't be verified without a real network call (out of scope for unit tests here). Flag for manual/exploratory verification when `cmd/harvest` is built.
- **Open Library response shape quirks**: the "not found" case returning HTTP 200 with an empty body (rather than 404) is easy to mis-handle; explicit test coverage above mitigates this.
- **No live caller yet**: since nothing in the codebase imports this package this increment, there's a risk of designing an interface that doesn't fit the eventual `cmd/harvest`/admin-lookup callers. Mitigated by keeping `ExternalBookRecord` field-compatible with `admin.BookInput` now, per the business analysis' explicit intent.
