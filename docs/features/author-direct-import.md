# Author Direct Import from Wikidata

Status: done

Epic: "Author moderation & import", feature 3 of 4. Builds on [author-status.md](author-status.md) (feature 1: `status`, QID, tombstones, harvest resolver rule 4), [author-moderation-queue.md](author-moderation-queue.md) (feature 2: `/authors/moderation`) and [harvest-author-search.md](harvest-author-search.md) (`SearchAuthors`, `GET /admin/harvest/authors/search`, `POST /admin/harvest`). Those docs are the source of truth and are not reopened here.

Feature 4 ([author-wikidata-enrichment.md](author-wikidata-enrichment.md)) shipped: a `created` import is now enriched with dates and a Commons photo from the same Wikidata fetch, and the response gains `enrichment`.

## Goal

Let an admin bring a specific author into BookHaven from Wikidata without running a harvest first. The admin searches by name, picks a candidate, and either imports only the author (pending, with QID, lands in the moderation queue) or imports the author and starts the existing async harvest of their books. Matching uses exactly the harvest rules, so no duplicates are created and rejected authors are never recreated.

## Out of scope

- Feature 4: enrichment (day-precision birth/death dates, Commons photo with attribution, biography). This feature only designs the fetch helper so feature 4 can extend it.
- Any change to `POST /admin/harvest`, `BuildAuthorListQuery`, the worker, or the harvest jobs UI.
- Bulk import of several authors at once (one candidate per click).
- Server-side P31 filtering of search results (the search stays unfiltered, per harvest-author-search.md). The P31 check applies to the single-author import only.
- Slug/namesake markers in the candidate list. Only QID matches are marked; slug matching happens at import time.
- A migration. Nothing in this feature needs schema changes.

## User Stories

- As an admin on "Модерация авторов", I open "Импорт из Wikidata", type a name and see candidates with description, QID link and, if the QID is already in BookHaven, the existing author's status.
- As an admin, I click "Импортировать автора" and get a pending author with the Wikidata QID and the Russian Wikidata name, visible at the top of the pending queue.
- As an admin, I click "Импортировать с книгами" and get the same author import plus a harvest job for that QID. The books land hidden in the books review queue.
- As an admin, if the author already exists (by QID or by name), nothing is duplicated. I am told which author matched and can open it.
- As an admin, if the author was rejected earlier, the import is refused with a clear message and a link to the rejected author.
- As an admin, if the Wikidata item has no Russian name or is not a person, the import is refused with a clear reason.

## Business Rules

1. **Entry point.** The primary (and only) import UI is a panel "Импорт из Wikidata" on `/authors/moderation`. The search widget is extracted into a shared feature slice and reused by `/harvest`. The harvest page keeps its current role, which is collecting several QIDs for one harvest job, and gets no import buttons. Rationale: import produces a pending author, and the moderation page is where pending authors are processed. The created author appears immediately on top of the pending tab, which is ordered newest first.
2. **Import order (single QID):**
   1. Normalize: trim, uppercase. Must match `^Q[0-9]+$`, otherwise 400.
   2. **DB first by QID** (`GetAuthorByExternalID(wikidata, qid)`). If found, return without calling Wikidata: `rejected` → outcome `rejected`, otherwise outcome `existing` (`match: "qid"`). This works even when Wikidata is down.
   3. Fetch the entity from Wikidata (`FetchAuthor`). Missing entity → 404. Rate limited or unavailable → 502.
   4. If Wikidata resolved a redirect (returned QID ≠ requested QID), use the returned canonical QID and repeat step 2 with it.
   5. **Name** = the Wikidata `ru` label of the entity. If there is no `ru` label, or `IsRussianText(label)` is false → refuse with 422 `no_ru_label`. **Decision: refuse, do not fall back to the search label.** `wbsearchentities` with `uselang=ru` applies language fallback, so its label can be English. This is consistent with russian-only-content.md: harvest only takes `ru` labels. The admin can still create the author manually.
   6. **Person check:** P31 (instance of) must contain at least one of `Q5` (human), `Q61002` (pseudonym), `Q16017119` (collective pseudonym), otherwise refuse with 422 `not_a_person`. The data comes from the same entity call, so the check costs nothing extra. The pseudonym values keep authors like «Козьма Прутков» importable. The allowlist is a constant in the admin service.
   7. **Resolve with the harvest rules** (author-status.md rule 4, from the slug step on) using `{name, canonical QID}`:
      - slug match, rejected → outcome `rejected` (QID backfilled if the author had none, same as harvest).
      - slug match, no QID → QID backfilled, status unchanged → outcome `existing`, `match: "slug"`, `qid_backfilled: true`.
      - slug match, same QID → `existing` (cannot normally happen after step 2, but handled).
      - slug match, different QID (namesake) → new pending author with slug `slugify(name)-q<id>` → outcome `created`, `namesake_of` = the existing author.
      - no match → new **pending** author `{name, slug, external_source: wikidata, external_id: QID}` → outcome `created`.
   8. Race: if create fails with a duplicate slug or a QID unique violation, re-run from step 2 once. If it fails again → 409.
