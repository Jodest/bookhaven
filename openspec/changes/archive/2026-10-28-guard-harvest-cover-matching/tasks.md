# Tasks

## 1. Backend: Open Library matching (`pkg/bookdata`)

- [x] 1.1 Add `match.go` with `NormalizeMatchText`, `TitleMatches` and `AuthorMatches` (design D1). Add table tests in `match_test.go`:
  - "I, Robot"/Asimov rejected for «История Петра I»/Пушкин;
  - a different Pushkin title rejected for «Капитанская дочка»;
  - «белый клык» = «Белый Клык»;
  - «Зов предков: повесть» matches «Зов предков»;
  - «Ёлка» = «Елка»;
  - "White Fang" matches via an alt title;
  - surname match "Jack London" vs «Джек Лондон» via an English name;
  - empty title or author lists never match.
- [x] 1.2 Add `NumFound` to `olSearchResponse` and the `olIgnoredQueryThreshold = 10000` guard to `FindByTitleAuthor` (returns `ErrNotFound`). Test in `openlibrary_test.go`: a fixture with `numFound: 907849` gives `ErrNotFound`, and the existing fixtures still pass.
- [x] 1.3 Add `TitleAuthorQuery` and `OpenLibraryClient.FindMatchByTitleAuthor` (limit 5, first doc that matches, same `numFound` guard, work-description follow-up only for the matched doc). Tests:
  - the second doc is chosen when the first is unrelated;
  - all 5 unrelated gives `ErrNotFound`;
  - the `numFound` guard;
  - the description follow-up is requested only for the matched work key.
- [x] 1.4 Extend `BuildAuthorListQuery` and its parser with the English work label and English author labels (D4), adding `ExternalBookRecord.TitleEn` and `ExternalAuthor.NameEn`. Update the query-shape assertions in `wikidata_test.go` and add a parse test where English labels are paired by QID or missing.
- [x] 1.5 Verify with `go build ./... && go vet ./... && go test ./internal/pkg/bookdata/...` from `backend/`. Run gofmt only on the changed files.

## 2. Backend: harvest enrichment

- [x] 2.1 In `enrichRecord`, replace the `FindByTitleAuthor` fallback with a type-asserted `titleAuthorMatcher.FindMatchByTitleAuthor`, built from `rec.Title`, `rec.TitleEn` and the Russian and English author names (D4). If the enrich client lacks the method, skip the title path. Keep the ISBN path and the gap-only and Russian-description rules unchanged.
- [x] 2.2 Update `fakeEnrichClient` in `handler_test.go` and the test around line 459 so that it feeds a matching doc. Add tests:
  - no match → book created with nil cover, description and ISBN;
  - a match keeps filling only empty fields;
  - an enrich client without the method makes no title lookup.
- [x] 2.3 Verify with `go build ./... && go vet ./... && go test ./internal/modules/harvest/... ./internal/modules/admin/...` from `backend/`.

## 3. Backend: audit command `olcoveraudit`

- [x] 3.1 Add an admin repository and service query that lists books with `external_source = 'wikidata'` and an Open Library `/b/id/` cover link, with author names, current author and genre IDs, any status. Test it in the repository test style the admin module already uses (no destructive DB commands).
- [x] 3.2 Implement the audit logic (D5) as a testable function with small interfaces:
  - reasons `shared_cover`, `no_match`, `other_match`;
  - per-book error reporting, without aborting the run;
  - apply only to the explicit IDs that are candidates, clearing `CoverURL` and `ISBN` through `adminservice.UpdateBook` with every other field preserved.

  Test with fakes:
  - the report mode makes no update calls;
  - two books with cover `9157583` are both `shared_cover`;
  - apply clears only the listed candidate;
  - an unknown or non-candidate ID is skipped;
  - an Open Library error is reported and the run continues;
  - an `UpdateBook` validation error is reported.
- [x] 3.3 Confirm that clearing the link through `UpdateBook` makes the book show no cover (spec `catalog/book-covers`, "Link removed"), extending the existing service or repository test if the copy state is not already reset or ignored for a NULL link.
- [x] 3.4 Add a thin `cmd/olcoveraudit/main.go` (`-apply`, `-ids`; `-apply` without `-ids` exits 2 with a usage line), and add the build and copy lines in `backend/Dockerfile` next to `tmdbpurge`. Do NOT run the command against the dev database and do not start a second compose stack; running it is the maintainer's step.
- [x] 3.5 Verify with `go build ./... && go vet ./... && go test ./...` from `backend/`.

## 4. Docs

- [x] 4.1 `docs/DEPLOYMENT.md`: add a short «Open Library cover audit» subsection under «Self-hosted covers». Cover:
  - what the command checks;
  - report first: `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/olcoveraudit worker`, and locally `go run ./cmd/olcoveraudit` with the dev `DATABASE_URL`;
  - then `-apply -ids=<uuid,uuid>` for the reviewed IDs;
  - that descriptions are not cleared and should be reviewed in the book editor;
  - that orphaned cover files are removed by the sweep.
- [x] 4.2 `docs/ROADMAP.md`: mark issue #687 as done in the relevant section.
