# Author Enrichment from Wikidata (dates + Commons photo)

Status: done

Epic: "Author moderation & import", feature 4 of 4 (final). Builds on [author-status.md](author-status.md) (status, QID), [author-moderation-queue.md](author-moderation-queue.md) (`/authors/moderation`, selection, `qid=with` filter), [author-direct-import.md](author-direct-import.md) (`FetchAuthor`, `ImportWikidataAuthor`, shared resolver), [bookdata-clients.md](bookdata-clients.md) (User-Agent, shared rate limiter) and [russian-only-content.md](russian-only-content.md). Those docs are the source of truth and are not reopened here.

## Goal

Fill empty author facts from Wikidata: day-precision birth/death dates (P569/P570, Gregorian) and a portrait (P18 → Wikimedia Commons) with licence attribution. Enrichment happens automatically for newly created Wikidata authors (direct import, harvest) and on demand from the moderation queue (single row or selected rows, which is also the backfill path). It is best-effort: it never fails an import or a harvest, and it never overwrites a value that is already set.

## Out of scope

- Biography (separate licence decision later).
- Changing an author's name, slug, status or QID (a Wikidata redirect is used for reading only, the stored QID is not rewritten).
- Preview/diff UI. The action is fill-empty-only and reports what it filled.
- A CLI backfill command. Backfill is done through the moderation-queue bulk action (decision below).
- Attribution on small avatars (literary calendar, book embeds). See Risks.
- Auto-refreshing a photo when the Commons file or its licence changes later.

## User Stories

- As an admin, when I import an author from Wikidata, the new pending author already has birth/death dates and a photo (if Wikidata has usable ones), and the result banner tells me what was added.
- As an admin, authors created by a harvest job are enriched at the end of the job, and the job report shows how many were enriched.
- As an admin, on `/authors/moderation` I can click "Обогатить из Wikidata" on a row with a QID, or select up to a page of rows and enrich them in bulk, and I see per-author what was filled or why nothing was.
- As an admin, my manual edits are never overwritten by enrichment.
- As a reader, on an author page with a Commons photo I see "Фото: <автор>, <лицензия>, Wikimedia Commons" with links to the licence and the file page.

## Business Rules

1. **Fill-empty-only.** A field is written only if it is NULL (or empty string for `photo_url`). This is enforced atomically in the SQL `UPDATE` (`COALESCE` / `CASE`), not only by a pre-read, so a concurrent admin edit is never overwritten. The photo attribution columns are written only together with `photo_url` (same `CASE` condition on `photo_url`).
2. **Dates (P569 birth, P570 death)**, decided in `bookdata` (pure function, table-tested):
   1. Consider statements with `mainsnak.snaktype == "value"` and rank ≠ `deprecated`.
   2. A value is **usable** only if `precision == 11` (day), the `time` string starts with `+` (BCE / negative years and year 0 are skipped), and month and day are non-zero.
   3. Calendar model: `http://www.wikidata.org/entity/Q1985727` (Gregorian) is used as-is. `http://www.wikidata.org/entity/Q1985786` (Julian, proleptic) is **converted to Gregorian** via Julian Day Number arithmetic (decision: convert, do not skip, because Russian classics often carry Julian-marked dates and the literary calendar works in the Gregorian calendar). Any other calendar model is skipped. `timezone`, `before`, `after` are ignored.
   4. The result must be a valid Gregorian calendar date (`time.Date` round-trip) and not in the future.
   5. Rank selection: take the highest rank group (`preferred` over `normal`) that contains at least one usable value. Within that group, if all usable values convert to the **same** date, use it. If they disagree, skip the field (`conflict`). Typical case: a Julian and a Gregorian statement for the same day both resolve to one date.
   6. If both dates result, and death < birth, drop the death date.
   7. Statements that exist but yield nothing usable set a note (`birth_date_unusable` / `death_date_unusable`). No statements → no note.
3. **Photo (P18)**:
   1. Pick the filename from the highest non-deprecated rank group, first statement in document order. `snaktype != value` is skipped.
   2. Resolve via the Commons API `imageinfo` (one call per ≤ 50 files): `thumburl` at a fixed width (constant `commonsThumbWidth`, use a standard Wikimedia thumbnail step such as 500; the page shows 200 px at 2x), `descriptionurl`, `extmetadata`.
   3. `photo_url` = `thumburl`; it must start with `https://upload.wikimedia.org/`, otherwise skip (`photo_unavailable`). Missing file / no imageinfo / no thumburl → skip (`photo_unavailable`).
   4. **Licence allowlist (decision: free licences only).** Accept only if `extmetadata.NonFree` is not `"true"` and the licence (`License` code lowercased, falling back to `LicenseShortName`) is one of: public domain (`pd`, `pd-*`, short name starting "Public domain"), `cc0`, `cc-by-<ver>`, `cc-by-sa-<ver>` (any version/jurisdiction port). Anything containing `-nc` or `-nd`, GFDL-only, unknown or missing → skip (`photo_license_not_free`).
   5. **Attribution**: `photo_author` = `Artist` with HTML tags stripped, entities unescaped, whitespace collapsed, trimmed, capped at 300 runes; empty → NULL. If the licence is CC BY / CC BY-SA and `photo_author` is NULL → skip the photo (`photo_attribution_missing`). PD/CC0 with no author is allowed. `photo_license` = `LicenseShortName` (capped at 100), `photo_license_url` = `LicenseUrl` if it is an `http(s)` URL else NULL, `photo_source_url` = `descriptionurl` (the `commons.wikimedia.org/wiki/File:` page; required, otherwise skip).
   6. Attribution text is legally required metadata and is stored verbatim (often Latin script). It is exempt from the Russian-only content policy.