3. **The import never changes an existing author's status, name or slug.** The only possible write to an existing row is the QID backfill (harvest rule).
4. **Import + books (b) is the UI calling two endpoints** (decision): first `POST /admin/authors/import-wikidata`, then, only if the outcome is `created` or `existing`, the unchanged `POST /admin/harvest {author_qids: [author.external_id]}` with no `limit`, so the server default applies. Rationale: no new endpoint, and the admin module stays free of asynq (as decided in admin-harvest-trigger.md). Both steps are idempotent. If the harvest call fails, the admin clicks again: the import returns `existing` and the harvest is retried. On `rejected` or any import error, the harvest is **not** started. The QID sent to harvest is the stored one (`author.external_id`), which is the canonical QID after a redirect.
5. **"Already in BookHaven" markers.** The existing `GET /admin/harvest/authors/search` response gains an optional `existing` object per candidate (QID match only), computed with **one** query `WHERE external_source='wikidata' AND external_id IN (...)` for the ≤ 10 candidate QIDs. This is an additive change, and `/harvest` shows the same marker. Slug matches are not marked (a matching name does not prove the same person). Import handles them and explains the result.
6. **Authorization:** the import endpoint is in the `authors` group (`adminRole`) plus `requireBearer`. The search endpoint is unchanged (`adminRole`, cookie-or-bearer GET). Moderators get 403, unauthenticated requests get 401.
7. Wikidata etiquette: `FetchAuthor` goes through `cfg.doRequest` (User-Agent, shared rate limiter, retries). `cmd/api` passes **the same** `WikidataClient` instance it already builds for search, so search and import share one rate-limit budget.

## Acceptance Criteria

Backend: bookdata
- [x] `bookdata.ExternalAuthorRecord{QID string; Name string; InstanceOf []string}` (doc comment: feature 4 adds dates, photo and similar fields here).
- [x] `(*WikidataClient).FetchAuthor(ctx, qid) (*ExternalAuthorRecord, error)` calls `wbgetentities` on `cfg.searchBaseURL` (`action=wbgetentities&ids=<qid>&props=labels|claims&languages=ru&format=json`) via `cfg.doRequest`.
- [x] An invalid QID returns `ErrInvalidQuery` with no HTTP call. A missing entity (`"missing"` key, or Wikidata error `no-such-entity`) returns `ErrNotFound`. HTTP 400 returns `ErrInvalidQuery`. Other ≥ 400 errors use the same mapping as `SearchAuthors`.
- [x] `QID` = the returned entity's `id` (canonical after a redirect). `Name` = `labels.ru.value` or `""`. `InstanceOf` = valid QIDs from the `claims.P31[].mainsnak.datavalue.value.id` values (skip `novalue`/`somevalue`).
- [x] The response body is read through `io.LimitReader` (e.g. 8 MiB, since famous entities have large claim sets). Malformed JSON returns a wrapped parse error, not a panic.

