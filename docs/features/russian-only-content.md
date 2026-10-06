# Russian-Only Catalog Content

Status: implemented

Product owner request (translated): "When adding books, genres, and other catalog information, only Russian-language content should be added." This contract resolves what that means concretely across the three existing content-entry paths — manual admin creation, bulk CSV/JSON import (`docs/features/catalog-manual-import.md`), and the async Wikidata harvest pipeline (`docs/features/async-harvest-queue.md`, `harvest-review-queue.md`, `admin-harvest-trigger.md`, `harvest-author-search.md`) — and the two entities named directly (`Genre`) and indirectly (`Author`).

## Scope boundaries (read first)

**Core interpretation decision — read this before anything else below.** "Russian-only" is **not** implemented as `Book.Language == "ru"`. Investigation of `backend/cmd/seed/main.go` shows `Language` already legitimately holds non-`"ru"` values (`"en"` for Huxley's *Brave New World*, Remarque's *All Quiet on the Western Front*, Orwell's *Animal Farm*) on books whose `Title`/`Description` are themselves in Russian — `Language` records the **original language of the work**, a distinct, pre-existing, intentional catalog concept with its own dedicated filter UI (`apps/web/src/widgets/catalog-filters/ui/filters/language-filter.tsx`, `GET /api/v1/languages`). Hard-requiring `Language == "ru"` would reject exactly the kind of foreign-classic-with-Russian-metadata entries the catalog already treats as first-class content. **`Language` validation (`supportedLanguageCodes` in `admin/service/service.go`) is unchanged by this contract.**

Instead, "Russian-only" is implemented as **actual text-language enforcement** on the catalog content itself — `Book.Title` (always) and `Book.Description` (when present) — using a pragmatic Cyrillic-character-ratio heuristic (see Business Rules). A full NLP language-detection library is judged disproportionate for this monorepo (no existing NLP dependency, Go ecosystem options are either CGO-heavy or large model-based); the heuristic is the deliberate MVP choice, not an open question.

**Genres/Authors are scoped out of the enforcement mechanism for this increment, not out of the request.** Verified against actual code:
- `backend/cmd/seed/main.go`'s five genres and all authors are Russian by construction (`Классика`, `Роман`, ..., `Лев Толстой`, `Фёдор Достоевский`, ...).
- `harvest_resolve.go`'s `ResolveOrCreateAuthorIDsByNames`/`ResolveOrCreateGenreIDsByNames` (the harvest pipeline's only way to create authors/genres) only ever receive names pulled from Wikidata `authorLabel`/`genreLabel` SPARQL bindings — which this contract changes to `ru`-only (see below), making harvested author/genre names Russian by construction too.
- `harvest-author-search.md`'s `SearchAuthors` already calls Wikidata's `wbsearchentities` with `language=ru&uselang=ru` — Russian-scoped by construction already.

The one gap this leaves is an admin manually typing a non-Russian `Author.Name`/`Genre.Name` directly into the admin panel forms — judged low-frequency/low-risk given the above, and flagged as an explicit follow-up in Risks rather than built here. `Author.Biography` is also left unvalidated (edge cases like quoted foreign titles/names are common and legitimate in a biography).

**Explicitly not touched:** the SPARQL class filter for literary works (`wd:Q7725634`), `BuildAuthorListQuery`'s QID `VALUES` clause construction, the curated-author-QID safety model, the admin-harvest-trigger/harvest-progress/harvest-author-search UI (no new UI surface — harvest failures continue to surface via the existing `HarvestResult.Errors` mechanism), Open Library's API itself (not queryable by language — this contract only changes how its response is used), `Language` field validation/`supportedLanguageCodes`, and the `/languages` filter feature.

## User Stories

- As a content admin, when I (or bulk-import CSV/JSON, or the harvest pipeline) try to save a book whose title is not in Russian, the system rejects it with a clear reason instead of silently adding non-Russian catalog content.
- As a content admin reviewing a bulk import report or a harvest job's result, a row/record rejected for non-Russian content shows up as a per-row/per-record error, exactly like any other validation failure — it doesn't abort the rest of the batch/job.
- As a site visitor, I no longer see catalog entries whose title is in a script/language I can't read, even though I can still find foreign classics by their Russian-translated titles and metadata (original-language filter is unaffected).

## Acceptance Criteria

