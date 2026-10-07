# Harvest Author Search

Status: implemented

This is the next increment on top of `docs/features/admin-harvest-trigger.md` (implemented: `POST /api/v1/admin/harvest` takes `author_qids: string[]` + optional `limit`; the admin `/harvest` page has a raw QID textarea) and `docs/features/harvest-progress.md` (implemented: live progress bar, not touched here). Today, to trigger a harvest the admin must already know the Wikidata QID of every author (e.g. `Q7200`) and type it by hand. This contract adds a name-based resolution step in front of that unchanged trigger: the admin types a Russian (or any) name, sees candidate Wikidata people to disambiguate, picks the right one(s), and only the resulting QIDs are submitted to the existing `POST /api/v1/admin/harvest`.

## Scope boundaries (read first)

**Everything from the two prior contracts stays exactly as decided — not reopened here:**
- `POST /api/v1/admin/harvest`'s request shape (`{"author_qids": string[], "limit"?: number}`), its `400`/`502`/`201` behavior, `bookdata.BuildAuthorListQuery`, the async enqueue path, the worker/job pipeline, and the jobs table/progress bar are all unchanged. This contract is purely a new *read-only* resolution step that runs before that endpoint is called — it produces QIDs, nothing else.
- Curated-QID-only input to the harvest pipeline itself — no raw SPARQL, no free-text author names reaching the worker. Name search only ever feeds back a QID that flows through the exact same validated path (`bookdata.BuildAuthorListQuery`) as today.
- `backend/internal/pkg/bookdata/wikidata.go`'s existing SPARQL-based `Query`/`FindByISBN`/`FindByTitleAuthor`/`BuildAuthorListQuery` are unchanged. This feature adds a **new method** alongside them, using a **different** Wikidata HTTP API (see below) — it does not touch or reuse the SPARQL query builders.

**Explicitly out of scope (do not build):**
- Any change to `POST /api/v1/admin/harvest`'s request/response shape.
- Any change to `bookdata.BuildAuthorListQuery`, the worker, or job status endpoints/UI.
- Silent auto-pick-first-match. **Product decision, not up for reconsideration**: a Russian surname search is often ambiguous (e.g. "Толстой" → Lev Tolstoy `Q7200`, Alexey N. Tolstoy `Q171976`, Alexey K. Tolstoy `Q368757`, and others); auto-picking risks silently harvesting the wrong bibliography. The admin must see candidates and explicitly pick.
- Bulk/batch name-file upload (the `-authors-file` CLI equivalent) — flagged as a possible follow-up, not built here.
- Fuzzy/typo-tolerant matching beyond whatever Wikidata's own `wbsearchentities` API already does natively — no client-side spell-correction.
- A dedicated "is this Wikidata item actually a human (`wdt:P31 wd:Q5`)" server-side filter. **Decision (see Business Rules for the full rationale): not built in this increment.** `wbsearchentities` is typed to `type=item` (not restricted to humans), and this contract relies on (a) that type filter, (b) the returned `description` being shown to the admin (Wikidata descriptions for people are almost always occupation-shaped, e.g. "русский писатель", making non-people candidates easy to spot), and (c) the mandatory confirmation step itself. Flagged as a possible follow-up (a batched `VALUES`-based SPARQL `P31`/`Q5` check against the returned candidate QIDs), not built here — see Risks.

## User Stories

- As a content admin, I can type an author's name (e.g. "Толстой") into the harvest page and see a list of matching Wikidata people (name + short description) to choose from, instead of having to already know their QID.
- As a content admin, when a name is ambiguous, I can see enough context (description) to pick the correct person, and I explicitly pick — the system never silently guesses.
- As a content admin, I can search and pick several different authors in one session before submitting a single harvest job, matching today's multi-QID capability.
- As a content admin who already knows a QID (e.g. from a previous session, or Wikidata directly), I can still paste it in directly without being forced through search.

## Acceptance Criteria