Backend: admin
- [x] Resolver refactor: the slug/create part of `resolveHarvestAuthor` returns a resolution `{Author, Outcome (created|existing|rejected), Match (qid|slug), QIDBackfilled bool, NamesakeOf *Author}`. `ResolveOrCreateHarvestAuthors` maps `rejected` → skip. **All existing `harvest_resolve_test.go` tests pass unchanged.**
- [x] `Service.ImportWikidataAuthor(ctx, qid) (*AuthorImportResult, error)` implements rule 2 exactly. Created authors are `pending` and carry the QID.
- [x] New option `WithAuthorFetcher(AuthorFetcher)` (interface `FetchAuthor(ctx, qid) (*bookdata.ExternalAuthorRecord, error)`). If the fetcher is nil and step 2 does not short-circuit → `ErrAuthorImportUnavailable` (503).
- [x] Sentinels: `ErrInvalidQID` (400), `ErrWikidataEntityNotFound` (404), `ErrNoRussianLabel` (422, code `no_ru_label`), `ErrNotAPerson` (422, code `not_a_person`). `bookdata.ErrRateLimited`/`ErrSourceUnavailable` → 502. `ErrDuplicateSlug` after the retry → 409.
- [x] `POST /api/v1/admin/authors/import-wikidata` → 201 for `created`, 200 for `existing` and `rejected`, using the response shape below.
- [x] Repository `ListAuthorsByExternalIDs(ctx, source, ids)` runs one query. The service method `FindAuthorsByWikidataQIDs(ctx, qids) (map[string]catalogDomain.Author, error)` wraps it.
- [x] An info log line per import (qid, outcome, author id).

Backend: harvest search
- [x] `harvesthttp.NewHandler` gains a 4th dependency `ExistingAuthorFinder` (interface, satisfied by `*adminservice.Service`). `SearchAuthors` fills `existing: {id, name, slug, status} | null` per item with one finder call. If the finder fails → the error is logged and the search still returns 200 with `existing: null` for every item (orchestrator adjustment, see Implementation Notes). Zero candidates → no finder call.
- [x] All existing search handler tests pass after the constructor update.

Frontend
- [x] `/authors/moderation` has a header button "Импорт из Wikidata" that toggles a panel containing the shared search widget.
- [x] Each candidate shows: label, description (or "без описания"), QID link to `https://www.wikidata.org/wiki/<QID>` (new tab, `noopener noreferrer`), and, if `existing`, an `AuthorStatusBadge` and an "Открыть" link to `/authors/moderation?status=<status>&search=<name>`.
- [x] Candidate actions:
  - not existing: "Импортировать автора", "Импортировать с книгами".
  - existing pending/published: "Импортировать книги" (harvest only; it still calls import first, which returns `existing`).
  - existing rejected: no import actions, only the text "Отклонён ранее" and "Открыть".
- [x] "Импортировать с книгами" / "Импортировать книги" ask `confirm()` first ("Запустить харвест книг автора «X»? Книги появятся скрытыми в очереди на проверку.").
- [x] Result banner (same green/destructive banner as the view), with messages per outcome (see UX). The buttons of the clicked candidate are disabled while pending.
- [x] After any import: invalidate `['admin','authors']`, `['admin','dashboard']`, and re-run the current search so the `existing` badges refresh. After a harvest trigger: also invalidate the harvest jobs query key.
- [x] `/harvest` uses the same extracted widget (behavior unchanged: "Добавить" / "Добавлено", chips, manual QID mode) and now also shows the `existing` badge.

## UX