**Backend — shared Russian-text check (`backend/internal/modules/admin/service`)**
- [ ] New file `backend/internal/modules/admin/service/language.go` (or add to `service.go` — developer's call, minimize churn) exporting:
  ```go
  const cyrillicRatioThreshold = 0.5

  // IsRussianText reports whether s appears to be written in Russian, via a
  // pragmatic Cyrillic-character-ratio heuristic: among s's letters
  // (unicode.IsLetter), at least cyrillicRatioThreshold must be Cyrillic
  // (unicode.Is(unicode.Cyrillic, r)). Non-letter runes (digits, punctuation,
  // whitespace) are ignored and don't count toward the ratio. A string with
  // no letters at all (empty, or all digits/punctuation) passes — there is no
  // text to judge, so it is not rejected on this basis.
  //
  // Exported so harvest/service can pre-filter enrichment text (see
  // handler.go's enrichRecord) before it ever reaches CreateBook.
  func IsRussianText(s string) bool
  ```
- [ ] New sentinel errors alongside the existing `ErrInvalid*` block: `ErrTitleNotRussian = errors.New("title must be in Russian")`, `ErrDescriptionNotRussian = errors.New("description must be in Russian")`.
- [ ] `validateBookInput` (used by both `CreateBook` and `UpdateBook` — covers manual admin, bulk import, and harvest, since all three funnel through these two methods) gains, after the existing title-required check: `if !IsRussianText(strings.TrimSpace(input.Title)) { return ErrTitleNotRussian }`; and, only when a description is actually provided: `if desc := normalizeOptionalString(input.Description); desc != nil && !IsRussianText(*desc) { return ErrDescriptionNotRussian }` (reuse the existing `normalizeOptionalString` helper for trimming/nil-handling consistency).
- [ ] `Language`/`supportedLanguageCodes` validation is unchanged — no new restriction there (see Scope boundaries).
- [ ] `Author`/`Genre` validation (`validateAuthorInput`, `validateGenreInput`) is unchanged in this increment — no `IsRussianText` check added to `Name`/`Biography` (see Scope boundaries, Risks).

**Backend — error → HTTP status mapping (`backend/internal/modules/admin/transport/http/handler.go`)**
- [ ] Add `errors.Is(err, adminService.ErrTitleNotRussian)` and `errors.Is(err, adminService.ErrDescriptionNotRussian)` to the existing `case` list that maps to `422 Unprocessable Entity` in both `CreateBook` (~line 465) and `UpdateBook` (~line 531) handlers — matching exactly how `ErrInvalidTitle`/`ErrInvalidLanguage` are already mapped there. No new status code, no new response shape.

**Backend — harvest pipeline: source-level Russian-label filter (`backend/internal/pkg/bookdata/wikidata.go`)**
- [ ] In `buildTitleAuthorQuery`, `buildISBNQuery`, and `BuildAuthorListQuery`: change every `FILTER(LANG(?itemLabel) IN ("ru","en"))` → `FILTER(LANG(?itemLabel) = "ru")`, every `FILTER(LANG(?authorLabel) IN ("ru","en"))` → `FILTER(LANG(?authorLabel) = "ru")`, and every `FILTER(LANG(?genreLabel) IN ("ru","en"))` → `FILTER(LANG(?genreLabel) = "ru")`. This applies to the work's own label (`itemLabel`, becomes `Book.Title`), the author's label (`authorLabel`, becomes `Author.Name` via `ResolveOrCreateAuthorIDsByNames`), and the genre's label (`genreLabel`, becomes `Genre.Name` via `ResolveOrCreateGenreIDsByNames`) — all three feed catalog text directly, so all three get the same fix, not just the title.
- [ ] `FILTER(LANG(?languageLabel) = "en")` (used only to map Wikidata's P407 "language of work" property to an ISO code via `wikidataLanguageCodes`, feeding `Book.Language` — the original-language metadata, not catalog text) is **unchanged** — out of scope per the Scope boundaries decision.
- [ ] Update the doc comments on `buildTitleAuthorQuery`/`buildISBNQuery`/`BuildAuthorListQuery` that currently say "(ru or en) label" / "ru/en label-preference idiom" to reflect the new ru-only behavior.
- [ ] Accepted tradeoff, documented in the doc comments and in Risks: fewer harvestable items (any work with no Russian-language Wikidata label is now skipped entirely, where it previously fell back to an English label) — an explicit, accepted reduction in catalog coverage in exchange for not harvesting non-Russian titles/author names/genre names in the first place.

**Backend — harvest pipeline: language defaulting + description pre-filtering (`backend/internal/modules/harvest/service/handler.go`)**
- [ ] `processRecord`: add a small local default, mirroring `import.go`'s existing `defaultImportLanguage` pattern exactly (same rationale: `Language` is required by `validateBookInput` and must not be empty): `language := strings.TrimSpace(enriched.Language); if language == "" { language = "ru" }` (a literal `"ru"` with a comment referencing `defaultImportLanguage`'s precedent is fine — don't introduce a shared cross-package constant for one literal). Use `language` (not `enriched.Language`) when building `BookInput`. This is a necessary companion fix: today, `enriched.Language` is frequently empty (Wikidata's P407 property is sparsely populated) and `CreateBook` already hard-rejects an empty `Language` (`ErrInvalidLanguage`) — without this default, most harvested records would fail validation for an unrelated reason, making this contract's new Title check impossible to verify against real harvest behavior. When Wikidata *does* supply a P407-derived language (e.g. `"en"` for a Russian-labeled translation of an English-original work), that value is preserved as-is — this only fills the empty case.
- [ ] `enrichRecord`: when copying Open Library's `Description` into the record (`rec.Description = enrichedRec.Description`), only do so if `adminservice.IsRussianText(*enrichedRec.Description)` is true; otherwise leave `rec.Description` as-is (nil, in practice, since Wikidata's SPARQL parsing never sets `Description` itself — confirmed via `parseSparqlBindings`, which has no `Description` field mapping). This is deliberately **not** a hard rejection of the whole record: Open Library's excerpt has no language filtering at all today (confirmed — `olBookData`/`olSearchDoc` have no per-field language tagging beyond the whole-doc `Language []string` on search results, which is never copied into `Description`), so a non-Russian excerpt is silently dropped rather than failing an otherwise-good, correctly-Russian-titled book over an optional enrichment field. Document this asymmetry (Title → hard reject via `validateBookInput`; harvest-sourced Description → silent drop before `CreateBook` is even called) explicitly in a code comment.
- [ ] No change to `CoverURL`/`PageCount`/`ISBN` enrichment — not text content, not in scope.

