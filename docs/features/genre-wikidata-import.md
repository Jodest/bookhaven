# Genre Import from Wikidata

Status: done

Issue: #303. Epic #290 "Genre moderation and Wikidata import", feature 3 of 3 (the last one; phase 7 of [admin-navigation-restructure.md](admin-navigation-restructure.md)). Builds on [genre-status.md](genre-status.md) (#301: `status`, `wikidata_qid`, `resolveHarvestGenre`, `GenreResolution`) and [genre-moderation-queue.md](genre-moderation-queue.md) (#302: alias rows via `merged_into_id`, `followGenreAlias`, `GenreResolution.ViaAlias`, `genreModerationHref`). Modeled on [author-direct-import.md](author-direct-import.md). Those docs are the source of truth and are not reopened here.

## Goal

Let an admin bring a specific literary genre into BookHaven from Wikidata without waiting for a harvest. The admin searches by name on `/import/genres`, sees only literary-genre candidates with an "already in BookHaven" marker, and imports one. The import uses the harvest genre rules exactly (QID first, aliases followed, rejected tombstones respected, slug match backfills the QID), so it never duplicates a genre and never revives a rejected one. A created genre is `pending` with its QID and appears in `/moderation/genres`.

## Out of scope

- Bulk import (one candidate per click).
- Importing books of a genre (no "import with books"; genre harvest does not exist).
- Any change to harvest, file import, `POST /admin/harvest` or the author search/import.
- Slug/name markers in the search list. Only QID matches are marked; slug matching happens at import and is explained in the result.
- Editing or publishing from the import page (moderation stays in `/moderation/genres`).
- A migration (see Database).

## User Stories

- As an admin, I open Импорт → Жанры, type "фэнтези" and see Wikidata candidates that are literary genres, each with its Russian name, description, a Wikidata link and, if the QID is already in BookHaven, the genre's status (and the merged alias, if the QID belongs to one).
- As an admin, I click "Импортировать" and get a pending genre with the QID and the Russian Wikidata name, with links to the moderation queue.
- As an admin, if the genre already exists (by QID, through a merged alias, or by name), nothing is duplicated, and I am told which genre matched and how.
- As an admin, if the genre was rejected earlier (directly, or its alias points to a rejected genre), the import is refused with a clear message and a link.
- As an admin, if the Wikidata item has no Russian name or is not a literary genre, the import is refused with a clear reason.

## Business Rules