**Backend — Wikidata name-search client**
- [x] `backend/internal/pkg/bookdata/wikidata.go` (or a new file in the same package, e.g. `wikidata_search.go` — developer's call, minimize churn): new exported type
  ```go
  type AuthorCandidate struct {
      QID         string
      Label       string
      Description string // may be empty; Wikidata doesn't guarantee one
  }
  ```
- [x] New method `func (c *WikidataClient) SearchAuthors(ctx context.Context, query string) ([]AuthorCandidate, error)`, calling Wikidata's `wbsearchentities` action API (`https://www.wikidata.org/w/api.php?action=wbsearchentities&search=<query>&language=ru&uselang=ru&type=item&format=json&limit=10`) — **not** the SPARQL endpoint (`query.wikidata.org`) used by every other method in this file; `wbsearchentities` is a plain JSON search API, no SPARQL involved. `limit=10` is a fixed constant (e.g. `authorSearchLimit = 10`, not caller-configurable in this increment — Wikidata's own default is 7, max 50; 10 is a reasonable MVP balance between "enough to disambiguate" and "not an overwhelming list").
- [x] `query` is trimmed; empty → `ErrInvalidQuery` (reuse the existing sentinel), no HTTP call made.
- [x] Because this hits a different host/path than the SPARQL endpoint, `config` gains a second, independently-overridable base URL: a new field `searchBaseURL string` (defaulted in `NewWikidataClient` to a new constant `defaultWikidataSearchBaseURL = "https://www.wikidata.org/w/api.php"`, mirroring how `baseURL`/`defaultWikidataBaseURL` already work) and a new `Option`, `WithSearchBaseURL(url string) Option` (in `transport.go`, alongside `WithBaseURL`), so tests can point it at a local `httptest` server exactly like `WithBaseURL` already does for the SPARQL endpoint. `WithBaseURL` itself is unchanged (still only affects the SPARQL endpoint).
- [x] `SearchAuthors` reuses `cfg.doRequest` for the actual call — same `User-Agent`, same shared rate limiter (~1 req/sec per client instance, per `docs/features/bookdata-clients.md`), same 429/5xx retry-with-backoff. No bespoke HTTP client logic. A `400` from Wikidata → `ErrInvalidQuery`; other `>= 400` after retries are exhausted → whatever `doRequest` already returns (`ErrRateLimited`/`ErrSourceUnavailable`), unchanged pass-through.
- [x] Response parsing: `wbsearchentities` returns `{"searchinfo": {...}, "search": [{"id": "Q7200", "label": "Лев Толстой", "description": "русский писатель", ...}], "success": 1}` on a normal call, and `{"search": [], "success": 1}` (HTTP 200) when there are zero matches — **zero matches is not an error**, `SearchAuthors` returns `(nil or empty slice, nil)` in that case, it does not return `ErrNotFound` (unlike `FindByISBN`/`FindByTitleAuthor` — those model a single point-lookup with a "not found" concept; this is a list/search endpoint where "no results" is a normal, displayable state, same shape decision `wbsearchentities` itself makes). Map `search[].id` → `QID`, `search[].label` → `Label` (fall back to `search[].match.text` only if `label` is empty — Wikidata omits `label` for items with no label in the requested language but does provide the raw matched text), `search[].description` → `Description` (leave empty if absent, do not error).
- [x] Malformed/unparseable JSON body → wrapped error (`fmt.Errorf("bookdata: parse wikidata search response: %w", err)`), not a panic.
- [x] Doc comment on `SearchAuthors` explicitly notes: this method does not filter results to confirmed humans (`wdt:P31 wd:Q5`) — see the package-level and `harvest-author-search.md` rationale — callers/UI must treat every returned item as an unconfirmed candidate the admin must visually vet via `Description`.