**Backend — bulk import (`backend/internal/modules/admin/service/import.go`, `.../transport/http/import_parse.go`)**
- [ ] No code changes required. `importRow` already funnels every row through `CreateBook`, so the new `ErrTitleNotRussian`/`ErrDescriptionNotRussian` checks apply automatically and surface as a per-row `ImportRowError{Row, Title, Message}` — the existing "one bad row doesn't abort the batch" behavior (`ImportBooks`) is unchanged and needs no new code.
- [ ] `defaultImportLanguage = "ru"` is unchanged (it already defaults the *original-language* field to `"ru"`, which was always a reasonable default for this catalog's primary audience — unrelated to, and unaffected by, this contract's actual content-language enforcement).

**Backend — manual admin create (no separate code path — funnels through the same `CreateBook`/`UpdateBook`)**
- [ ] Confirm (via a test, see Tests) that `POST /api/v1/admin/books` and `PUT /api/v1/admin/books/:id` return `422 {"error": "title must be in Russian"}` / `422 {"error": "description must be in Russian"}` for the respective violations — no new endpoint, no new request/response field.

**Backend — Genres/Authors**
- [ ] No validation change to `CreateAuthor`/`CreateGenre`/`UpdateAuthor`/`UpdateGenre` in this increment (see Scope boundaries decision).

## Business Rules