- Panel: name input + "Найти" (Enter also searches) and a candidate list, exactly as on `/harvest`. The empty-state text is a prop: the moderation page says "Совпадений не найдено. Проверьте написание или создайте автора вручную."
- Messages:
  - created: «Автор «{name}» импортирован и ждёт модерации.» With a namesake: add «В каталоге уже есть тёзка «{namesake}» с другим QID — создан отдельный автор.»
  - existing (qid): «Автор «{name}» уже есть в каталоге ({status label}).» + "Открыть".
  - existing (slug, backfilled): «Найден автор «{name}» без QID — QID {qid} привязан к нему.» + "Открыть".
  - rejected: «Автор «{name}» был отклонён ранее и повторно не создаётся.» + "Открыть". For the with-books action, add «Книги не импортированы.»
  - harvest started: append «Харвест книг запущен — книги появятся скрытыми в очереди на проверку.» + link "К харвесту" (`/harvest`).
  - harvest failed after import: «Автор импортирован, но харвест не запущен: {error}. Нажмите «Импортировать книги» ещё раз.»
  - 422 `no_ru_label`: «У элемента Wikidata нет русского названия — импорт невозможен. Создайте автора вручную.»
  - 422 `not_a_person`: «Элемент Wikidata не является человеком или псевдонимом — импорт отменён.»
  - 502: «Wikidata недоступна, попробуйте позже.» Other errors: the `ApiError` message.
- Map by `ApiError.code` for the 422 codes. Fall back to the message.

## API

`POST /api/v1/admin/authors/import-wikidata` (admin role + bearer)
- Request: `{"qid": "Q7200"}`
- 201 / 200:
```json
{
  "outcome": "created | existing | rejected",
  "match": "qid | slug | null",
  "qid_backfilled": false,
  "namesake_of": null,
  "author": { "...authorResponse (id, name, slug, status, external_source, external_id, ...)": "" }
}
```
`match` is `null` for `created`. `namesake_of` is an `authorResponse` or `null`.
- 400 `{"error":"invalid qid"}`, 404 `{"error":"wikidata entity not found"}`, 422 `{"error":"...","code":"no_ru_label"|"not_a_person"}`, 409 duplicate after retry, 502 Wikidata unavailable, 503 fetcher not configured, 401/403.
- Route: `authors.POST("/import-wikidata", adminHandler.ImportWikidataAuthor, requireBearer)` next to the other `authors` routes. Add Swagger annotations and regenerate swagger (`swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`).

`GET /api/v1/admin/harvest/authors/search?q=`: each item gains `"existing": {"id","name","slug","status"} | null` (always present, `null` when absent). No other changes.

`POST /api/v1/admin/harvest`: unchanged, reused for (b).

## Database

No migration. Uses the existing `idx_authors_external_source_id` partial unique index (000015) for the QID lookups and for the `IN` query.

## Backend changes

- `pkg/bookdata/wikidata_author.go` (new): `ExternalAuthorRecord`, `FetchAuthor`, wbgetentities response structs. Reuses `wikidataQIDPattern`.
- `modules/admin/service/harvest_resolve.go`: resolution struct and refactor (rule 2.7). Behavior is identical for harvest.
- `modules/admin/service/author_import.go` (new, one file per action): `ImportWikidataAuthor`, `AuthorImportResult`, the P31 allowlist, sentinels, `FindAuthorsByWikidataQIDs`.
- `modules/admin/service/service.go`: `authorFetcher` field + `WithAuthorFetcher`.
- `modules/admin/repository`: `ListAuthorsByExternalIDs`.
- `modules/admin/transport/http/handler.go`: `ImportWikidataAuthor` handler + response DTO (reuse `toAuthorResponse`). Add `ImportWikidataAuthor` to the handler's `Service` interface and to the fakes.
- `modules/harvest/transport/http/handler.go`: `ExistingAuthorFinder` interface, 4th `NewHandler` param, `existing` in `authorCandidateResponse`.
- `transport/http/router.go` (`RegisterAdminRoutes`): pass the finder (admin service) to `harvesthttp.NewHandler`, add the route.
- `cmd/api/main.go`: `adminService.WithAuthorFetcher(wikidataClient)` using the same instance passed as `AuthorSearcher`. `cmd/worker` does not need it (the fetcher is optional).

## Frontend

`packages/api-client/src/index.ts`
- `AdminHarvestAuthorCandidate.existing: { id: string; name: string; slug: string; status: AdminAuthorStatus } | null`.
- `AdminAuthorImportResult { outcome: 'created' | 'existing' | 'rejected'; match: 'qid' | 'slug' | null; qid_backfilled: boolean; namesake_of: AdminAuthor | null; author: AdminAuthor }`.
- `api.admin.authors.importWikidata(qid: string)` → `POST /api/v1/admin/authors/import-wikidata`.