4. **When enrichment runs:**
   - (a) **Direct import**: only when the outcome is `created`. Dates and the P18 filename come from the same `wbgetentities` call `FetchAuthor` already makes; one extra Commons call for the photo. `existing` / `rejected` outcomes are never enriched implicitly.
   - (b) **Harvest**: authors **created in this job** that carry a QID are collected (even if the book creation for that record later fails) and enriched once at the end of the job, in chunks of 50 QIDs: one `wbgetentities` call (`ids=Q1|Q2|…`) plus at most one Commons call per chunk. Linked pre-existing authors are not touched by harvest. Decision: `wbgetentities` batches, not SPARQL (same parser as `FetchAuthor`, gives rank/precision/calendar directly).
   - (c) **Admin action**: `POST /admin/authors/enrich-wikidata` with 1..50 author IDs, used for the single-row button and the bulk toolbar on `/authors/moderation` (any status tab). **Backfill** of existing QID authors = queue filter `qid=with`, select page (limit 50), bulk enrich. No CLI.
5. **Best-effort:** an enrichment failure (Wikidata/Commons down, rate limit, parse error, DB error on the fill update) never fails the import or the harvest; it is logged (warn) and reported. In the admin action, a failure for one author does not stop the others.
6. Authors without a Wikidata QID are skipped (`no_qid`). If the stored QID now redirects, the canonical entity's data is used; the stored QID is not changed. A missing entity → `skipped` with note `entity_not_found`.
7. **Manual photo change clears attribution:** `PATCH /admin/authors/:id` with a `photo_url` different (after normalization) from the stored one, or cleared, sets the four `photo_*` attribution columns to NULL in the same update. Attribution is not editable via the API.
8. **Etiquette:** all calls go through `cfg.doRequest` of the **same** `WikidataClient` instance (shared User-Agent, rate limiter, retries) in `cmd/api` (search + import + enrichment) and in `cmd/worker` (harvest query + enrichment). Commons uses a new configurable base URL on that client.
9. **Authorization:** the enrich endpoint is in the `authors` group (`adminRole`) + `requireBearer`. Moderators → 403, unauthenticated → 401.

## Acceptance Criteria

Backend: bookdata
- [x] `ExternalAuthorRecord` gains `BirthDate, DeathDate *time.Time` (UTC midnight, Gregorian, day precision only), `BirthDateUnusable, DeathDateUnusable bool`, `PhotoFile string` (P18 filename, `""` when none). `FetchAuthor` fills them from the same response (no new request, request params unchanged).
- [x] `(*WikidataClient).FetchAuthors(ctx, qids []string) (map[string]*ExternalAuthorRecord, error)`: 1..50 valid QIDs (else `ErrInvalidQuery`, no HTTP call), one `wbgetentities` request with `ids` joined by `|`, keyed by the **requested** QID. Missing entities are absent from the map (no error). Shares the parser with `FetchAuthor` (no duplicated entity parsing). `FetchAuthor` behavior and its existing tests are unchanged.
- [x] Claim structs gain `rank` and the time value (`time`, `precision`, `calendarmodel`); values stay lenient (malformed snak → skipped, never a fetch error).
- [x] Julian → Gregorian conversion helper (JDN based). Cases: Julian 1799-05-26 → 1799-06-06; Julian 1828-08-28 → 1828-09-09; Julian 1900-02-29 → 1900-03-13 (originally written as 03-14, corrected: see Implementation Notes); Gregorian values unchanged.
- [x] Date selection per rule 2: precision 10/9 ignored; BCE ignored; deprecated ignored; preferred beats normal; preferred group with only imprecise values falls through to normal; conflicting dates in the chosen group → nil + unusable flag; Julian + Gregorian same day → that date; unknown calendar model skipped; invalid day (Gregorian 1900-02-29) skipped; death < birth → death dropped.
- [x] New option `WithCommonsBaseURL(url)` (default `https://commons.wikimedia.org/w/api.php`).
- [x] `(*WikidataClient).FetchCommonsImages(ctx, filenames []string) (map[string]*CommonsImage, error)`: 1..50 names, one `action=query&prop=imageinfo&iiprop=url|extmetadata&iiurlwidth=<commonsThumbWidth>&iiextmetadatafilter=Artist|LicenseShortName|License|LicenseUrl|NonFree&titles=File:…|File:…&format=json&formatversion=2` request via `doRequest`; maps `query.normalized` back so the result is keyed by the **input** filename; missing files absent. Body through `io.LimitReader`.
- [x] `CommonsImage{ThumbURL, PageURL, Author, License, LicenseURL string; Free bool}` where `Free` and the cleaned `Author` are computed per rule 3.4–3.5 (pure helpers, table-tested). Errors map like `SearchAuthors` (400 → `ErrInvalidQuery`, ≥ 400 → `ErrSourceUnavailable`, 429/5xx via `doRequest`).

