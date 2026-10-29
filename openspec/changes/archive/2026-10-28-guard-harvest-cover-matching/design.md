# Design

## Context

`enrichRecord` (`backend/internal/modules/harvest/service/handler.go`) falls back to `OpenLibraryClient.FindByTitleAuthor` (`backend/internal/pkg/bookdata/openlibrary.go`), which returns `mapOLSearchDoc(parsed.Docs[0])` without any check. When Open Library cannot match a Cyrillic title it ignores the title (the response has `"q":""` and a huge `numFound`) and returns an arbitrary book. All 683 local books come from the Wikidata harvest and none has a Wikidata ISBN, so this title search is the only Open Library path they ever used.

What the wrong match can write onto a harvested book:
- `CoverURL`: `mapOLSearchDoc` builds `https://covers.openlibrary.org/b/id/<cover_i>-L.jpg`.
- `ISBN`: `doc.isbn[0]`. With a bad checksum, `CreateBook` even fails the whole record with `ErrInvalidISBN`.
- `Description`: from the `/works/{key}.json` follow-up (`WithWorkDescriptionLookup`, worker only), kept only if `IsRussianText`.
- `PageCount`: **not affected**. The search doc carries no page count; only `FindByISBN` sets it. The issue's worry about page counts does not apply to the title path.

`FindByTitleAuthor` is also the admin lookup (`backend/internal/modules/admin/service/lookup.go`, `GET /admin/books/lookup`).

## Decisions

### D1. Strict matching lives in `bookdata`, applied only by the harvest

- Add a pure, exported matcher in a new `backend/internal/pkg/bookdata/match.go`:
  - `NormalizeMatchText(s string) string`: lower case, `ё`→`е`, drop every rune that is not `unicode.IsLetter`/`IsDigit`/space, collapse spaces, trim.
  - `TitleMatches(candidate string, wanted []string) bool`: the normalized candidate, or the normalized part of the raw candidate before the first `:`, `(`, ` — `, ` - `, equals a non-empty normalized wanted title.
  - `AuthorMatches(candidate []string, wanted []string) bool`: the last word of some normalized candidate name equals the last word of some normalized wanted name. Empty lists never match.
  - No fuzzy or Levenshtein matching on purpose. A missed cover is cheap; a wrong cover is what this issue is about. Normalization already absorbs case, «ё», quotes and punctuation.
- Add `TitleAuthorQuery{Title string; AltTitles []string; Authors []string}` and `func (c *OpenLibraryClient) FindMatchByTitleAuthor(ctx, q TitleAuthorQuery) (*ExternalBookRecord, error)`. It searches with `title=q.Title` and `author=q.Authors[0]` (the first Russian author name, as today) with `limit=5`. It returns the first doc where `TitleMatches(doc.Title, [Title]+AltTitles)` and `AuthorMatches(doc.AuthorName, Authors)` both hold, else `ErrNotFound`. The work-description follow-up runs only for the matched doc.
- Keep the `bookdata.Client` interface unchanged, so `WikidataClient` is not touched. Harvest declares a local interface `titleAuthorMatcher { FindMatchByTitleAuthor(...) }` and type-asserts `enrich`. If `enrich` does not implement it, the title fallback is skipped entirely. There is no loose fallback any more.
- The harvest **never** calls `FindByTitleAuthor` after this change.

### D2. Admin lookup keeps the loose search, plus the "query ignored" guard

The admin lookup is a human-reviewed prefill. Authors and genres are never auto-selected, and admins often type a Russian title for a book that Open Library lists in English, which a strict title check would turn into 404s. So `FindByTitleAuthor` keeps returning `docs[0]`, with one exception: the shared guard in D3. This is deliberate and recorded in the `book-lookup` delta.

### D3. `numFound` guard in the client, for both paths

- Add `NumFound int \`json:"numFound"\`` to `olSearchResponse`.
- Add the constant `olIgnoredQueryThreshold = 10000`. If `NumFound > 10000`, both `FindByTitleAuthor` and `FindMatchByTitleAuthor` return `ErrNotFound`. A dropped query returns roughly the whole index (about 900 000 in the repro). A real title+author search is orders of magnitude smaller, so this only catches the "query ignored" case and does not hide legitimate broad admin queries.
- The admin handler already maps `ErrNotFound` to 404. No transport change.

### D4. English labels in the harvest SPARQL