`apps/admin` (FSD)
- `entities/author/api`: re-export `importWikidata`.
- `features/search-wikidata-author/` (new, extracted from `views/harvest/ui/author-name-search.tsx`): `WikidataAuthorSearch({ renderActions(candidate), emptyHint })` holds the input, the search mutation, error/empty states, the candidate cards (label, description, QID link, `existing` badge via `entities/author` `AuthorStatusBadge`). It exposes a way to re-run the last search (e.g. the search mutation lives in a hook `useWikidataAuthorSearch` returned to the parent, or the widget accepts a `refreshKey`; developer's call). No `<form>` inside (it is nested in harvest's form).
- `views/harvest/ui/author-name-search.tsx`: becomes a thin composition of `WikidataAuthorSearch` ("Добавить"/"Добавлено" action) + the selected chips. `harvest-view.tsx` is otherwise unchanged.
- `features/import-wikidata-author/` (new): `useImportWikidataAuthor()` with `importAuthor(candidate)` and `importWithBooks(candidate)` (import → gate on outcome → `adminHarvestApi.trigger`), invalidations, and `lib/result-text.ts` message builders (pure, unit-tested). It imports `entities/author` and `entities/harvest` only (no cross-feature imports).
- `views/author-moderation/ui/wikidata-import-panel.tsx` (new): composes both features (search widget + import actions + result banner). `author-moderation-view.tsx` adds the toggle button and renders the panel. Panel state is local, not in the URL.
- typedRoutes: hrefs with query strings use the existing `as any` convention. The CI frontend build is the gate (do not run a native `next build`).

## Existing Patterns

- `bookdata/wikidata_search.go` `SearchAuthors`: same host (`searchBaseURL`), `doRequest`, status mapping, and test style via `WithSearchBaseURL`.
- `admin/service/harvest_resolve.go` `resolveHarvestAuthor` / `createHarvestAuthor`: the matching rules (refactored, not duplicated).
- `admin/service/lookup.go` + handler `LookupBook`: an optional bookdata client wired via an `Option`, 502 mapping.
- `admin/service/bulk_publish.go`, `author_moderation.go`: one file per action.
- `features/moderate-author` (`useAuthorStatusMutation`, `lib/confirm-text.ts`): mutation hook + pure text builders + invalidations.
- `author-moderation-view.tsx`: feedback banner, `AuthorStatusBadge`, `AUTHOR_STATUS_LABELS`.

## Tests

Backend (via `docker compose run`; never `migrate down` or reset the DB):
- `bookdata` `FetchAuthor` (httptest via `WithSearchBaseURL`): ru label + P31 parsed; no ru label → `Name ""`; redirect → canonical `QID`; missing entity → `ErrNotFound`; invalid QID → `ErrInvalidQuery` with no HTTP call; 400 → `ErrInvalidQuery`; 5xx → `ErrSourceUnavailable`; malformed JSON → wrapped error; `novalue` P31 snak skipped; the request goes through `doRequest` (User-Agent asserted, as in the existing tests).
- `admin/service` `ImportWikidataAuthor` (fakes):
  - QID found published/pending → `existing` with no fetcher call.
  - QID found rejected → `rejected` with no fetcher call.
  - New → pending + QID, `created`.
  - Slug match without QID → backfill, `existing/slug`.
  - Slug match rejected → `rejected` + backfill.
  - Namesake → `-q` slug, `namesake_of` set.
  - Redirect → canonical QID used for the lookup and storage.
  - No ru label / Latin label → `ErrNoRussianLabel`.
  - P31 Q5 / Q61002 / Q16017119 accepted; other → `ErrNotAPerson`.
  - Fetcher `ErrNotFound` → `ErrWikidataEntityNotFound`; rate limited → passed through.
  - Nil fetcher → `ErrAuthorImportUnavailable`.
  - Duplicate on create → one retry → `existing`.
  - Lowercase / whitespace QID normalized; invalid → `ErrInvalidQID`.