Backend: DB / domain
- [x] Migration `000016_authors_photo_attribution` (up/down) adds nullable `photo_author TEXT`, `photo_license VARCHAR(100)`, `photo_license_url TEXT`, `photo_source_url TEXT`. Applied with `migrate up` only.
- [x] `catalogDomain.Author` gains `PhotoAuthor, PhotoLicense, PhotoLicenseURL, PhotoSourceURL *string`.

Backend: admin service
- [x] New interface `AuthorEnrichmentSource { FetchAuthors(...); FetchCommonsImages(...) }` (satisfied by `*bookdata.WikidataClient`) and option `WithAuthorEnrichmentSource(src)`.
- [x] New file `admin/service/author_enrich.go`: `EnrichAuthorsFromWikidata(ctx, ids []uuid.UUID) (*AuthorEnrichmentReport, error)`: dedupes, loads authors, skips `no_qid`, chunks QIDs by 50, fetches entities then Commons for chosen photo files, computes the fill set per author against the pre-read values, applies one repository fill update per author, and reports per item. Chunk-level fetch failure → those items `error` (`wikidata_unavailable` / `commons_unavailable` for photo only, dates still applied), other chunks continue. Nil source → `ErrAuthorEnrichmentUnavailable`.
- [x] Internal `enrichFromRecord(ctx, author, record)` used by import (record already fetched) and by the batch path, so fill logic exists once.
- [x] `ImportWikidataAuthor`: on outcome `created`, calls enrichment with the fetched record (+ Commons for the photo if a source is configured; without a source only dates are applied). Enrichment errors are logged and returned in `AuthorImportResult.Enrichment` (`filled`, `notes`), never as the import error. `existing`/`rejected` → `Enrichment == nil`.
- [x] Harvest resolver: new `ResolveHarvestAuthorRefs(ctx, refs) (HarvestAuthorsResolution{IDs []string; CreatedWithQID []uuid.UUID}, error)`; `ResolveOrCreateHarvestAuthors` becomes a thin wrapper returning `IDs`. **All existing `harvest_resolve_test.go` tests pass unchanged.**
- [x] `UpdateAuthor` clears the four attribution fields when the normalized `photo_url` changes (rule 7); unchanged photo keeps them.
- [x] Info log per enriched author (`author_id`, `qid`, `filled`, `notes`); warn on chunk failures.

Backend: admin repository
- [x] `FillAuthorEnrichment(ctx, id, AuthorEnrichmentFill{BirthDate, DeathDate *time.Time; Photo *PhotoFill})` runs one `UPDATE authors SET birth_date = COALESCE(birth_date, ?), death_date = COALESCE(death_date, ?), photo_url = CASE WHEN photo_url IS NULL OR photo_url = '' THEN ? ELSE photo_url END, photo_author = CASE WHEN <same condition> THEN ? ELSE photo_author END, … , updated_at = ? WHERE id = ?` (portable to SQLite). Only non-nil parts are included.
- [x] `GetAuthorsByIDs(ctx, ids)` (one query) if no equivalent exists. All repository fakes gain the new methods.
- [x] Explicit column lists (e.g. `author_moderation.go` select) include the new columns where the admin response exposes them.

Backend: harvest
- [x] `processRecord` reports created-with-QID author IDs as soon as authors are resolved (even if `CreateBook` then fails); `runHarvest` accumulates them and, after the record loop, calls `admin.EnrichAuthorsFromWikidata` once (best-effort). `HarvestResult` gains `authors_enriched` (int) and `author_enrichment_errors` (int); a final `reportProgress` is written after enrichment. Enrichment errors never change the task's return value. If the service has no enrichment source, the step is skipped (no error).
- [x] `cmd/worker/main.go`: create `wikidataClient` before the admin service and pass the same instance to `WithAuthorEnrichmentSource` and `harvestservice.NewHandler`. `cmd/api/main.go`: pass the existing `wikidataClient` to `WithAuthorEnrichmentSource` (same instance as search/import).

