# Tasks

## 1. Backend

- [x] 1.1 In `backend/internal/modules/admin/service/harvest_resolve.go`, add the unexported `resolveOrCreateAuthorsByNames` and `resolveOrCreateGenresByNames`. They return the IDs, the created entities, and an error. On a mid-list failure they return the entities created so far together with the error. A genre alias match or any existing match is never counted as created. Turn the exported `ResolveOrCreate{Author,Genre}IDsByNames` into wrappers with unchanged signatures. Verify that the existing `harvest_resolve_test.go`, `genre_status_test.go` and `genre_moderation_test.go` pass unchanged.
- [x] 1.2 In `backend/internal/modules/admin/service/import.go`, add `ImportCreatedEntity{ID, Name, Row}` and the fields `CreatedAuthors`/`CreatedGenres` (`created_authors`/`created_genres`) on `ImportReport`, initialized to empty slices. `importRow` returns the entities it created, also on its error and skipped paths. `ImportBooks` appends them with `Row: i + 1`, dedupes by ID and uses the stored entity `Name`.
- [x] 1.3 Add tests in `backend/internal/modules/admin/service/import_test.go` for:
  - a new author and a new genre being listed with their row;
  - existing entities and a merged-genre match not being listed;
  - the same new author on two rows being listed once, with the first row;
  - a new author on a row that fails validation being listed, and that row being in `errors`;
  - a new genre on a `skipped_existing` row being listed;
  - an empty file returning `[]` for both arrays;
  - a partial resolver failure still reporting the already-created entity, if the test double allows injecting a create failure.
- [x] 1.4 In `backend/internal/modules/admin/transport/http/handler_test.go`, make `fakeAdminService.ImportBooks` return empty `CreatedAuthors`/`CreatedGenres`. If a handler test decodes the report, assert that both arrays are serialized as `[]`, not `null`.
- [x] 1.5 Regenerate Swagger with `docker compose exec backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal` and check that `ImportReport`/`ImportCreatedEntity` appear in `backend/docs/swagger.yaml`.
- [x] 1.6 Verify:
  - `docker compose run --rm backend go build ./...`
  - `docker compose run --rm backend go vet ./...`
  - `docker compose run --rm backend go test ./internal/modules/admin/...`
  - Run gofmt only on the changed Go files (for example `docker compose run --rm backend gofmt -l internal/modules/admin/service/import.go internal/modules/admin/service/harvest_resolve.go`). Never format the whole tree.
  - Never run destructive DB commands (`migrate down`, drop, reset).

## 2. Frontend

- [x] 2.1 In `packages/api-client/src/index.ts`, add the `ImportCreatedEntity` interface and `created_authors`/`created_genres` on `ImportReport`.
- [x] 2.2 In `apps/admin/src/widgets/book-file-import/ui/book-file-import-panel.tsx`, render the «Созданные авторы (N)» and «Созданные жанры (N)» lists only when they are non-empty. Each item links to `/catalog/authors/{id}/edit` or `/catalog/genres/{id}/edit` (use the same typedRoutes cast as `author-create-section.tsx`) and shows «строка N». The genres list shows the note «Новые жанры созданы на модерации и не видны на сайте до публикации.» Use `?? []` for missing fields.
- [x] 2.3 In `apps/admin/src/widgets/book-file-import/ui/book-file-import-panel.test.tsx`, update the report fixtures with the new fields and add tests:
  - lists with counts, link hrefs and row labels;
  - the pending note on genres;
  - no lists for empty arrays.
  If a fixture in `apps/admin/src/views/dashboard/ui/dashboard-view.test.tsx` is typed as `ImportReport`, fix it too.
- [x] 2.4 Verify natively:
  - `tsc --noEmit` in `packages/api-client` and `apps/admin`
  - `apps/admin/node_modules/.bin/next lint` from `apps/admin`
  - the widget's vitest tests
  - Never run a native `next build`; the CI frontend job is the typedRoutes gate.

## 3. Docs

- [x] 3.1 Do not edit the frozen `docs/features/catalog-manual-import.md`. The behavior lives in this change's spec delta, which is merged into `openspec/specs/catalog-import/book-file-import/spec.md` at archive time.
- [x] 3.2 Validate the change with `npx -y @fission-ai/openspec@1.13.2 validate report-import-created-entities --strict` (with `OPENSPEC_TELEMETRY=0`).