- `admin/service` harvest resolver: existing tests unchanged and green (regression guard for the refactor).
- `admin/repository` SQLite test: `ListAuthorsByExternalIDs` (match, other source ignored, empty input → no query/empty).
- `admin/transport/http`: import handler status codes (201/200/400/404/409/422 with `code`/502/503), response shape, 401 without bearer, 403 moderator.
- `harvest/transport/http`: `existing` filled/null; finder error → 200 with `existing: null`; finder not called for zero candidates; existing tests updated for the new constructor param.

Frontend (vitest):
- `features/search-wikidata-author`: renders candidates with QID link and existing badge; empty hint prop; error message.
- `features/import-wikidata-author` `result-text`: every outcome/code message.
- `wikidata-import-panel.test.tsx`: import-only calls `importWikidata` and not `trigger`; with books → `importWikidata` then `trigger({author_qids:[author.external_id]})`; `rejected` outcome → no `trigger`, message + "Открыть" href; 422 `no_ru_label` → message, no `trigger`; harvest failure after import → partial message; existing-rejected candidate has no import buttons; confirm cancel → nothing called.
- `harvest-view.test.tsx` / `author-moderation-view.test.tsx`: existing tests still pass (adjust mocks for the `existing` field); toggle button shows the panel.

## Documentation

- This file → `Status: done` + Implementation Notes on completion.
- `harvest-author-search.md`: note the `existing` field and the widget extraction to `features/search-wikidata-author`.
- `author-status.md` / `author-moderation-queue.md`: one-line pointer in "Out of scope" that feature 3 shipped.
- `docs/ROADMAP.md`: add/tick the epic item (feature 3 of 4).
- Swagger regenerated.

## Risks

- **wbgetentities payload size** for famous authors (all claims, hundreds of KB). This is one call per click and is bounded by `LimitReader` and the 15 s timeout. Feature 4 needs the claims anyway (P569/P570 with precision, P18), so the same call stays reusable.
- **Slug-match backfill can attach the QID to a QID-less namesake** (e.g. a legacy "Иван Петров" who is a different person). This is inherent to the harvest rule, which import must mirror. Mitigated: the result message says explicitly "найден автор без QID — QID привязан", with a link, so the admin can check and fix it (merge, or edit the author).
- **No ru label → refusal** blocks some foreign authors that only have English labels. Accepted per russian-only-content.md. Manual creation remains possible.
- **P31 allowlist** may reject legitimate edge cases (e.g. heteronyms, or items typed only as "writer" without P31=Q5). The 422 message points to manual creation. Extend the allowlist if it recurs.
- **Two-step (b) is not atomic.** The author can be imported while the harvest enqueue fails. It is visible in the banner, and a repeat click is idempotent. Accepted over adding asynq to the admin module.
- **Harvest of an existing published author** may add books for an author an admin already curated. This is intended: the books are hidden in the review queue.
- **Concurrent imports / harvest** racing on the same QID or slug: one retry, then 409. Same class as the existing harvest race.
- **Search `existing` lookup adds a DB query** to a previously DB-free endpoint. The query is a cheap indexed `IN` over ≤ 10 values. A DB failure does not fail the search: it is logged and the markers are `null` (see Implementation Notes).
- **Shared rate limiter**: search and import in `cmd/api` must use the same `WikidataClient` instance. A second instance would double the outbound rate. Verify in review.

## Implementation Notes