Backend: HTTP
- [x] `POST /api/v1/admin/authors/enrich-wikidata` per the API section. Swagger annotations + regenerate.
- [x] Import response gains `enrichment`.
- [x] Admin `authorResponse` gains `photo_attribution` (object or null).
- [x] Public `GET /authors/:slug` (`authorDetailResponse`) gains `photo_attribution` (object or null, present only when `photo_url` and `photo_source_url` are both set). Book embeds / list items / calendar unchanged.

Frontend
- [x] `apps/web` author page shows the attribution line under the photo when `photoAttribution` is present; links open in a new tab with `rel="noopener noreferrer"`; plain `<a>` (external, no typedRoutes involvement).
- [x] `apps/web` renders `upload.wikimedia.org` photos correctly: `next.config.js` `remotePatterns` += `{ protocol: 'https', hostname: 'upload.wikimedia.org', pathname: '/wikipedia/commons/**' }`, and photos from that host are rendered with `unoptimized` (browser hotlinks the thumb directly; avoids the Next optimizer fetching Wikimedia without a policy-compliant User-Agent). Apply to every `next/image` that renders `photoUrl` (author page, calendar `AuthorAvatar` if it uses `next/image`).
- [x] `apps/admin` moderation queue: per-row "Обогатить из Wikidata" (only rows with `external_id`), bulk toolbar "Обогатить из Wikidata (N)" (only QID rows are sent; if none, button disabled), result banner, invalidations.
- [x] `apps/admin` import panel banner appends the enrichment summary for `created`.
- [x] `apps/admin` author edit form shows the attribution read-only under the photo URL field, and a hint "При смене фото атрибуция будет удалена" when the field differs from the loaded value.
- [x] `apps/admin` harvest job report shows "Авторов обогащено из Wikidata: N" (and errors count if > 0) when the fields are present.

## UX

- Row button: "Обогатить из Wikidata", no confirm (non-destructive, fill-empty-only). Disabled while pending.
- Bulk: "Обогатить из Wikidata (N)", no confirm. Rows without QID in the selection are not sent; banner says "Без QID пропущено: K".
- Result banner (single): «Автор «X»: добавлены дата рождения, дата смерти, фото.» / «Автор «X»: нечего добавить — поля уже заполнены или в Wikidata нет подходящих данных.» + notes mapped to Russian:
  - `birth_date_unusable` / `death_date_unusable`: «дата рождения/смерти в Wikidata неточная или противоречивая — пропущена»
  - `photo_license_not_free`: «фото под несвободной лицензией — пропущено»
  - `photo_attribution_missing`: «у фото не указан автор — пропущено»
  - `photo_unavailable`: «файл фото недоступен на Commons»
  - `entity_not_found`: «элемент Wikidata не найден»
  - `no_qid`: «у автора нет QID»
- Bulk banner: «Обогащено X из N» + first 3 per-author errors/notes. 502: «Wikidata недоступна, попробуйте позже.» 503: «Обогащение не настроено.»
- Field labels in messages: `birth_date` → «дата рождения», `death_date` → «дата смерти», `photo` → «фото».
- Web attribution line (small muted text under the photo): `Фото: {author}, {license}, Wikimedia Commons` where `{license}` links to `license_url` (plain text if null) and "Wikimedia Commons" links to `source_url`. Without author: `Фото: {license}, Wikimedia Commons`.
- Admin form read-only block: same text, plus a link to the Commons file page.

## API

`POST /api/v1/admin/authors/enrich-wikidata` (admin role + bearer)
- Request: `{"ids": ["<uuid>", ...]}`. 1..50 valid UUIDs, deduped (400 otherwise).
- 200:
```json
{
  "processed": 3,
  "enriched": 1,
  "items": [
    { "author_id": "…", "name": "…", "outcome": "enriched | unchanged | skipped | error",
      "filled": ["birth_date", "death_date", "photo"], "notes": ["photo_license_not_free"], "message": "" }
  ]
}
```
  `outcome`: `enriched` (≥ 1 field filled), `unchanged` (fetched, nothing to fill), `skipped` (`no_qid`, `entity_not_found` in `notes`), `error` (`message` set). Unknown ID → `error` "author not found".
- 502 only when the `wbgetentities` call for the whole request failed (rate limited / unavailable) and nothing was processed; 503 source not configured; 400; 401; 403.

`POST /api/v1/admin/authors/import-wikidata`: response gains `"enrichment": {"filled": [...], "notes": [...], "error": "" } | null` (null unless outcome `created`).

Admin `authorResponse` (all admin author payloads) and public `GET /api/v1/authors/:slug` gain:
```json
"photo_attribution": { "author": "Sergey Prokudin-Gorsky" , "license": "Public domain", "license_url": null, "source_url": "https://commons.wikimedia.org/wiki/File:…" } | null
```
`author` may be null. Public: always present on the detail response (null when absent).

