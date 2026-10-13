# Tasks

## 1. Backend

- [x] 1.1 In `backend/internal/modules/admin/service/service.go`, make `UpdateBook` keep the stored `ExternalSource`/`ExternalID` instead of assigning them from `BookInput`. Update the `BookInput` field comment to say they apply on create only. Keep `CreateBook` (the harvest sets them there) unchanged.
- [x] 1.2 Service test (existing fakes in `service_test.go`): updating a book that has `ExternalSource = "wikidata"` and an `ExternalID` with a `BookInput` that leaves them nil keeps both; the returned book and the stored one both still have them.
- [x] 1.3 Verify:
  - `docker compose run --rm backend go build ./...`
  - `docker compose run --rm backend go vet ./...`
  - `docker compose run --rm backend go test ./internal/modules/admin/... ./internal/modules/harvest/...`
  - gofmt only the changed files, never the whole tree.