1. **Literary-genre check (bounded).** *Amended during implementation after the mandated live verification; see Implementation Notes for the evidence. The planned P136-usage branch and the root `Q12308638` were invalid.* Constants in `bookdata`, every QID verified against live Wikidata:
   - `LiteraryGenreRoots` = `wd:Q223393` (literary genre) and `wd:Q4263830` (literary form). `Q12308638`, which the plan proposed for "literary form", is actually "poetry collection". **`wd:Q483394` (genre) is NOT a root**: walking `P279*` up to "genre" accepts music and film genres.
   - `fictionGenreClass` = `wd:Q108465955` (fiction genre / "genre of fiction"). Fantasy, detective, crime fiction, thriller and science fiction reach it, but they do not reach a literary root.
   - `nonLiteraryGenreRoots` = `wd:Q188451` (music genre) and `wd:Q201658` (film genre).
   - Every class path is `?item (wdt:P31|wdt:P279)/wdt:P279* ?x`, walked **upward from a bound item** (never `?x wdt:P279* wd:Q223393`, which enumerates the whole subtree and times out on WDQS).
   - An item **is a literary genre** when `byClass || (byFiction && !nonLiterary)`:
     - `byClass`: the path reaches a literary root (novel, poetry, comedy, horror fiction, romance novel).
     - `byFiction`: the path reaches fiction genre. This replaces the planned usage signal and catches genres typed only as "genre of fiction".
     - `nonLiterary`: the path reaches music or film genre. It vetoes only the fiction branch. An item that is explicitly a literary genre always passes.
   - The planned **usage branch** (`EXISTS { ?work wdt:P136 ?item ; wdt:P31 wd:Q7725634 }`) is **dropped** for two reasons. It accepts music and film genres (rock music, action film and drama film are all P136 of some literary work). It is also unbounded for negatives: it scans every P136 user of a big music genre. Live it took 6.8-36 s, and with an `IF` guard it timed out (504). `literaryWorkClass` is therefore not added.
   - One SPARQL query classifies a whole batch (≤ 20 QIDs via `VALUES`, built only from validated `^Q[0-9]+$` QIDs, no user text in the query) and returns the `ru` label in the same query:
     ```sparql
     SELECT ?item ?ruLabel ?byClass ?byFiction ?nonLiterary WHERE {
       VALUES ?item { wd:Q1 wd:Q2 }
       OPTIONAL { ?item rdfs:label ?ruLabel . FILTER(LANG(?ruLabel) = "ru") }
       BIND(EXISTS { ?item (wdt:P31|wdt:P279)/wdt:P279* ?root . FILTER(?root IN (wd:Q223393, wd:Q4263830)) } AS ?byClass)
       BIND(EXISTS { ?item (wdt:P31|wdt:P279)/wdt:P279* ?fiction . FILTER(?fiction = wd:Q108465955) } AS ?byFiction)
       BIND(EXISTS { ?item (wdt:P31|wdt:P279)/wdt:P279* ?other . FILTER(?other IN (wd:Q188451, wd:Q201658)) } AS ?nonLiterary)
     }
     ```
     `hint:Prior hint:gearing "forward"` was tried and is **not** used: it made every class path false and was slower.
   - **Deadline:** the service wraps every classification call in `context.WithTimeout(ctx, 8s)` so `doRequest` retries cannot stretch a WDQS timeout into minutes.
2. **Search** (`GET /admin/genres/wikidata-search?q=`):
   1. `q` trimmed, empty → 400.
   2. `wbsearchentities` (`language=ru&uselang=ru&type=item`, limit **20** to leave room for filtering). Zero results → 200 `items: []`, no further calls.
   3. One classification query for all candidate QIDs.
      - OK: drop candidates that are not literary genres; `ru_label` comes from the query (`null` when absent); `class_check: "ok"`.
      - Fails (timeout, 5xx, rate limit, parse error): **fallback** — log a warning and return the unfiltered candidates with `literary_genre: null`, `ru_label: null` and `class_check: "unavailable"`. The import re-checks authoritatively.
   4. Keep the first **10** (search order).
   5. `existing` marker per candidate: one query `wikidata_qid IN (...)` that **includes alias rows**; each alias is followed with `followGenreAlias` to its canonical genre. Marker = canonical `{id, name, slug, status}` plus `via_alias: {id, name, slug} | null`. A marker lookup failure (or a broken alias) is logged and gives `existing: null` (the search stays 200), as for authors.
   6. Calls per search: 2 Wikidata requests + 1–2 DB queries.
3. **Import order** (`POST /admin/genres/import-wikidata {qid}`):
   1. Normalize (trim, uppercase), must match `^Q[0-9]+$` (reuse `importQIDPattern`, `ErrInvalidQID`) → else 400.
   2. **DB first by QID**: `GetGenreByQID` (unfiltered, may return an alias) → `followGenreAlias` → outcome `existing` (or `rejected` when the canonical genre is rejected), `match: "qid"`, `via_alias` set when an alias was followed. **No Wikidata call**, no class check (a QID that is already in BookHaven came from harvest or an admin).
   3. `FetchGenre` (wbgetentities, `props=labels|claims`, `languages=ru`). Missing → 404. Rate limited / unavailable → 502.
   4. Redirect (returned QID ≠ requested) → repeat step 2 with the canonical QID.
   5. **Name** = `ru` label, trimmed; empty or `!IsRussianText(name)` → 422 `no_ru_label`. No fallback to the search label.
   6. **Class check**: first locally, for free, on the fetched claims: `P31 ∪ P279` contains a root → accept. Otherwise run the classification query (rule 1) for the canonical QID: `false` → 422 `not_a_literary_genre`; the call fails → 502 (the import never skips the check).
   7. **Resolve** with `resolveGenreBySlug(name, canonicalQID)`, unchanged (genre-status rule 5 from the slug step, genre-moderation-queue rule 5):
      - alias by slug → canonical genre, `existing`/`rejected`, `match: "slug"`, `via_alias`, no backfill;
      - slug match without QID → QID backfilled (status unchanged), `qid_backfilled: true`, `existing` (or `rejected` if the genre is rejected — tombstone, backfilled first);
      - slug match with a **different** QID → `existing`, genre untouched, `qid_mismatch` = the genre's QID (no namesake genre, genre-status rule 5);
      - slug match with the same QID → `existing`;
      - no match → new `pending` genre with the QID → `created`.
   8. Race: `ErrDuplicateSlug` or `ErrDuplicateGenreQID` from create → re-run once from step 2 with the canonical QID (no second Wikidata call). Fails again → 409.
