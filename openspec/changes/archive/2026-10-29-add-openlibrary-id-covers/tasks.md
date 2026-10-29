# Tasks

## 1. Backend: bookdata

- [x] 1.1 `BuildAuthorListQuery` reads P648 into `?olids` and `parseSparqlBindings` fills `ExternalBookRecord.OpenLibraryIDs` (design D1). Verify: updated query-shape test and a parser test with two IDs in `wikidata_test.go`.
- [x] 1.2 Add `OrderOpenLibraryIDs` + `MaxOpenLibraryIDsPerWork = 3` (D2). Verify: table test covering invalid values dropped, duplicates removed, works before editions, numeric (not lexical) order, cap of 3, and the Q332581 example.
- [x] 1.3 Add `OpenLibraryClient.FindCoverByOpenLibraryIDs` (D3), cover only, `/works/` vs `/books/`, first positive cover ID, next ID on miss/failure. Verify: fixture-server tests listed in D3.
- [x] 1.4 Add `WikidataClient.FetchOpenLibraryIDs` with `MaxOpenLibraryIDBooks = 50` and QID validation (D5). Verify: query-shape, parsing and invalid-input tests modeled on the `FetchAdaptations` tests.
- [x] 1.5 `go build ./... && go vet ./... && go test ./internal/pkg/bookdata/...` from `backend/`; gofmt only the changed files.

## 2. Backend: harvest enrichment

- [x] 2.1 In `enrichRecord`, add the cover-by-ID step between the ISBN lookup and the strict title search via a local `openLibraryCoverFinder` interface (D4). Gap-filling rules and the Russian-description filter stay unchanged.
- [x] 2.2 Extend `fakeEnrichClient` in `handler_test.go`. Tests:
  - an ID cover is used and the title-search cover is ignored;
  - IDs without a cover fall back to the title-search cover;
  - an ID lookup error still creates the book and tries the title search;
  - a Wikidata cover or an ISBN cover skips the ID step;
  - no description is taken from the ID step;
  - a record without IDs makes no ID call.

  Existing #687 tests stay green.
- [x] 2.3 `go test ./internal/modules/harvest/...` from `backend/`.

## 3. Backend: backfill command

- [x] 3.1 Admin repository + service: `ListWikidataBooksWithoutCover` (D5). Add it to the three admin repository fakes. Verify with an SQLite repository test: Wikidata books with a NULL or empty cover and status pending/published are included; rejected books, books with a cover and non-Wikidata books are excluded; Authors and Genres are preloaded.
- [x] 3.2 `harvest/service/cover_backfill.go`: `RunCoverBackfill` + report writer (D5). Verify with unit tests on fakes:
  - report-only changes nothing;
  - apply updates only the findings, and only those in `-ids` when given;
  - a book with a cover by the time of the re-read is skipped;
  - unknown and non-candidate IDs are skipped;
  - a Wikidata batch error is reported for its books and the run continues;
  - an Open Library error is reported per book;
  - an invalid QID is skipped;
  - the `UpdateBook` input keeps ISBN, description, status, authors and genres, and sets only `CoverURL`;
  - batches hold at most 50 QIDs.
- [x] 3.3 `backend/cmd/olcoverbackfill/main.go`, thin. It handles flags (`-apply`, `-ids` only with `-apply`, exit 2 otherwise) and wires `WithCoverEnqueuer` when `CoversMirrorEnabled`. Add its build and copy lines to `backend/Dockerfile` next to `olcoveraudit`. Verify: `go build ./... && go vet ./...`.
- [x] 3.4 `go test ./internal/modules/admin/... ./internal/modules/harvest/...` from `backend/`. Do NOT run the command with `-apply` against the dev database, and do not start a second compose stack; the maintainer runs it after merge. Do not run any destructive DB command.

## 4. Docs

- [x] 4.1 `docs/DEPLOYMENT.md`: add an "Open Library cover backfill" section next to the cover audit: what it does, dry-run first, `-apply` and `-ids`, prod/dev commands, and that it is safe to re-run.
- [x] 4.2 `docs/ROADMAP.md`: mark #688 accordingly.
- [x] 4.3 Validate the change: `npx -y @fission-ai/openspec@1.13.2 validate add-openlibrary-id-covers --strict` (OPENSPEC_TELEMETRY=0).
