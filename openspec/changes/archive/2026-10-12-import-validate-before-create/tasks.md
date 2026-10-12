# Tasks

## 1. Backend

- [x] 1.1 Add the transaction helper.
  - In `backend/internal/modules/admin/repository/repository.go`, add `WithinTransaction(ctx, fn func(repo Repository) error) error` to `Repository`. Implement it on `GormRepository` as `r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&GormRepository{db: tx}) })`.
  - Add `admin/repository/within_transaction_sqlite_test.go`, following the existing `*_sqlite_test.go` setup. Cover two cases:
    - An author created through the tx-bound repository, with `fn` then returning an error, is absent afterwards.
    - On a nil return, the author is present, and a `CreateBook` called inside the same transaction (the nested savepoint) is committed with its author links.
- [x] 1.2 Implement `WithinTransaction` in every test fake of `repository.Repository`.
  - `fakeRepository` (`admin/service/service_test.go`), `fakeMergeAuthorsRepository` (`admin/service/merge_author_test.go`) and `fakeAdminRepository` (`harvest/service/handler_test.go`): implement it as `return fn(f)`.
  - `fakeImportRepository` (`admin/service/import_test.go`): copy `books`, `authors` and `genres` before calling `fn`, and restore them when `fn` returns an error.
  - Add a `createBookErr func(book *catalogDomain.Book) error` hook to `fakeImportRepository`, consulted in `CreateBook`.
  - Verify that `go build ./...` and `go vet ./...` compile all the fakes.
- [x] 1.3 In `admin/service/service.go`, extract the checks that `CreateBook` runs before the UUID parsing into an unexported helper, for example `prepareNewBook`. In today's order, the checks are: status, `validateBookInput`, slug derivation and `isURLSafeSlug`, `BookSlugExists` (`ErrDuplicateSlug`), and ISBN normalization and checksum. The helper returns the derived slug, status and normalized ISBN. `CreateBook` must use it. Verify that every existing `CreateBook`/book test in `service_test.go` passes unchanged.
- [x] 1.4 In `admin/service/service.go` (or `import.go`), add `func (s *Service) withRepo(repo repository.Repository) *Service`, which returns a shallow copy with `repo` replaced.
- [x] 1.5 Rewrite `importRow` in `admin/service/import.go` per design Decision 3.
  - Run the no-authors check first: no non-blank trimmed name gives `errImportNoAuthors`.
  - Then build the `BookInput` (with the language default) and run `prepareNewBook`. Its errors are row errors, and `ErrDuplicateSlug` is a skip. No write happens before this point.
  - Then, inside `s.repo.WithinTransaction`, use `s.withRepo(txRepo)` to resolve the authors and the genres and call `CreateBook`, returning any error so the row rolls back.
  - Classify a skip only for `ErrDuplicateSlug` coming from `CreateBook`. An `ErrDuplicateSlug` from author or genre creation, for example a concurrent import, stays a row error (design Risks).
  - Return the created entities only on the created outcome.
  - Update the doc comments on `ImportReport.CreatedAuthors`/`CreatedGenres`, `importRow` and `ImportBooks`.
- [x] 1.6 Update and extend `admin/service/import_test.go`.
  - Replace the existing tests "new author on a failed row is listed" and "new genre on a skipped row is listed". The new expectations: the row is in `errors` or counted in `skipped_existing`, and the entity is neither in the fake's `authors`/`genres` nor in the report.
  - Add these tests:
    - A row with blank authors and a new genre creates no genre.
    - A new author first named on a failed row 2 and then on a successful row 5 is listed once with `row: 5`.
    - A partial resolver failure (`createAuthorErr` on the second name) leaves the first new author absent and unlisted, and the row is in `errors`.
    - A `createBookErr` returning a generic error after the row's new author and genre were created leaves both absent and unlisted, and the row is in `errors`.
    - A `createBookErr` returning a duplicate-key error (a concurrent book) counts the row as `skipped_existing` and leaves nothing behind.
    - An `ErrDuplicateSlug` from author creation is a row error, not a skip.
    - A row with an existing slug and an invalid ISBN is still `skipped_existing`, which proves the order is unchanged.
    - Valid rows around a failing row are unaffected.
  - Keep the existing happy-path, dedup, merged-genre, existing-entities and empty-file tests passing.
- [x] 1.7 If a swagger-annotated doc comment changed, regenerate with `docker compose exec backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`. Otherwise leave `backend/docs` untouched.
- [x] 1.8 Verify:
  - `docker compose run --rm backend go build ./...`
  - `docker compose run --rm backend go vet ./...`
  - `docker compose run --rm backend go test ./internal/modules/admin/... ./internal/modules/harvest/...`
  - Run gofmt only on the changed files, for example `docker compose run --rm backend gofmt -l internal/modules/admin/service/import.go internal/modules/admin/service/service.go internal/modules/admin/repository/repository.go …`. Never format the whole tree.
  - Never run destructive DB commands (`migrate down`, drop, reset).

## 2. Frontend

- [x] 2.1 No change: the report shape is unchanged, and the result view's copy («Созданные авторы (N)», «Созданные жанры (N)», the pending note) does not imply that failed rows create entities. Confirm with a quick read of `apps/admin/src/widgets/book-file-import/ui/book-file-import-panel.tsx` and do not edit it.

## 3. Docs

- [x] 3.1 Do not edit the frozen `docs/features/catalog-manual-import.md` or the archived `report-import-created-entities` change. The new behavior lives in this change's spec delta, which is merged into `openspec/specs/catalog-import/book-file-import/spec.md` at archive time.
- [x] 3.2 Validate the change with `npx -y @fission-ai/openspec@1.13.2 validate import-validate-before-create --strict` (with `OPENSPEC_TELEMETRY=0`).