- **What counts as "Russian text"**: a Cyrillic-letter-ratio heuristic, not true language detection. `IsRussianText` looks only at the ratio of Cyrillic letters among all letters in the string; punctuation, digits, and whitespace don't count either way. Threshold `0.5` (a strict majority) — chosen so a Russian title containing an embedded Latin proper noun or brand name (e.g. "Изучаем Go") still passes, while a wholly English/Latin-script title is rejected. This is a pragmatic MVP choice, explicitly not a linguistically rigorous language detector (it cannot, for example, distinguish Russian from Ukrainian or Bulgarian — both Cyrillic). Acceptable because the practical failure mode this contract targets is "Wikidata/Open Library returned plain English/Latin-script text," not "returned Cyrillic text in the wrong Slavic language."
- **`Language` field semantics are unchanged**: it records the work's original language (evidenced by existing seed data and the `/languages` filter feature), not the language of the catalog's displayed text. This contract does not touch its validation, default, or the `/languages` filter.
- **Failure handling matches each path's existing established pattern, extended, not reinvented**:
  - Manual admin create/update: `422` with a clear message (`ErrTitleNotRussian`/`ErrDescriptionNotRussian`), same convention as every other `ErrInvalid*` validation failure on these endpoints.
  - Bulk import: per-row error in `ImportReport.Errors`, batch continues — no new code needed, inherited from `CreateBook`.
  - Harvest: `Title` failures become a per-record entry in `HarvestResult.Errors` (via `processRecord`'s existing error return → `runHarvest`'s existing error-collection loop), job continues — no new code needed. *(Superseded: harvest titles are now exempt from the Title check — see the Latin-script official titles exemption below.)* `Description` failures never reach `CreateBook` at all (silently dropped upstream in `enrichRecord`, so they can't cause a record failure).
- **Harvest's SPARQL fix is the primary, most targeted mitigation for the actual reported problem** (English-only Wikidata content getting harvested) — it prevents fetching non-Russian-labeled items/authors/genres in the first place, rather than relying solely on the downstream heuristic to catch them after the fact. The Cyrillic-ratio check on `Title` remains as defense-in-depth for the rare case of a `ru`-tagged Wikidata label that is not actually Cyrillic (data-quality edge case), and as the actual enforcement mechanism for the manual/import paths, which have no SPARQL layer to fix. *(Superseded for harvest: harvest titles are now exempt, so this defense-in-depth no longer applies to harvest — see the Latin-script official titles exemption below; harvested books still land hidden for admin review.)*
- **Latin-script official titles exemption** (added by `docs/features/latin-book-titles.md`): the Title check (`ErrTitleNotRussian`) is not applied to (a) Wikidata harvest records, since the title is taken only from the `ru` label and is the official Russian title even when written in Latin script (e.g. «Empire V», «S.N.U.F.F.»); (b) admin create/update requests with `allow_non_russian_title: true` (checkbox «Название на латинице допустимо»); (c) admin updates, including single/bulk publish, that leave the stored title unchanged. CSV/JSON bulk import has no bypass. The Description check (`ErrDescriptionNotRussian`) is unaffected on every path.

## Database

None. No schema change — `Book.Language`, `Author`, `Genre` are all unchanged. This is a validation-layer-only change.

## API

No new endpoints, no request/response shape changes. Behavioral change only:
- `POST /api/v1/admin/books`, `PUT /api/v1/admin/books/:id`: now also return `422` for a non-Russian `title` (`{"error": "title must be in Russian"}`) or non-Russian `description` when provided (`{"error": "description must be in Russian"}`).
- `POST /api/v1/admin/books/import`: unchanged response shape; rows failing the new checks appear in the existing `errors[]` array with the same messages as above.
- `POST /api/v1/admin/harvest` and `GET /api/v1/admin/harvest/jobs*`: unchanged response shape; records failing the new `Title` check appear in the existing `HarvestResult.errors[]` array. Harvest jobs may now report a lower `total`/`created` overall (SPARQL now returns fewer candidate records per job, since non-`ru`-labeled items are excluded at the query level) — this is the accepted coverage tradeoff, not a bug.

## Frontend

- No required frontend changes. The manual admin book-create/edit form (`apps/admin/src/features/manage-book/ui/book-form.tsx`) already surfaces `ApiError` messages inline (existing pattern for `ErrInvalidTitle` etc.) — a `422` with `"title must be in Russian"`/`"description must be in Russian"` will display the same way with zero code changes.
- Optional, not required for this increment: a short helper-text hint under the Title/Description fields in `book-form.tsx` (e.g. "Только на русском языке") — nice-to-have, developer's call whether to include; not an acceptance criterion.

## Existing Patterns

- `backend/internal/modules/admin/service/service.go` — `validateBookInput`, `normalizeOptionalString`, the `ErrInvalid*` sentinel block, and the "shared validation used by both Create and Update" shape — extend directly, don't parallel.
- `backend/internal/modules/admin/service/import.go` — `defaultImportLanguage = "ru"` — the precedent for `processRecord`'s new empty-language default in harvest.
- `backend/internal/modules/harvest/service/handler.go` — `processRecord`/`enrichRecord`'s existing "per-record error doesn't abort the job" pattern (`runHarvest`'s error-collection loop) — reuse as-is, no new mechanism.
- `backend/internal/modules/admin/transport/http/handler.go` — the existing `switch { case errors.Is(err, ...): return c.JSON(http.StatusUnprocessableEntity, ...) }` blocks in `CreateBook`/`UpdateBook` — extend the case list, don't add a new switch.
- `backend/internal/pkg/bookdata/wikidata.go` — `buildTitleAuthorQuery`/`buildISBNQuery`/`BuildAuthorListQuery`'s existing `FILTER(LANG(...) IN (...))` idiom — mechanical find/replace to `= "ru"`, same structure otherwise.

## Tests

- `backend/internal/modules/admin/service/service_test.go` (or a new `language_test.go`): table-driven tests for `IsRussianText` (pure Cyrillic → true; pure Latin → false; mixed above/below threshold → true/false at the `0.5` boundary; empty string → true; digits/punctuation-only → true).
- Extend `TestCreateBook_*` (and the equivalent `UpdateBook` tests) in `service_test.go`: a non-Russian title → `ErrTitleNotRussian`; a non-Russian description with a Russian title → `ErrDescriptionNotRussian`; a Russian title with a nil/empty description → no error (unaffected); an `"en"`-`Language` book with a Russian title/description → still succeeds (proves `Language` is untouched — regression guard for the Scope-boundaries decision).
- `backend/internal/modules/admin/transport/http/handler_test.go`: `CreateBook`/`UpdateBook` non-Russian title/description → `422` with the expected message, extending the existing table-driven error-mapping test if one exists for `ErrInvalidTitle`/`ErrInvalidLanguage`.
- `backend/internal/pkg/bookdata/wikidata_test.go`: update/extend existing SPARQL-string assertions (if any check the literal `FILTER(LANG(...) IN ("ru","en"))` substring) to the new `= "ru"` form for all three query builders.
- `backend/internal/modules/harvest/service/handler_test.go`: extend `processRecord`/`runHarvest` tests — a fake Wikidata record with empty `Language` → created book has `Language == "ru"` (default applied); a fake enrichment `Description` that is non-Russian → book is still created, `Description` is nil (dropped, not rejected); a fake record whose `Title` is non-Russian (simulating a data-quality edge case past the SPARQL filter) → appears in `HarvestResult.Errors`, job's other records still process.
- `backend/internal/modules/admin/service/import_test.go`: a row with a non-Russian title → appears in `ImportReport.Errors`, other rows in the same batch still succeed (extends the existing partial-success test pattern).
- `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/`.
- No required frontend test changes (no frontend code changes); if the optional helper-text hint from the Frontend section is added, a trivial render-only assertion is sufficient, not a hard requirement.

## Documentation

- Mark this file `Status: implemented` when done.
- `docs/ROADMAP.md`: no dedicated line needed (consistent with how the three prior harvest increments were not separately tracked there); optional one-line mention under "Next"/"Completed" at the developer's discretion.
- Brief note in `backend/internal/pkg/bookdata/wikidata.go`'s package or function doc comments (already an Acceptance Criterion above) explaining the ru-only change and the accepted coverage tradeoff, so a future reader doesn't mistake it for an oversight.

## Risks

- **Cyrillic-ratio heuristic is not true language detection.** It cannot distinguish Russian from other Cyrillic-script languages (Ukrainian, Bulgarian, Serbian, etc.), and a short title/description with few letters is a noisier signal than a long one (e.g. a 4-letter mixed-script title sits right at the threshold's edge). Accepted for this MVP; revisit only if false positives/negatives prove disruptive in practice (e.g. by raising the threshold or adding a minimum-letter-count exemption).
- **Reduced harvest coverage.** Dropping the `"en"` SPARQL fallback means literary works with no Russian-language Wikidata label are no longer harvestable at all (previously they'd be created with an English title). This is the explicit, accepted tradeoff — not a regression to fix later, unless the product owner reports the coverage loss as a real problem.
- **Genre/Author manual-entry gap.** An admin can still type a non-Russian `Author.Name`/`Genre.Name`/`Biography` directly into the admin panel — not validated in this increment (see Scope boundaries). Judged low-risk given all three automated write paths (seed, harvest, author search) are Russian by construction, but flagged as a follow-up if it proves to be a real gap in practice.
- **Silent Description-dropping in harvest could surprise an admin** reviewing a pending-review book that has a title but no description, where Open Library did return *something* (just not in Russian). No new UI surfaces this distinction (per Scope boundaries, no new UI is built) — an admin editing the book in the review queue simply sees an empty description field and can fill it in manually, same as any other book Open Library couldn't enrich.
- **Existing books already in the catalog are not retroactively checked.** This is a write-path (create/update) validation only — no backfill/audit of existing rows. Out of scope; flag as a follow-up only if the product owner wants existing data cleaned up.