4. **The import never changes an existing genre's name, slug or status.** The only write to an existing row is the slug-match QID backfill (harvest rule).
5. Created genres are `pending` (never `published`, unlike manual `POST /admin/genres`). The admin publishes them in `/moderation/genres`.
6. **Authorization:** both endpoints live in the existing `genres := admin.Group("/genres", adminRole)` group; the POST adds `requireBearer`. The search is a GET (cookie-or-bearer). Moderators → 403, anonymous → 401.
7. **Wikidata etiquette:** every call goes through `cfg.doRequest` (User-Agent, rate limiter, retries). `cmd/api` passes the **same** `wikidataClient` instance already used for author search/import, so all admin Wikidata traffic shares one budget (1 rps). Expected latency: search ~1–2 s, import 1–2 s.

## Acceptance Criteria

Backend: bookdata
- [x] `SearchGenres(ctx, q) ([]GenreCandidate, error)` (`GenreCandidate{QID, Label, Description}`), limit 20, sharing an extracted `searchEntities(ctx, q, limit)` helper with `SearchAuthors` (author behavior and tests unchanged).
- [x] `ClassifyGenres(ctx, qids) (map[string]GenreClassification, error)` (`GenreClassification{RuLabel string; IsLiteraryGenre bool}` = `byClass || (byFiction && !nonLiterary)`, rule 1 as amended). Runs the rule-1 query on `cfg.baseURL` (SPARQL) via `doRequest`. 0 or > 20 QIDs, or any invalid QID → `ErrInvalidQuery` with no HTTP call. A QID missing from the bindings → `IsLiteraryGenre: false`. xsd:boolean literals parsed. Uses a raw-bindings helper extracted from `Query` (Query behavior unchanged).
- [x] `FetchGenre(ctx, qid) (*ExternalGenreRecord, error)` (`{QID, Name, InstanceOf, SubclassOf}`) on `getEntities`, same error mapping as `FetchAuthor` (invalid → `ErrInvalidQuery` without a call, missing → `ErrNotFound`, redirect → canonical `QID`). P31/P279 parsing shares a helper with `parseAuthorEntity`.
- [x] Exported `LiteraryGenreRoots` (used by the service's local check).

Backend: admin
- [x] Option `WithGenreWikidataSource(GenreWikidataSource)` (interface with the three methods above). Nil source → search and import (when step 2 does not short-circuit) return `ErrGenreImportUnavailable` (503).
- [x] `Service.SearchWikidataGenres(ctx, q) (*GenreSearchResult, error)` implements rule 2, including the fallback and the 8 s classification deadline.
- [x] `Service.ImportWikidataGenre(ctx, qid) (*GenreImportResult, error)` implements rule 3; `GenreImportResult{Genre, Outcome, Match, QIDBackfilled, QIDMismatch *string, ViaAlias *Genre}` built from `GenreResolution`.
- [x] New sentinels: `ErrNotALiteraryGenre` (422 `not_a_literary_genre`), `ErrGenreImportUnavailable` (503). Reused: `ErrInvalidQID` (400), `ErrWikidataEntityNotFound` (404), `ErrNoRussianLabel` (422 `no_ru_label`).
- [x] Repository `ListGenresByQIDs(ctx, qids)` — one query, **unfiltered** (includes alias rows), empty input → `[]` without a query. Every repository fake gains it.
- [x] Info log per import: `qid`, `outcome`, `match`, `via_alias_id`, `genre_id`.
- [x] `GET /api/v1/admin/genres/wikidata-search` and `POST /api/v1/admin/genres/import-wikidata` with the shapes and statuses below; Swagger annotations in Russian; swagger regenerated.

Frontend
- [x] `/import/genres` renders `GenreImportView`; the placeholder is gone.
- [x] Search input + "Найти" (Enter searches), loading/error/empty states. Empty hint: "Литературных жанров не найдено. Проверьте написание или создайте жанр вручную в каталоге."
- [x] Each candidate: name (`ru_label`, else `label` with "нет русского названия"), description (or "без описания"), QID link `https://www.wikidata.org/wiki/<QID>` (`target="_blank" rel="noopener noreferrer"`), and the `existing` marker (`GenreStatusBadge`, "через слитый жанр «X»" when `via_alias`, "Открыть" → `genreModerationHref(existing)`).
- [x] `class_check: "unavailable"` → a warning above the list: "Не удалось проверить, что это литературные жанры — проверка будет выполнена при импорте."
- [x] Actions: not existing and (`ru_label` or unknown) → "Импортировать"; existing pending/published → text "Уже в каталоге" (no button); existing rejected → "Отклонён ранее" (no button); `ru_label === null` with `class_check: "ok"` → "Нет русского названия" (no button). The clicked candidate's button is disabled while pending.
- [x] Result banner (green/destructive, as on `/import/author`) with per-outcome messages (UX) and links. After any import: invalidate `['admin','genres']` (prefix) and the dashboard key, then `search.refresh()`.
- [x] FSD: `app` imports only `views`; `views` compose features; no cross-feature imports.

## UX

- Page title "Импорт жанров из Wikidata", subtitle "Найдите литературный жанр в Wikidata и импортируйте его. Импортированные жанры ждут модерации."
- Messages (`features/import-wikidata-genre/lib/result-text.ts`):
  - created: «Жанр «{name}» импортирован и ждёт модерации.» Links: "Открыть" (`genreModerationHref`), "К модерации жанров" (`/moderation/genres`).
  - existing, qid, direct: «Жанр «{name}» уже есть в каталоге ({status label}).» + "Открыть".
  - existing via alias (qid or slug): «Этот жанр был слит в «{name}» ({status label}) — новый жанр не создан.» + "Открыть".
  - existing, slug, backfilled: «Найден жанр «{name}» без QID — QID {qid} привязан к нему.» + "Открыть".
  - existing, slug, `qid_mismatch`: «Жанр «{name}» уже есть в каталоге с другим QID ({qid_mismatch}) — новый жанр не создан. Если это дубликат или другое понятие, разберитесь в модерации.» + "Открыть" + "К модерации жанров".
  - existing, slug, otherwise: «Жанр «{name}» уже есть в каталоге ({status label}).»
  - rejected (any match): «Жанр «{name}» был отклонён ранее и повторно не создаётся.» (+ « QID привязан к нему.» when backfilled) + "Открыть". Destructive style.
  - 422 `no_ru_label`: «У элемента Wikidata нет русского названия — импорт невозможен. Создайте жанр вручную в каталоге.»
  - 422 `not_a_literary_genre`: «Элемент Wikidata не является литературным жанром — импорт отменён.»
  - 502: «Wikidata недоступна, попробуйте позже.» 503: «Импорт жанров не настроен на сервере.» Other: the `ApiError` message.
- Map 422 by `ApiError.code`, fall back to the message.

## API

`GET /api/v1/admin/genres/wikidata-search?q=фэнтези` (admin)
```json
{
  "class_check": "ok | unavailable",
  "items": [{
    "qid": "Q132311",
    "label": "фэнтези",
    "description": "жанр художественной литературы",
    "ru_label": "фэнтези",
    "literary_genre": true,
    "existing": {
      "id": "uuid", "name": "фэнтези", "slug": "fentezi", "status": "pending",
      "via_alias": { "id": "uuid", "name": "фентези", "slug": "fentezi-2" }
    }
  }]
}
```
`ru_label` and `literary_genre` are `null` when `class_check` is `unavailable`. `existing` and `via_alias` are always present (`null` when absent). 400 empty `q`, 502 search failed, 503 not configured, 401/403.

`POST /api/v1/admin/genres/import-wikidata` (admin + bearer), body `{"qid": "Q132311"}` → 201 `created`, 200 `existing`/`rejected`:
```json
{
  "outcome": "created | existing | rejected",
  "match": "qid | slug | null",
  "qid_backfilled": false,
  "qid_mismatch": null,
  "via_alias": null,
  "genre": { "...genreResponse": "id, name, slug, status, wikidata_qid" }
}
```
`via_alias` is a `genreResponse` or `null`; `genre` is the canonical genre. Errors: 400 `invalid qid`, 404 `wikidata entity not found`, 422 `{"error","code":"no_ru_label"|"not_a_literary_genre"}`, 409 `genre was created concurrently; retry`, 502 `wikidata is unavailable; retry later`, 503 `genre import is not configured`, 500 otherwise.

Routes in `internal/transport/http/router.go`, next to `genres.GET("/moderation", …)`:
```go
genres.GET("/wikidata-search", adminHandler.SearchWikidataGenres)
genres.POST("/import-wikidata", adminHandler.ImportWikidataGenre, requireBearer)
```
Regenerate swagger: `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`.

## Database

**No migration** (confirmed). Everything uses columns and indexes from 000017/000018: `wikidata_qid` + `idx_genres_wikidata_qid` (QID lookups and the `IN` query), `slug` unique, `merged_into_id`. Agents never run `migrate down` or any destructive DB command.

## Backend changes

- `internal/pkg/bookdata/wikidata_genre.go` (new): `GenreCandidate`, `SearchGenres`, `GenreClassification`, `ClassifyGenres` + query builder, `ExternalGenreRecord`, `FetchGenre`, `LiteraryGenreRoots`, `fictionGenreClass`, `nonLiteraryGenreRoots` (no `literaryWorkClass`, see rule 1).
- `wikidata_search.go`: extract `searchEntities` (SearchAuthors keeps limit 10). `wikidata.go`: extract a raw SPARQL request/bindings helper from `Query`. `wikidata_author.go`: extract the entity-ID claim parser for reuse.
- `modules/admin/service/genre_import.go` (new): `GenreWikidataSource`, sentinels, `SearchWikidataGenres`, `ImportWikidataGenre`, result types, `isLiteraryGenreLocally`.
- `modules/admin/service/service.go`: `genreSource` field + `WithGenreWikidataSource`.
- `modules/admin/repository`: `ListGenresByQIDs` (interface + Gorm + all fakes).
- `modules/admin/transport/http/handler.go` (or a new `handler_genre_import.go`): two handlers, DTOs (reuse `toGenreResponse`), add the methods to the handler's `Service` interface and its fakes.
- `cmd/api/main.go`: `adminservice.WithGenreWikidataSource(wikidataClient)` with the existing instance. `cmd/worker` unchanged.
- `harvest_resolve.go`: **no behavior change**. The import calls `resolveHarvestGenre`'s parts (`GetGenreByQID` + `followGenreAlias`, then `resolveGenreBySlug`) directly; only update the "future #303" comment.

## Frontend

`packages/api-client/src/index.ts`
- `AdminGenreWikidataCandidate { qid; label; description; ru_label: string | null; literary_genre: boolean | null; existing: AdminGenreExistingMarker | null }`, `AdminGenreExistingMarker { id; name; slug; status: AdminGenreStatus; via_alias: { id; name; slug } | null }`, `AdminGenreWikidataSearchResult { class_check: 'ok' | 'unavailable'; items }`, `AdminGenreImportResult`.
- `api.admin.genres.searchWikidata(q)`, `api.admin.genres.importWikidata(qid)`; exposed via `adminGenresApi` (`entities/genre/api`).

`apps/admin`
- `features/search-wikidata-genre/` (new): `useWikidataGenreSearch()` (name, candidates, classCheck, error, hasSearched, isPending, `search`, `refresh`) + `WikidataGenreSearch({ search, inputId, label, emptyHint, renderActions })`. Mirrors `features/search-wikidata-author`; do not generalize the author widget (different types; keep the author slice untouched).
- `features/import-wikidata-genre/` (new): `useImportWikidataGenre()` → `importGenre(candidate)`, `pendingQid`; invalidations; `lib/result-text.ts` (`genreImportResultFeedback`, `genreImportErrorMessage`), pure and unit-tested. Imports only `entities/genre` and `shared`.
- `views/genre-import/` (new): `GenreImportView` composes both features (banner + search + actions). Mirrors `views/author-import`.
- `app/import/genres/page.tsx`: render `GenreImportView`. `PlaceholderPage` then has no caller: delete `shared/ui/placeholder-page.tsx` and its export.
- Navigation tab already exists (`import.genres`); no change.
- typedRoutes: query-string hrefs use the existing `as any` convention. The CI frontend build is the gate; no native `next build`.

## Existing Patterns

- `bookdata/wikidata_search.go` `SearchAuthors`, `wikidata_author.go` `FetchAuthor`/`getEntities`, `wikidata.go` `Query` (SPARQL), test style via `WithSearchBaseURL`/`WithBaseURL` + fake limiter.
- `admin/service/author_import.go` (`ImportWikidataAuthor`: DB-first, redirect, ru label, allowlist, one retry) and `FindAuthorsByWikidataQIDs`.
- `admin/service/harvest_resolve.go` `resolveHarvestGenre`, `resolveGenreBySlug`, `followGenreAlias`, `GenreResolution`.
- `harvest/transport/http/handler.go` `SearchAuthors`: `existing` marker, finder failure → `null`.
- Frontend: `views/author-import`, `features/search-wikidata-author`, `features/import-wikidata-author/lib/result-text.ts`, `entities/genre` (`GenreStatusBadge`, `GENRE_STATUS_LABELS`, `genreModerationHref`).

## Tests

Backend (via `docker compose run`, changed packages only; gofmt only changed files; no live network; never `migrate down`):
- `bookdata` (httptest fixtures):
  - `SearchGenres`: params (`language=ru`, `limit=20`), parsing, empty query → no call; `SearchAuthors` tests unchanged.
  - `ClassifyGenres`: the query contains `VALUES` with exactly the given QIDs, the three upward `EXISTS` walks, the roots, and no `wdt:P279* wd:Q…` subtree pattern; `byClass`/`byFiction`/`nonLiterary` combos; missing ru label; QID missing from bindings → false; invalid/empty/> 20 → `ErrInvalidQuery` without a call; 5xx → `ErrSourceUnavailable`; malformed JSON → wrapped error; User-Agent asserted.
  - `FetchGenre`: ru label, P31 and P279 parsed, `novalue` skipped, redirect → canonical QID, missing → `ErrNotFound`, invalid → no call.
- `admin/service` `genre_import_test.go` (fakes):
  - search: non-genres filtered; cap 10 after filtering; `ru_label` from classification; classification error and deadline → unfiltered + `unavailable`; zero results → no classify call; `existing` direct and via alias (target returned, `via_alias` set); marker lookup error → `null`; nil source → `ErrGenreImportUnavailable`.
  - import: QID found pending/published → `existing` with no fetch; QID found rejected → `rejected`; QID on an alias → canonical + `via_alias` (and rejected target → `rejected`); redirect → canonical lookup and storage; no/Latin ru label → `ErrNoRussianLabel`; local class hit → no classify call; local miss + classify true → created; classify false → `ErrNotALiteraryGenre`; classify error → 502 sentinel passthrough; new → pending + QID `created`; slug without QID → backfilled `existing`; slug rejected → `rejected` + backfill; slug different QID → `existing` + `qid_mismatch`, nothing written; slug alias → canonical, no backfill; duplicate on create → one retry → `existing`; normalized/invalid QID; nil source with DB short-circuit still works.
  - existing `harvest_resolve`/`genre_status`/`genre_moderation` tests unchanged and green.
- `admin/repository` SQLite: `ListGenresByQIDs` returns alias rows, ignores others, empty input → no query.
- `admin/transport/http`: both handlers — shapes (`null`s present), 201/200, every error status with `code`, 401/403, POST without bearer → 401.

Frontend (admin vitest):
- `features/search-wikidata-genre`: renders candidates (ru label, description, QID link, badge, via-alias text), unavailable warning, empty hint, error.
- `features/import-wikidata-genre` `result-text`: every outcome/flag/code message and links.
- `views/genre-import/ui/genre-import-view.test.tsx`: import calls `importWikidata`; actions per candidate state (new, existing, rejected, no ru label, unknown); banner + links; refresh after import; invalidations.
- Remove nothing else; if a test referenced the placeholder on `/import/genres`, update it.

## Documentation

- This file → `Status: done` + Implementation Notes (including the live QID/timing verification of rule 1).
- `docs/ROADMAP.md`: mark #303 done and **tick the epic #290 line** (all three features shipped).
- `docs/features/admin-navigation-restructure.md` phase 7: item 3 links here and is marked done; drop "the `/import/genres` placeholder stays until #303" and the placeholder mentions (`PlaceholderPage` removed).
- `genre-status.md` and `genre-moderation-queue.md` "Out of scope": one-line pointer that #303 shipped.
- Swagger regenerated.

## Risks

- **Class-check precision.** Wikidata genre modeling is messy. Some literary genres reach neither a literary root nor fiction genre: items typed only as "genre" (Q483394) that the dropped usage branch would have accepted. That is a false negative → 422; the admin creates the genre manually, and the root list can grow. A fiction genre that is also a TV or video-game genre (not in `nonLiteraryGenreRoots`) can pass. That is a false positive; moderation catches it, and the import only creates `pending`.
- **WDQS latency/timeouts.** Bounded by upward-only paths, `EXISTS`, a ≤ 20 batch and an 8 s deadline. Search degrades to unfiltered candidates; import refuses with 502 only when the free local check also misses.
- **Root QIDs**: verified live during implementation (`Q12308638` turned out to be "poetry collection" and was replaced by `Q4263830`). A wrong root silently narrows or widens the filter.
- **Slug backfill on a QID-less genre** may attach the QID to a same-named different concept (inherent harvest rule); the banner says so explicitly with a link.
- **Different-QID slug match** reports `existing` without creating anything (genre-status rule 5); two concepts sharing a ru label stay one genre until an admin intervenes.
- **Rate budget**: search costs 2 requests on a 1 rps limiter shared with author search/import; use the same client instance (verify in review).
- **Alias invariant breach** (target missing/alias) → import 500 and marker `null`, logged by `followGenreAlias`; same fail-safe as harvest.

## Implementation Notes

**Rule 1 amendment (live verification, 2026-10-08).** I checked the public Wikidata APIs with a descriptive User-Agent: 4 action-API requests and 8 SPARQL queries, run sequentially. The results contradicted the plan in two places, so rule 1 above is amended. Everything else follows the contract as planned.

- **Root QIDs** (wbgetentities): `Q223393` = "literary genre" (ru «литературный жанр») ✓. `Q7725634` = "literary work" ✓. `Q483394` = "genre" (correctly not a root). `Q12308638` = **"poetry collection"** ✗. wbsearchentities gives "literary form" = **`Q4263830`**, now used as the second root (novel `Q8261` is `P31 Q4263830, Q223393`).
- **The planned query** (literary roots + P136 usage), 6 items: 4.5 s. Novel passed by class. Fantasy and detective failed by class and passed only via usage. **Rock music `Q11399` and action film `Q188473` passed via usage**, which is wrong.
- **The planned query plus a music/film veto**, 14 items: 10.2 s (over the 8 s deadline). Timing the branches alone: the class walks took 1.3 s, and usage alone returned **502 after 6.8 s**. Usage is unbounded for negatives, because it scans every P136 user of a big music genre. Variants also failed: `hint:Query hint:optimizer "None"` took 27 s, the path form `?item ^wdt:P136/wdt:P31 wd:Q7725634` took 36 s, and `IF(?byClass || ?nonLiterary, false, EXISTS{usage})` timed out (504, 65 s) because Blazegraph does not evaluate EXISTS lazily. `hint:Prior hint:gearing "forward"` on the class paths made every path false and took 13 s.
- **The amended query** (as shipped), 20 items: **3.45 s, HTTP 200**.
  - Accepted by class: роман `Q8261`, поэзия `Q482`, комедия `Q40831`, литература ужасов `Q193606`, любовный роман `Q858330`, фэнтезийная литература `Q1057172`.
  - Accepted by fiction genre with no veto: фэнтези `Q132311`, детектив `Q186424`, триллер `Q182015`, научная фантастика `Q24925`, криминальный жанр `Q5937792`, жанр фантастики `Q20076756`.
  - Rejected: джаз `Q8341`, рок-музыка `Q11399` and хард-рок `Q83270` (music); боевик `Q188473`, фильм ужасов `Q200092` and драматический фильм `Q130232` (film); «Война и мир» `Q161531` and «Гарри Поттер» `Q8337` (works, not genres).
  - The dystopia QID I guessed (`Q20937557`) is actually «ряд», so антиутопия was not checked.
- Search latency is therefore about 0.5-1 s for wbsearchentities plus about 1.5-3.5 s for classification. The 8 s deadline leaves headroom; on a WDQS slowdown, search degrades to `class_check: "unavailable"`.

**Other implementation details.**
- bookdata helpers extracted, with unchanged behavior: `searchEntities` (shared by `SearchAuthors`/`SearchGenres`), `sparqlBindings` (shared by `Query`/`ClassifyGenres`), `entityIDClaims` (P31/P279 parsing, used by `parseAuthorEntity` and `FetchGenre`) and `getEntity` (single-entity pick with redirect/missing handling, shared by `FetchAuthor`/`FetchGenre`).
- The service's local check accepts direct `P31`/`P279` ∈ `LiteraryGenreRoots` only. Fiction genre is not checked locally, so the music/film veto always comes from SPARQL. Any classification failure in the import becomes a 502: a non-bookdata error, such as the deadline or a parse error, is wrapped with `bookdata.ErrSourceUnavailable`.
- Search: an empty `q` gives 400 in the handler. If wbsearchentities returns more than 20 hits, they are truncated to 20 before classification. Markers come from `ListGenresByQIDs` (one query) plus one `GetGenreByID` per alias hit (`followGenreAlias`). A broken alias gives `existing: null` for that QID only.
- Import: the QID step is `resolveGenreByQID` in `genre_import.go`, which is `GetGenreByQID` + `followGenreAlias`, the same as the QID step of `resolveHarvestGenre`. Harvest code is unchanged except a comment. `qid_mismatch` is set for a non-alias slug match whose genre has a different QID. Both `ErrDuplicateSlug` and `ErrDuplicateGenreQID` trigger the one retry, and either one maps to 409.
- Frontend: the "existing, slug, otherwise" banner also carries "Открыть", which matches the QID case. The unchecked-classification warning has `role="alert"`. `PlaceholderPage` and its `@/shared/ui` export are deleted; nothing else used them.
- Swagger regenerated with `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal` (the `make swagger` command, run via `docker compose run`): additions only.