Harvest job result (`GET` harvest job endpoints): gains `authors_enriched`, `author_enrichment_errors` (numbers; absent in older job results).

## Database

`backend/migrations/000016_authors_photo_attribution.up.sql`:
```sql
ALTER TABLE authors ADD COLUMN photo_author TEXT;
ALTER TABLE authors ADD COLUMN photo_license VARCHAR(100);
ALTER TABLE authors ADD COLUMN photo_license_url TEXT;
ALTER TABLE authors ADD COLUMN photo_source_url TEXT;
```
`.down.sql` drops them in reverse order (provided for completeness; never run it, apply with `migrate up` only). No indexes. Existing authors keep NULL attribution (manually set photos have none, which is correct).

## Frontend

`packages/api-client/src/index.ts`
- `AuthorPhotoAttribution { author: string | null; license: string; license_url: string | null; source_url: string }` (web entity maps to camelCase like the existing author mapper).
- Public author detail type += `photo_attribution: AuthorPhotoAttribution | null`; `AdminAuthor` += same.
- `AdminAuthorEnrichmentReport`, `AdminAuthorEnrichmentItem`; `AdminAuthorImportResult.enrichment: { filled: string[]; notes: string[]; error: string } | null`.
- `api.admin.authors.enrichWikidata(ids: string[])` → `POST /api/v1/admin/authors/enrich-wikidata`.
- Harvest job result type += optional `authors_enriched?`, `author_enrichment_errors?`.

`apps/web` (FSD)
- `entities/author`: map `photo_attribution` → `photoAttribution`; new `entities/author/ui/photo-attribution.tsx` (`PhotoAttribution`), exported from the slice.
- `views/author-detail/ui/author-detail-view.tsx`: render `PhotoAttribution` under the photo block; `unoptimized` for Wikimedia URLs (small helper `isWikimediaUrl` in `entities/author/lib` or `shared/lib`).
- `next.config.js`: remote pattern (see AC). CI frontend build is the gate (no native `next build`).

`apps/admin` (FSD)
- `entities/author/api`: re-export `enrichWikidata`. `entities/author/ui/photo-attribution-text.tsx` (read-only text, reused by the form).
- `features/enrich-author/` (new): `useEnrichAuthors()` (mutation, invalidates `['admin','authors']`), `lib/enrich-text.ts` (pure builders: field labels, note texts, single/bulk banner, import enrichment suffix). Imports `entities/author` only.
- `views/author-moderation/ui/author-moderation-view.tsx`: row action + bulk toolbar button + banner via the feature.
- `features/import-wikidata-author/lib/result-text.ts`: append the enrichment suffix for `created` using the builder from `entities/author/lib` (move the field/note label maps there so both features use them without cross-feature imports).
- `features/manage-author/ui/author-form.tsx`: read-only attribution + change hint.
- Harvest job report component: the new counters line.

## Existing Patterns

- `pkg/bookdata/wikidata_author.go` (`FetchAuthor`, lenient raw-JSON snak decoding, `LimitReader`) and `wikidata_search.go` (status mapping, `WithSearchBaseURL` tests).
- `admin/service/author_import.go` (optional source via `Option`, sentinel → status mapping), `author_moderation.go` / `bulk_publish.go` (per-row continue-on-error report, cap constant, one file per action).
- `admin/service/harvest_resolve.go` (thin-wrapper refactor style used in feature 3).
- `harvest/service/handler.go` `enrichRecord` (best-effort, never fatal) and `reportProgress`.
- Moderation view feedback banner + `features/moderate-author` mutation hook + pure text builders.

## Tests

