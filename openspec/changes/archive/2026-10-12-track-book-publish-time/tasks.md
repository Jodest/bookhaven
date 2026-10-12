# Tasks

## 1. Backend

- [x] 1.1 Add the forward-only migration `backend/migrations/000020_books_published_at.up.sql` and its `.down.sql`, exactly as in design "Database". Apply it locally with `migrate up` only, and check that every published book has `published_at = created_at` and that every non-published book has `NULL`. Never run `migrate down`, drop or reset.
- [x] 1.2 In `catalog/domain/book.go`, add `PublishedAt *time.Time` and `MarkPublished(now time.Time)`. `MarkPublished` sets the field only when the book is published and the field is nil. Add a unit test for the three cases: pending (stays nil), published with nil (set), and published with an existing value (kept).
- [x] 1.3 In `admin/service/service.go`, make `CreateBook` call `book.MarkPublished(time.Now())` before `repo.CreateBook`. Tests (service fakes): a default create has `PublishedAt` set; a `pending` create has nil; a file import with batch `published` sets it, and one with batch `pending` does not.
- [x] 1.4 In `admin/repository/repository.go`, update `UpdateBook`. Add `"PublishedAt"` to the `Omit`. When `book.Status` is published, run `UPDATE books SET published_at = COALESCE(published_at, ?) WHERE id = ?` inside the same transaction. Repository test: publishing a pending book through `UpdateBook` sets the time; a later unpublish and republish keeps it; an edit that passes a stale or nil `PublishedAt` in the struct does not clear it.
- [x] 1.5 In `admin/repository/author_moderation.go`, update `SetBookStatus`. When the status is published, add `published_at = COALESCE(published_at, now)` to the `Updates` map, using the same `now` as `updated_at`. Update the comment, which currently says that only status and updated_at are written. Repository test: the bulk-publish scenario from the bulk-status delta (one book never published, one previously published), and a bulk reject that keeps `published_at`.
- [x] 1.6 In `admin/transport/http/handler.go`, add `PublishedAt *time.Time \`json:"published_at"\`` to `bookResponse` (no omitempty) and map it in `toBookResponse`. Handler test: the field is `null` for a pending book and set for a published one. Leave the public catalog `bookResponse` unchanged, and assert in an existing public handler test that `published_at` is absent.
- [x] 1.7 In `home/repository/repository.go`, change `ListNewBookIDs` to `ORDER BY published_at DESC NULLS LAST, id DESC`. Update its interface comment and the package doc if it mentions created_at. Update the home repository tests that seeded the order through `created_at`, and add the test "old created_at, recent published_at ranks first".
- [x] 1.8 In `cmd/seed/main.go`, call `item.book.MarkPublished(time.Now())` before `FirstOrCreate`.
- [x] 1.9 Regenerate swagger: `docker compose exec backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`.
- [x] 1.10 Verify:
  - `docker compose run --rm backend go build ./...`
  - `docker compose run --rm backend go vet ./...`
  - `docker compose run --rm backend go test ./internal/modules/catalog/... ./internal/modules/admin/... ./internal/modules/home/... ./internal/modules/harvest/...`
  - gofmt only the changed files (`docker compose run --rm backend gofmt -l <files>`), never the whole tree.

## 2. Frontend

- [x] 2.1 In `packages/api-client/src/index.ts`, add `published_at: string | null;` to `AdminBook`, with a doc comment. Fix any `AdminBook` fixtures or mocks that `tsc` flags. Make no UI change.
- [x] 2.2 Verify with `tsc --noEmit` and `next lint` in `apps/admin` (and in `apps/web` if it imports `AdminBook`). Never run a native `next build`.

## 3. Docs

- [x] 3.1 In `docs/features/home-page-content.md`:
  - In the «Новое в каталоге» section, replace the `created_at` ordering with `published_at DESC NULLS LAST, id DESC`, and point to this change.
  - Mark the `created_at` Risk as resolved by #245.
  - In the Database note, mention `idx_books_published_at`.
  - Do not edit the frozen `docs/features/book-status-model.md`.
- [x] 3.2 In `docs/ROADMAP.md`, mark the #245 entry done when the change ships.
- [x] 3.3 Validate the change with `npx -y @fission-ai/openspec@1.13.2 validate track-book-publish-time --strict` (`OPENSPEC_TELEMETRY=0`).