In `BuildAuthorListQuery` (`backend/internal/pkg/bookdata/wikidata.go`, line 287), plus its row parser:
- `OPTIONAL { ?item rdfs:label ?itemEn . FILTER(LANG(?itemEn) = "en") }` and select `(SAMPLE(?itemEn) AS ?itemLabelEn)`.
- `OPTIONAL { ?author rdfs:label ?authorEn . FILTER(LANG(?authorEn) = "en") }` and add `(GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?author), "/entity/"), "\t", ?authorEn); separator="|") AS ?authorsEn)`, the same `QID\tlabel` shape as `?authors`, so the parser pairs the English names by QID.
- `ExternalBookRecord` gets `TitleEn string`. `ExternalAuthor` gets `NameEn string`. The parser fills them. Other queries (`FindByISBN`/`FindByTitleAuthor` on Wikidata, the free SPARQL from `cmd/harvest`) leave them empty, and matching then simply uses the Russian values.
- Mind the `GROUP BY ?item ?itemLabel`: the two new OPTIONALs multiply rows inside the group, but `SAMPLE`/`GROUP_CONCAT DISTINCT` keep the output stable. Update the existing query-shape tests in `wikidata_test.go`.
- Nothing new is persisted.

`enrichRecord` builds `TitleAuthorQuery{Title: rec.Title, AltTitles: [rec.TitleEn] if non-empty, Authors: ru names + en names of rec.AuthorRefs}`, falling back to `rec.Authors`. The `Title` and first author are what is searched; the rest is used only for matching.

### D5. Cleanup is a report-first CLI, `cmd/olcoveraudit`

This follows the `cmd/tmdbpurge` / `cmd/similarity` pattern: `config.Load`, `database.Open`, a context of about 15 minutes (Open Library pacing is about 1 request/s), and an `OpenLibraryClient` with no work-description lookup.
- Flags: `-apply` (default false) and `-ids` (comma-separated UUIDs; required with `-apply`, otherwise exit 2 with a usage message). There is no "apply all" option on purpose: the operator copies IDs from the report.
- Candidates come from a new admin repository/service query: books with `external_source = 'wikidata'` and `cover_url LIKE 'https://covers.openlibrary.org/b/id/%'`, any status, with author names preloaded. Extract the cover ID with a regexp, `/b/id/(\d+)-[SML]\.jpg`.
- Checks: `shared_cover` is computed in Go from the candidate set, plus any other book with the same cover ID; one query over all books with that prefix is enough. `no_match`/`other_match` come from `FindMatchByTitleAuthor(Title: book.Title, Authors: author names)`, comparing the matched record's `CoverURL` to the book's. English labels are not available here, so a correct English-titled match can show up as `no_match`. That is why the operator reviews the list.
- Output: one plain-text line per flagged candidate (`id status reasons title | authors | cover | isbn | has_description`), then `checked=N flagged=M ok=K errors=E`. With `-apply`, also one line per ID: `cleared` / `skipped: not a candidate` / `skipped: unknown id` / `error: …`.
- Clearing: `adminservice.UpdateBook(ctx, id, input)`, where `input` is built from the stored book: `Title`, `Slug`, `Description`, `PublishedYear`, `PageCount`, `Language`, current author and genre IDs, `Status: nil`, with `CoverURL: nil` and `ISBN: nil`. The title is unchanged, so the title-language check is skipped (grandfathering). The normal save is what makes the copy follow `catalog/book-covers` "Link removed": verify that the repository update resets or ignores the copy state when the link becomes NULL, and fix it there if not. The orphaned file is removed by the sweep (more than 1 hour old, unreferenced), so the CLI needs no cover storage mount. If `UpdateBook` returns a validation error (for example a stored description that fails the Russian check), report `error:` for that ID and continue.
- The CLI lives in `cmd/olcoveraudit/main.go`, thin. The audit logic goes in a testable function, for example in `backend/internal/modules/harvest/service/cover_audit.go`, behind small interfaces for the book source, the matcher and the updater.

## API

No HTTP changes. `GET /api/v1/admin/books/lookup` keeps its contract; only the D3 case moves from 200 to 404.

## Database

No migration. The audit is a read query plus normal `UpdateBook` saves of explicitly listed IDs. It never runs SQL deletes, `migrate down` or bulk updates.

## Existing Patterns

- `cmd/tmdbpurge/main.go`: report-first maintenance command (`-dry-run`), run with `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/tmdbpurge worker`. Add the build and copy lines for `olcoveraudit` to `backend/Dockerfile` the same way.
- `enrichRecord` tests in `backend/internal/modules/harvest/service/handler_test.go` (`fakeEnrichClient`). Extend the fake with `FindMatchByTitleAuthor`. The test around line 459 that expects a description from the title path must now feed a matching doc.
- `openlibrary_test.go` fixture server pattern for the search tests.

## Risks

- **Fewer covers.** Strict matching will drop some correct covers, for example a Russian work whose Open Library doc has only a transliterated title and no English Wikidata label. This is accepted: no cover beats a wrong cover, and an admin can set the cover by hand.
- **SPARQL cost.** Two extra OPTIONAL label joins add a little to the Wikidata query time. The limit is still at most 5000 works.
- **Audit false positives.** `no_match` without English labels: mitigated by the review step, and the report carries the cover link so the operator can open it.
- **Agents must not run `olcoveraudit -apply`** against the shared dev database, and must not start a second compose stack. Running it is the maintainer's step after merge.