Backend (via `docker compose run`; never `migrate down` or reset the DB; no live Wikidata/Commons: fake `RoundTripper` via `WithHTTPClient` or `httptest` via `WithSearchBaseURL`/`WithCommonsBaseURL`, instant limiter):
- `bookdata` dates: every rule 2 case from the AC as a table test on the pure helper; JDN conversion cases.
- `bookdata` `FetchAuthor`: existing tests unchanged; new fixture with P569/P570/P18 parses the new fields.
- `bookdata` `FetchAuthors`: `ids` joined in one request; >50 / invalid → `ErrInvalidQuery` with no call; missing entity absent; redirect keyed by requested QID; User-Agent asserted.
- `bookdata` `FetchCommonsImages`: normalized title mapping (spaces/underscores); missing file absent; licence allowlist table (PD, `pd-old-70`, CC0, CC BY 4.0, CC BY-SA 3.0 `de` port, CC BY-NC → not free, CC BY-ND → not free, GFDL → not free, `NonFree=true` → not free, missing → not free); Artist HTML stripping/unescape/cap; non-`upload.wikimedia.org` thumb rejected; 5xx → `ErrSourceUnavailable`.
- `admin/service` `EnrichAuthorsFromWikidata` (fakes): fills only empty fields; non-empty birth date kept while photo filled; manual photo kept (attribution untouched); no QID → skipped; entity missing → skipped; CC BY without author → photo skipped with note; Commons failure → dates applied, photo note/error; entity fetch failure → items error and source error surfaced; >50 ids → sentinel; dedupe; unknown ID → row error; 60 IDs from harvest → two chunks.
- `admin/service` `ImportWikidataAuthor`: `created` → enrichment applied and reported; enrichment failure → import still `created` with `enrichment.error`; `existing` → no enrichment call; nil enrichment source → dates only.
- `admin/service` `UpdateAuthor`: photo changed/cleared → attribution cleared; unchanged → kept.
- Resolver: existing tests unchanged; `ResolveHarvestAuthorRefs` returns only created-with-QID IDs.
- `admin/repository` SQLite: `FillAuthorEnrichment` never overwrites non-NULL values (including a photo set between read and update), writes attribution only with the photo; SQLite test schemas updated with the new columns.
- `harvest/service`: created authors enriched once after the loop; enrichment error does not fail `runHarvest`; created author whose book creation failed is still enriched; counters in the result; nil source → skipped.
- HTTP: enrich endpoint 200 shape / 400 / 401 / 403 / 502 / 503 through the real middleware; import response `enrichment`; public author detail `photo_attribution` present/null; admin `photo_attribution`.

Frontend (vitest):
- web: `PhotoAttribution` with/without author, with/without licence URL; author detail renders it only when present.
- admin: `enrich-text` builders (every note/field, single/bulk); moderation view row button only for QID rows, bulk sends only QID IDs, banner text; import result text with enrichment suffix; author form read-only attribution and change hint; harvest report counters.

## Documentation

- This file → `Status: done` + Implementation Notes on completion.
- `author-direct-import.md`, `author-status.md`: one-line pointer that feature 4 shipped (enrichment on create).
- `bookdata-clients.md`: `FetchAuthors`, `FetchCommonsImages`, `WithCommonsBaseURL`, shared limiter covers Commons.
- `russian-only-content.md`: note that photo attribution is exempt (legally required verbatim metadata).
- `docs/ROADMAP.md`: tick the epic (feature 4 of 4).
- Swagger regenerated.

## Risks

- **Julian-date semantics.** The contract assumes the `wbgetentities` JSON `time` is expressed in the calendar given by `calendarmodel` (Wikibase JSON model; the SPARQL/RDF export normalizes to Gregorian, the JSON API does not). Verify once manually against live Wikidata on a Russian classic with a Julian-marked statement before relying on it; the conversion is isolated in one tested helper.
- **Conflicting statements** (disputed dates) are skipped rather than guessed. Some authors stay without dates; the admin can fill them manually.
- **Licence compliance is minimum viable.** Attribution is shown on the author page only. The literary calendar avatars and any other small thumbnails reuse the photo without an attribution line; they link to the author page where attribution is shown. Licences other than PD/CC0/CC BY/CC BY-SA are skipped. Commons metadata can be wrong or change later; there is no re-check.
- **Hotlinking / Wikimedia thumbnail policy.** Photos are hotlinked from `upload.wikimedia.org` with `unoptimized` images. Non-standard thumbnail widths may be throttled by Wikimedia; use a standard step for `commonsThumbWidth`. If Wikimedia changes thumb URLs, stored URLs may break (admin can replace the photo).
- **Harvest enrichment runs at the end of the job.** If the task times out or is retried after authors were created, the retry sees them as `existing` and does not enrich them. Covered by the moderation-queue bulk action (`qid=with`).
- **Extra latency** on the import click (one Commons call) and on the bulk action (≤ 2 rate-limited calls per 50 authors). Acceptable for admin actions.
- **Admin edit race:** the fill update is atomic per column, so a concurrent manual edit wins; the per-item report may then claim a field as filled that was actually kept. Cosmetic.
- **Admin PATCH is full replace:** clearing attribution relies on comparing normalized `photo_url`; any client that re-sends the same URL keeps attribution (intended).
- **Cache staleness:** the home literary calendar / daily home cache picks up new dates/photos only after refresh (accepted earlier). Newly dated published authors start appearing in the calendar after enrichment, which is the intended effect.
- **Shared rate limiter:** `cmd/worker` must pass the same `WikidataClient` instance to the harvest handler and the admin service (currently built after the admin service; reorder). A second instance would double the outbound rate. Verify in review.
- **SQLite test schemas** are hand-written in several repository tests; they must gain the new columns or GORM selects will fail.

## Implementation Notes

### Live verification (October 2026)