**Backend — admin endpoint**
- [x] New interface in `backend/internal/modules/harvest/transport/http/handler.go` (mirrors this package's existing `TaskEnqueuer`/`TaskInspector` "handler depends on a small interface" convention, not the concrete `*bookdata.WikidataClient`):
  ```go
  type AuthorSearcher interface {
      SearchAuthors(ctx context.Context, query string) ([]bookdata.AuthorCandidate, error)
  }
  ```
- [x] `NewHandler` signature gains a third parameter: `func NewHandler(enqueuer TaskEnqueuer, inspector TaskInspector, searcher AuthorSearcher) *Handler`. `Handler` struct gains `searcher AuthorSearcher`. Update the one production call site (`cmd/api/main.go`, via `RegisterAdminRoutes`) and all existing test call sites in `handler_test.go`.
- [x] New handler method `SearchAuthors(c echo.Context) error`, reading `q` from the query string (same param name as `GET /admin/books/lookup`'s `q`, for naming consistency). Trims; empty → `400 {"error": "q is required"}` (inline check in the handler — this package has no separate `service.go`/service layer of its own today, matching how `TriggerHarvest` itself validates inline rather than delegating to a service; do not introduce a new service layer just for this one passthrough call).
- [x] Calls `h.searcher.SearchAuthors(ctx, q)`. Error mapping (mirrors `admin.LookupBook`'s existing convention, reused for consistency across the codebase's two admin-lookup-style endpoints):
  - `errors.Is(err, bookdata.ErrInvalidQuery)` → `400` with the error message.
  - `errors.Is(err, bookdata.ErrRateLimited)` or `errors.Is(err, bookdata.ErrSourceUnavailable)` → `502 {"error": "search source is unavailable; retry later or enter the QID manually"}` (note: unlike book lookup, this endpoint always has a manual-entry escape hatch in the UI — the message should point the admin at it).
  - anything else → `500 {"error": "failed to search authors"}`.
- [x] Success → `200 {"items": [{"qid": "...", "label": "...", "description": "..."}]}` (empty `query` param already rejected above; zero matches → `200 {"items": []}`, not an error — see client-layer note). Response DTO:
  ```go
  type authorCandidateResponse struct {
      QID         string `json:"qid"`
      Label       string `json:"label"`
      Description string `json:"description,omitempty"`
  }
  ```
- [x] Route: `harvest.GET("/authors/search", harvestHandler.SearchAuthors)` inside the existing `admin.Group("/harvest", adminRole)` group in `backend/internal/transport/http/router.go` — same `adminRole` gate as every other harvest route, **no** `requireBearer` (read-only `GET`, matching `harvest.GET("/jobs", ...)` and `admin/books.GET("/lookup", ...)`'s existing convention of cookie-or-bearer read auth for `GET`s). Moderators get `403`, same as every other admin route.
- [x] `RegisterAdminRoutes` gains a new parameter `harvestAuthorSearcher harvesthttp.AuthorSearcher` (developer's call on exact position/whether to fold into an options struct — minimize diff, consistent with how `harvestEnqueuer`/`harvestInspector` were already added as plain params by the prior contract). `harvesthttp.NewHandler(harvestEnqueuer, harvestInspector, harvestAuthorSearcher)` updated accordingly.
- [x] `backend/cmd/api/main.go` constructs `wikidataClient := bookdata.NewWikidataClient()` (zero-config, mirroring the existing `bookdata.NewOpenLibraryClient()` construction two lines above it and `cmd/worker/main.go`'s own dual construction of both clients — same accepted placeholder-`User-Agent` caveat documented in `docs/features/bookdata-clients.md`, not re-litigated here) and passes it into `RegisterAdminRoutes` as the new `AuthorSearcher` argument (it satisfies the interface directly, no adapter needed).
- [x] Swaggo doc comments on `SearchAuthors`, matching the existing convention (`@Summary`, `@Tags admin`, `@Param q query string true "..."`, `@Success 200 {object} listAuthorCandidatesResponse`, `@Failure 400/401/403/502/500`, `@Router /admin/harvest/authors/search [get]` — see `LookupBook` in `admin/transport/http/handler.go` for the exact shape to mirror).

**Frontend (`apps/admin`)**
- [x] `packages/api-client/src/index.ts` gains:
  ```ts
  export interface AdminHarvestAuthorCandidate {
    qid: string;
    label: string;
    description?: string;
  }
  ```
  and `api.admin.harvest.authors = { search: (q: string) => Promise<{ items: AdminHarvestAuthorCandidate[] }> }` (fetcher `GET /api/v1/admin/harvest/authors/search?q=<encoded q>`), added alongside the existing `api.admin.harvest.trigger`/`jobs` object, following the existing `admin.books.lookup`-style single-param-GET fetcher shape exactly.
- [x] `apps/admin/src/entities/harvest/api/index.ts` gains `searchAuthors: api.admin.harvest.authors.search` on the existing `adminHarvestApi` export (thin re-export, unchanged pattern).
- [x] `apps/admin/src/views/harvest/ui/harvest-view.tsx`'s form input UX changes from a single QID textarea to a two-mode form (developer's call on exact component boundaries; extracting the new name-search UI into a sibling file, e.g. `apps/admin/src/views/harvest/ui/author-name-search.tsx`, is recommended to keep `harvest-view.tsx` from growing unmanageably, but not mandated):
  - A mode toggle (e.g. two `Button`s styled as a segmented control, or a simple `Label` + checkbox — developer's call, no new `packages/ui` component required for this) between **"Поиск по имени"** (default) and **"Ввести QID вручную"** (advanced/escape hatch).
  - **Name-search mode (default):**
    - A single-line `Input` for one name at a time + a "Найти" `Button` (submits on Enter too). Calls `adminHarvestApi.searchAuthors(name)` via `useMutation` (mirrors the existing `triggerMutation` pattern — not `useQuery`, since this is an explicit user-triggered action, not cache-key-driven data).
    - Renders the returned candidates as a plain list (reuse `Card`/`Button`/`Badge` from `@bookhaven/ui`, no new shared UI component needed for an MVP list of ≤10 items) below the input: each candidate shows `label` + `description` (or "без описания" if empty) and a "Добавить" button. Zero results → "Совпадений не найдено. Проверьте написание или используйте ручной ввод QID." (pointing at the escape hatch). A `400`/`502` error surfaces via the same `ApiError`-message pattern already used for the trigger mutation's `onError`.
    - Clicking "Добавить" on a candidate adds `{qid, label}` to an accumulated `selectedAuthors` list (deduped by `qid`; clicking an already-added candidate is a no-op or shows "Добавлено" — developer's call), rendered as a removable list/chips (each with a small "×"/remove `Button`) above the submit button — this is the list that becomes `author_qids` on submit. The name `Input` and results list clear (or stay, developer's call) after adding, ready for the next name.
  - **Advanced/QID mode:** unchanged existing `Textarea` + `parseAuthorQIDs` behavior from today, feeding the same `author_qids` array directly (bypasses `selectedAuthors` entirely for that submission — the two modes are mutually exclusive per submit, not merged, since merging silently would be surprising; a mode switch discards the other mode's in-progress input, developer's call whether to warn or just silently reset).
  - The `limit` field and submit button are unchanged, positioned below whichever mode's input is active.
  - Client-side validation before calling `triggerMutation`: name-search mode requires at least one entry in `selectedAuthors`; QID mode requires at least one parsed QID (unchanged existing check) — same local `error`-state pattern as today ("Выберите хотя бы одного автора." / "Введите хотя бы один QID автора.").
- [x] No other part of `harvest-view.tsx` changes — the jobs table, polling, progress bar, and the final `triggerMutation.mutate({ author_qids, limit })` call are unchanged; this contract only changes how `author_qids` gets populated before that call.

## Business Rules

- **No silent auto-pick.** Confirmed product decision (see Scope boundaries) — the admin always sees candidates and explicitly clicks to add each one to the harvest set. There is no "if exactly one result, auto-select it" shortcut in this increment (flagged as a possible low-risk follow-up, not built here, since even a single match can occasionally be the wrong item, e.g. a namesake).
- **No human-only (`P31`/`Q5`) server-side filter in this increment.** Wikidata's `wbsearchentities` API filters by `type=item` only (there is no `type=human` option), and adding a follow-up SPARQL check would (a) double the outbound Wikidata calls per search, (b) add latency to what should stay a fast, synchronous admin-facing lookup, and (c) still not fully resolve ambiguity between multiple humans sharing a name — the actual disambiguation mechanism is, and remains, the admin reading `description` and choosing. This is judged sufficient for MVP; see Risks for the revisit trigger.
- Zero search results is a normal `200` response (`items: []`), not an error — the UI's job is to guide the admin to the manual-QID escape hatch in that case, not to treat it as a failure.
- The two input modes (name-search, manual QID) are mutually exclusive per form submission — `author_qids` sent to `POST /admin/harvest` comes from exactly one of `selectedAuthors` (mapped to QIDs) or the parsed QID textarea, never a merge of both, to keep the mental model simple ("this is how I built my list this time").
- Authorization: `GET /admin/harvest/authors/search` requires `RoleAdmin` (moderators get `403`) and the existing cookie-or-bearer read-auth convention, identical to `GET /admin/harvest/jobs` and `GET /admin/books/lookup` — no bearer-only requirement, since this is read-only and never mutates state.
- This endpoint performs a live, synchronous outbound call to Wikidata (unlike `POST /admin/harvest`, which only enqueues) — same category of synchronous external call as the already-shipped `GET /admin/books/lookup` (Open Library), not a new risk pattern for this codebase; bounded by the same ~1 req/sec per-instance rate limiter and 15s HTTP timeout as every other `bookdata` client call.

## UX

- Default flow on `/harvest`: type a name → "Найти" → see candidate cards (name + description) → "Добавить" the correct one(s) → repeat for more authors → optionally set `limit` → "Запустить харвест" (unchanged submit button/label).
- Selected-authors list sits between the search UI and the limit/submit controls, so the admin can see and edit (remove) their accumulated picks before submitting, mirroring how the existing QID textarea already let an admin see/edit their whole list before submitting.
- A visible, low-friction toggle to "Ввести QID вручную" — this is not buried in a settings menu; it's a normal alternate input mode for admins who already know the QID (e.g. re-running a harvest for an author they just handled), avoiding forcing a search round-trip on them.
- No change to the jobs table, polling, or progress bar below the form.

## API

`GET /api/v1/admin/harvest/authors/search?q=<name>`
- `200`: `{"items": [{"qid": "Q7200", "label": "Лев Толстой", "description": "русский писатель"}, ...]}` (`items: []` for zero matches — not an error).
- `400`: missing/empty `q`.
- `401`/`403`: auth/role failures, standard shape.
- `502`: Wikidata rate-limited or unavailable after retries.
- `500`: unexpected failure.

No changes to `POST /admin/harvest`, `GET /admin/harvest/jobs`, or `GET /admin/harvest/jobs/:id`.

## Database

None. This is a stateless passthrough to Wikidata's search API — no caching table, no persistence of searches or candidates.

## Frontend

- `packages/api-client/src/index.ts` — new `AdminHarvestAuthorCandidate` type + `api.admin.harvest.authors.search`.
- `apps/admin/src/entities/harvest/api/index.ts` — `searchAuthors` added to `adminHarvestApi` (existing file, one-line addition).
- `apps/admin/src/views/harvest/ui/harvest-view.tsx` — input section replaced with the two-mode (search / manual QID) UX; everything below (jobs table, polling, progress) unchanged.
- `apps/admin/src/views/harvest/ui/author-name-search.tsx` — new (recommended extraction, not mandated), holds the search-input + candidate-list + selected-authors-accumulator UI.

## Existing Patterns

- `backend/internal/pkg/bookdata/wikidata.go` — `WikidataClient`, `cfg.doRequest` reuse, `ErrInvalidQuery`/`ErrRateLimited`/`ErrSourceUnavailable` sentinels — mirror exactly for `SearchAuthors`; do not build a second HTTP client or reinvent retry/throttle logic.
- `backend/internal/pkg/bookdata/transport.go` — `WithBaseURL`/`defaultWikidataBaseURL` — mirror for the new `WithSearchBaseURL`/`defaultWikidataSearchBaseURL` pair.
- `backend/internal/modules/admin/service/lookup.go` + `admin/transport/http/handler.go`'s `LookupBook` — the existing read-only admin-lookup precedent (query-param-driven, `400`/`502`/`500` mapping, no persistence) — mirror the *shape and conventions* (not the code — this feature's handler talks to the `bookdata` client directly with no separate service layer, matching how `TriggerHarvest` itself already does inline validation in this specific package).
- `backend/internal/modules/harvest/transport/http/handler.go` — `TaskEnqueuer`/`TaskInspector` "handler depends on a small interface" shape — mirror for the new `AuthorSearcher` interface.
- `backend/internal/transport/http/router.go` — `RegisterAdminRoutes`'s existing `harvest` group wiring — add the new route the same way.
- `apps/admin/src/views/harvest/ui/harvest-view.tsx` — `triggerMutation`/`error`-state pattern, `parseAuthorQIDs` (kept, reused unchanged for the manual-QID mode) — mirror for the new search mutation and selection-accumulator state.
- `packages/api-client/src/index.ts` — `admin.books.lookup`'s single-query-param GET fetcher shape — mirror for `admin.harvest.authors.search`.

## Tests

- Backend, `backend/internal/pkg/bookdata/wikidata_test.go` (extend): fake `http.RoundTripper`/`httptest` server pointed at via `WithSearchBaseURL` — cover: a normal multi-result response parses into the expected `[]AuthorCandidate`; a zero-result response (`{"search": [], "success": 1}`) returns an empty slice, no error; empty `query` → `ErrInvalidQuery`, no HTTP call made; a `400` response → `ErrInvalidQuery`; malformed JSON body → a wrapped error, not a panic; confirm `User-Agent`/rate-limiter/retry reuse via the same assertions style already used for `Query`/`FindByISBN` in this file (no new retry-logic test needed — it's the same `doRequest`, just confirm it's actually called).
- Backend, `backend/internal/modules/harvest/transport/http/handler_test.go` (extend): hand-rolled fake `AuthorSearcher`. Cover: valid `q` → `200` with mapped `items`; missing/empty `q` → `400`; fake returning `bookdata.ErrInvalidQuery` → `400`; fake returning `bookdata.ErrRateLimited`/`ErrSourceUnavailable` → `502`; fake returning an unmapped error → `500`; update every existing `NewHandler(...)` call site in this file for the new third parameter.
- `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`.
- Frontend, `apps/admin/src/views/harvest/ui/harvest-view.test.tsx` (extend — check its current contents first per the prior contract's own note): mode toggle switches the visible input; name-search mode — typing a name and searching renders returned candidates (label + description), "Добавить" adds to the selected list and it's reflected in what's ultimately passed to `triggerMutation`, zero results shows the empty-state message, a `400`/`502` `ApiError` surfaces its message; QID mode — existing `parseAuthorQIDs` coverage from `admin-harvest-trigger.md` still passes unchanged; submit validation blocks with no selection/QIDs in either mode.
- Admin app type-check/lint clean; no requirement to run the full monorepo test suite (per `docs/features/admin.md`'s existing stated bar).

## Documentation

- Mark this file `Status: implemented` when done.
- `docs/features/admin-harvest-trigger.md`: add a one-line pointer near its form/UX Acceptance Criteria noting that `harvest-author-search.md` added a name-search resolution step in front of the QID input (default mode), with the original QID textarea preserved as an "advanced" escape hatch — one or two lines, matching the existing cross-contract pointer convention.
- `docs/ai/features/catalog-data-import/business-analysis.md`: no change required — this contract doesn't alter source selection (Wikidata remains the structured-facts source) or the CLI-vs-admin-trigger decision already superseded by `admin-harvest-trigger.md`; it only adds a resolution step ahead of the existing curated-QID harvest trigger.

## Risks

- **No human-only filter means a non-person Wikidata item (e.g. a book, a disambiguation page, an organization sharing the searched name) can appear in results.** Mitigated only by the admin reading `description` before clicking "Добавить" — an inattentive admin could pick a wrong item, and unlike a wrong-person QID (which just harvests the wrong bibliography), a non-person QID would make `BuildAuthorListQuery`'s `wdt:P50` (author) join simply return zero records for that QID (harmless — a wasted pick, not corrupt data) since non-people generally aren't the `wdt:P50` value of any work. Low real-world impact; revisit with a batched SPARQL `P31`/`Q5` check (single extra query per search, not per candidate) if this proves a recurring confusion in practice.
- **Two independent Wikidata client instances now exist in `cmd/api`'s process** (the pre-existing `OpenLibraryClient` for book lookup, and this contract's new `WikidataClient` instance) — each with its own independent ~1 req/sec rate limiter; they don't share a budget, but they also don't hit the same host (Wikidata vs. Open Library) except for this new client and any future Wikidata caller in the same process, so no double-counting risk today. Worth knowing if a future increment adds a second concurrent Wikidata caller in `cmd/api`.
- **`wbsearchentities` result quality for short/very common Russian surnames is unverified in this planning pass** (no live network access here) — e.g. whether `language=ru` search matches are ranked sensibly, whether transliterated/Latin input also matches. Accepted as an MVP risk; if search quality proves poor in practice, the manual-QID escape hatch remains available as a fallback, and this is a tuning problem (parameters like `language`/`uselang`, or trying multiple language variants) rather than an architectural one.
- **The manual-QID escape hatch reintroduces the original risk this feature exists to reduce** (an admin pasting a wrong/mistyped QID with no name-based sanity check) — accepted, since it's opt-in, low-frequency (power users who already know the QID), and was the *entire* existing UX until this contract shipped, so it's not a regression, just an unremoved prior risk.

## Later changes

- [author-direct-import.md](author-direct-import.md) (epic "Author moderation & import", feature 3): each search item now carries `existing: {id, name, slug, status} | null` — the BookHaven author already holding that QID (QID match only, one `IN` query per search via `ExistingAuthorFinder`, 4th `harvesthttp.NewHandler` param). If that lookup fails, the search still returns 200 with `existing: null` (error logged). The search widget was extracted to `apps/admin/src/features/search-wikidata-author` (`useWikidataAuthorSearch` + `WikidataAuthorSearch`); `/harvest` keeps its "Добавить"/"Добавлено" flow, chips and manual QID mode, and now also shows the QID link and the existing-author badge with an "Открыть" link.