- **Orchestrator adjustment (search markers):** if `FindAuthorsByWikidataQIDs` fails, `GET /admin/harvest/authors/search` does **not** return 500. It logs the error (`c.Logger().Errorf`) and returns the candidates with `existing: null`, so the search stays usable; the import endpoint does the authoritative match anyway. Covered by `TestSearchAuthors_FinderError_ReturnsCandidatesWithNullExisting`.
- `bookdata.FetchAuthor` (`pkg/bookdata/wikidata_author.go`): `wbgetentities` on `cfg.searchBaseURL` via `doRequest`; body capped by `io.LimitReader` at 8 MiB. The `entities` map is keyed by the requested ID even after a redirect (the entity's own `id` is the canonical QID); a single-entity fallback covers other keyings. A non-`no-such-entity` API error maps to `ErrInvalidQuery`. P31 values are decoded leniently (raw JSON per snak, invalid shapes skipped). Verified once against live Wikidata: `Q7243` → `{QID: Q7243, Name: "Лев Николаевич Толстой", InstanceOf: [Q5]}`, `Q999999999` → `ErrNotFound`.
- Resolver refactor (`admin/service/harvest_resolve.go`): `AuthorResolution {Author, Outcome, Match, QIDBackfilled, NamesakeOf}` with exported `AuthorOutcome*` / `AuthorMatch*` constants. `resolveHarvestAuthor` = `resolveAuthorByQID` (step 1, nil when no match) + `resolveAuthorBySlug` (slug/backfill/namesake/create). For a rejected outcome `Author` is the tombstone; `ResolveOrCreateHarvestAuthors` skips it. Existing `harvest_resolve_test.go` is unchanged and green.
- `ImportWikidataAuthor` (`admin/service/author_import.go`): QID normalized and validated with its own `^Q[0-9]+$`; a fetcher `ErrInvalidQuery` also maps to `ErrInvalidQID`. Redirect: the canonical QID is looked up again in the DB, then used for slug resolution and storage. Name = trimmed ru label, refused when empty or `!IsRussianText`. Race retry: `ErrDuplicateSlug` (slug pre-check or Postgres 23505, which also covers the QID unique index) → one re-run from the QID lookup (no second Wikidata call) → `ErrDuplicateSlug` → 409. Info log per import: `qid`, `outcome`, `match`, `author_id`.
- Handler: 422 bodies are `{"error", "code"}`; 409 `author was created concurrently; retry`; 502 `wikidata is unavailable; retry later`; 503 `author import is not configured`; unknown errors → 500 `failed to import author`.
- `ListAuthorsByExternalIDs` returns `[]` without querying for empty input; all five `repository.Repository` fakes gained the method.
- Wiring: `cmd/api/main.go` builds `wikidataClient` before the admin service and passes the same instance to `WithAuthorFetcher` and to `RegisterAdminRoutes` (search), so both share one rate limiter. `RegisterAdminRoutes` passes `adminService` as the `ExistingAuthorFinder`. `cmd/worker` is unchanged.
- Frontend: `features/search-wikidata-author` (`useWikidataAuthorSearch` hook owned by the parent, exposing `refresh()` to re-run the last query, plus the `WikidataAuthorSearch` widget with `inputId`/`label`/`emptyHint`/`renderActions` props). The "Открыть" href helper `authorModerationHref` lives in `entities/author` because both features use it (no cross-feature imports). `entities/harvest` exports `HARVEST_JOBS_QUERY_KEY` (same value as the local constant in `harvest-view.tsx`, which is left unchanged). `features/import-wikidata-author`: `useImportWikidataAuthor()` → `importAuthor` / `importWithBooks` / `pendingQid`; a harvest failure after a successful import resolves with `harvest.state = 'failed'` (not a rejection), so the partial-success banner is shown and the caches are still invalidated. `lib/result-text.ts` has `importResultFeedback`, `importErrorMessage` and `harvestConfirmText`. The rejected banner uses the destructive style, and so does the harvest-failed banner. Created authors get no "Открыть" link.
- `/harvest` now also shows the candidate's QID link (part of the shared widget) besides the existing marker; its actions, chips and manual QID mode are unchanged.
- Swagger regenerated.
- Tests: `bookdata/wikidata_author_test.go`; `admin/service/author_import_test.go` (outcomes, redirect, refusals, P31 allowlist, error mapping, duplicate retry, `FindAuthorsByWikidataQIDs`); `admin/repository/author_import_sqlite_test.go`; `admin/transport/http/handler_author_import_test.go` (201/200, shape with `match`/`namesake_of` null, every error status, 401/403/bearer through the real middleware); harvest search marker tests; admin vitest: widget, result texts, import panel, moderation view toggle, harvest existing badge.
- Local DB: no test data was created (no live import against the local stack; only one live Wikidata fetch through a throwaway program, since removed).