- `wbgetentities` for **Q7243** (Leo Tolstoy): P569 has a `preferred` statement `{"time": "+1828-08-28T00:00:00Z", "precision": 11, "calendarmodel": "http://www.wikidata.org/entity/Q1985786"}` (Julian) and a `normal` one `+1828-09-09…` with `Q1985727` (Gregorian). P570: `preferred` Julian `+1910-11-07`, `deprecated` Gregorian `+1910-11-10`, `normal` Gregorian `+1910-11-20`. So the JSON `time` **is expressed in the calendar named by `calendarmodel`**, as the contract assumed. The real client returned birth 1828-09-09, death 1910-11-20 (preferred Julian values converted).
- **Q7200** (Pushkin; Q8862 is a different item): P569 `preferred` Julian `+1799-05-26` → 1799-06-06; the `normal` group holds conflicting values (Julian 05-26 and 05-27, Gregorian 06-06, plus a precision-9 year) but is not used because `preferred` wins. P570 `preferred` Julian `+1837-01-29` → 1837-02-10.
- Commons `imageinfo` for `L.N.Tolstoy Prokudin-Gorsky.jpg` and `Kiprensky Pushkin.jpg`: `extmetadata` values are `{"value": "...", "source": ...}`; `License` = `pd`, `LicenseShortName` = `Public domain`, no `LicenseUrl`, no `NonFree` key; `Artist` is HTML (`<bdi><a …><span …>Sergei Prokudin-Gorskii</span></a></bdi>` → cleaned `Sergei Prokudin-Gorskii`; `Orest Kiprensky`). A CC example (`Montbolo 001.jpg`) gives `License` = `cc-by-sa-3.0`, `LicenseShortName` = `CC BY-SA 3.0`, `LicenseUrl` = `https://creativecommons.org/licenses/by-sa/3.0`. Titles with underscores come back in `query.normalized`.
- **Deviation (thumbnail host):** Commons now returns `thumburl` on **`thumb.wikimedia.org`** with tracking parameters (`…/500px-X.jpg?utm_source=commons.wikimedia.org&utm_campaign=imageinfo&utm_content=thumbnail`), not on `upload.wikimedia.org`. Both hosts serve the same path (checked with HEAD: 200 image/jpeg, identical size). The rule "must start with `https://upload.wikimedia.org/`" is therefore implemented as: `https`, host `upload.wikimedia.org` **or** `thumb.wikimedia.org`, path under `/wikipedia/commons/`; query string and fragment are dropped. The host is kept as returned. `apps/web` `remotePatterns` and `isWikimediaUrl` include both hosts.
- **Batch `no-such-entity`:** a `wbgetentities` request with one non-existent ID fails as a whole (`{"error": {"code": "no-such-entity", "id": "Q…"}}`); it does not return a `missing` entry. `FetchAuthors` drops the named ID and re-requests the rest (one extra rate-limited call per missing ID). Verified live with `Q7243|Q7200|Q999999999999`.
- All live calls were made with curl and one throwaway Go program (`cmd/tmplivecheck`, deleted). No local DB data was created or changed by the checks (the local DB had no QID authors).

### Deviations and decisions

- **Contract test case corrected:** Julian 1900-02-29 is Gregorian **1900-03-13**, not 03-14. The Julian–Gregorian offset is still 12 days on Julian 1900-02-29 and becomes 13 days from Julian 1900-03-01 (= Gregorian 1900-03-14). The table test uses 03-13 and adds Julian 1900-03-01 → 03-14.
- **Two service entry points:** `EnrichAuthorsFromWikidata` (admin action: 1..50 deduped IDs, `ErrAuthorEnrichmentTooLarge` / `ErrAuthorEnrichmentEmpty` → 400) and `EnrichHarvestAuthors` (harvest: no cap, chunked by 50). Both share `enrichAuthors`. This resolves the contract's ">50 ids → sentinel" vs "60 IDs from harvest → two chunks".
- `bookdata.CommonsImage` gains `AttributionRequired bool` (CC BY / CC BY-SA), so the service applies the `photo_attribution_missing` rule without re-parsing the licence. `ThumbURL` / `PageURL` are `""` when unusable (→ `photo_unavailable`).
- Extra notes beyond the contract list: `wikidata_unavailable` (entity fetch failed for the chunk, item `error`) and `commons_unavailable` (Commons failed, dates still applied). A dropped death date (death before birth, also against an existing birth date) reports `death_date_unusable`. Statements that exist only as `deprecated` also set the `*_unusable` note.
- 502: the service returns the source error (with the report) when every attempted `wbgetentities` chunk failed; the handler maps `ErrRateLimited` / `ErrSourceUnavailable` to 502. Any other all-chunks failure returns 200 with per-item errors.
- `HarvestResult.authors_enriched` / `author_enrichment_errors` are `*int` with `omitempty`: absent while the record loop runs, in older results and when no enrichment source is configured; set (also 0) once the end-of-job step ran. A total enrichment failure counts every created QID author as an error. The step uses the task context.
- Import without an enrichment source applies only the dates (no photo, no note). A Commons failure on import adds `commons_unavailable` to `enrichment.notes` (`error` stays empty); `enrichment.error` is set only when the fill update fails.
- `FillAuthorEnrichment` builds one `UPDATE` with `COALESCE` for the dates and `CASE WHEN photo_url IS NULL OR photo_url = '' THEN ? ELSE <col> END` for `photo_url` and the four attribution columns (SET expressions see the pre-update row on Postgres and SQLite). `GetAuthorsByIDs` loads authors with one `IN` query per chunk.
- `UpdateAuthor` compares trimmed `photo_url` values (nil and `""` are equal); a change or clear sets the four attribution fields to NULL (the update is a full `Save`).
- Admin `authorResponse.photo_attribution` and public detail `photo_attribution` are non-null only when `photo_url` and `photo_source_url` are both set.
- Admin texts: the label/note maps and the import suffix live in `entities/author/lib/enrichment-text.ts`; `features/enrich-author` holds `useEnrichAuthors` and the single/bulk/error builders. Single row: «Автор «X»: добавлены …» / «… нечего добавить …» + «Примечания: …»; skipped: «Автор «X»: пропущен — …»; error: «Автор «X»: не удалось обогатить — …». Bulk: «Обогащено X из N.» + «Без QID пропущено: K.» + the first three items with an error, skip or note. The bulk button is shown in every status tab and is disabled when no selected row has a QID; enrich buttons are disabled while an enrichment request is pending.
- Import banner suffix for `created`: « Из Wikidata добавлены: …» / « Из Wikidata ничего не добавлено.» + « Примечания: …»; on a save error « Данные из Wikidata не сохранены: …».
- Admin author form: read-only «Фото: {author}, {license}, Wikimedia Commons · Страница файла» under the photo URL field; the hint «При смене фото атрибуция будет удалена» appears only for an attributed photo whose URL field differs from the loaded value.
- Harvest page: a completed job with the counters shows a second line «Авторов обогащено из Wikidata: N» (+ «, ошибок обогащения: M» when > 0) under the existing summary.
- Web: `entities/author` exports `PhotoAttribution` and `isWikimediaUrl`; the author page wraps the photo and the attribution in one grid cell and renders Wikimedia photos `unoptimized`. The literary-calendar `AuthorAvatar` is a plain `<img>`, so no change there. `api.authors.get` maps `photo_attribution` → `photoAttribution` (null when absent).
- Wiring: `cmd/worker` builds `wikidataClient` before the admin service and passes the same instance to `WithAuthorEnrichmentSource` and `harvestservice.NewHandler`; `cmd/api` passes its existing `wikidataClient` (search + import) to `WithAuthorEnrichmentSource`.
- Migration `000016_authors_photo_attribution` applied locally with `migrate up`. The SQLite test schemas that have author photo columns (`admin/repository/author_moderation_sqlite_test.go`, `catalog/repository/author_status_sqlite_test.go`) gained the four columns; the minimal merge-QID schema has no photo columns and is unchanged.
- Swagger regenerated.

### Tests

- `bookdata`: `wikidata_dates_test.go` (JDN cases, every rule 2 case, P18 selection, death before birth), `wikidata_authors_test.go` (FetchAuthor dates/photo from a Q7243-shaped fixture, FetchAuthors batching via httptest, redirect keying, missing entity, no-such-entity retry, invalid input without HTTP, 5xx), `commons_test.go` (normalized titles, missing file, thumb host/query handling, licence allowlist table, Artist cleaning, status mapping). Existing `FetchAuthor` tests unchanged.
- `admin/service`: `author_enrich_test.go`, `author_enrich_import_test.go` (fill rules, notes, Commons/entity/DB failures, limits, 60 IDs → two chunks, import created/existing/no source/failure, `UpdateAuthor` attribution clearing, `ResolveHarvestAuthorRefs`). Existing `harvest_resolve_test.go` unchanged and green.
- `admin/repository`: `author_enrich_sqlite_test.go` (fill-empty-only incl. a manual photo set between read and update, attribution only with the photo, empty-string photo, `GetAuthorsByIDs`).
- HTTP: `handler_author_enrich_test.go` (200 shape, 400/502/503/500, 401/403/bearer through the real middleware, import `enrichment` and admin `photo_attribution`), catalog `handler_photo_attribution_test.go`.
- `harvest/service`: `handler_enrich_test.go` (one batched enrichment after the loop incl. an author whose book failed, counters in the final progress write, failure does not fail the job, nil source skips).
- Frontend: web `photo-attribution.test.tsx`, `is-wikimedia-url.test.ts`, `author-detail-view.test.tsx`, api mapping test; admin `enrichment-text.test.ts`, `enrich-text.test.ts`, moderation view (row button only for QID rows, bulk sends only QID IDs, banners, 502), import result text with the suffix, `author-form.test.tsx`, harvest counters.
