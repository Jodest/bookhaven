# Design

## Context

`books.created_at` is the insert time. Harvested books are inserted `pending` and published from the review queue days or weeks later, so `ListNewBookIDs` (`backend/internal/modules/home/repository/repository.go`, `ORDER BY created_at DESC, id DESC`) ranks them by harvest time. A bulk publish of old harvests may therefore never reach «Новое в каталоге». `docs/features/home-page-content.md` (Risks) names `published_at` as the follow-up. There is no status history table, so publish times of past transitions are unknown.

Paths that can make a book `published` today:
- `Service.CreateBook` (`admin/service/service.go`): used by `POST /admin/books` (default `published`), the file import (`admin/service/import.go`, batch status, inside `WithinTransaction`), and the harvest (`harvest/service/handler.go`, always `pending`).
- `Service.UpdateBook` → `GormRepository.UpdateBook` (`tx.Omit("Authors","Genres").Save(book)`), used by `PATCH /admin/books/:id`.
- `Service.BulkSetBookStatus` → `GormRepository.SetBookStatus` (`admin/repository/author_moderation.go`, `Updates(map{status, updated_at})`), used by `POST /admin/books/bulk-status`.
- `backend/cmd/seed/main.go`: `db.FirstOrCreate(&item.book, …)` with `Status: published`.

No other code writes `books.status` (verified by grep for `BookStatus*`). Author and genre merges do not touch book status.

## Decisions

1. **Set once, on the first transition into `published`; never cleared.** The rule is `published_at = COALESCE(published_at, now())` whenever the new status is `published`. Unpublishing keeps the value, and republishing keeps the original.
   - Why: «Новое в каталоге» should mean "newly visible on the site". A book that was public before is not new.
   - Alternatives rejected:
     - "Every transition into published" would let any admin bump a book to the top by unpublishing and republishing it, and accidental unpublish/republish would reshuffle the row.
     - "Clear on unpublish" has the same problem, and loses information too.
   - Trade-off: a book that was rejected long ago, fixed and republished does not reappear as new. That is acceptable, because it was visible before.
2. **Backfill = `created_at`** for every currently published book.
   - Why: `created_at` reproduces the current home ordering exactly, so nothing jumps on deploy.
   - Why not `updated_at`: it moves on any edit (description, cover, slug), so it would promote recently edited old books. For queue-published harvests it is often close to the publish time, but not reliably, and we have no history to tell the cases apart.
   - Pending and rejected books stay `NULL`, including rejected books that were once published (unknowable). If such a book is republished later, it counts as first published at that time. This is a minor, one-off inaccuracy.
3. **Application-level, not a DB trigger.**
   - The project has no triggers. The four write paths are few and explicit.
   - A domain helper keeps the rule in one place for struct-based writes. The two update queries use the same `COALESCE` expression in SQL.
   - No `CHECK (status <> 'published' OR published_at IS NOT NULL)` constraint: many integration tests insert published books with raw SQL and would break. The read side tolerates `NULL` instead (Decision 5).
4. **`published_at` is not overwritten by `Save`.** `GormRepository.UpdateBook` must not write a stale `published_at` from the loaded struct:
   - add `"PublishedAt"` to its `Omit(...)`;
   - when `book.Status == published`, run `UPDATE books SET published_at = COALESCE(published_at, ?) WHERE id = ?` in the same transaction.

   This avoids a lost update when a bulk publish lands between the service's read and its `Save`.
5. **Home query:** `ORDER BY published_at DESC NULLS LAST, id DESC`. Postgres puts NULLs first in DESC order, so `NULLS LAST` is mandatory. After the backfill, published rows only have `NULL` in raw-SQL test fixtures.
6. **Scope kept tight:**
   - The public `sort=new` (published year) and the default rating sort are unchanged. They are browse sorts, not "recently added", and the home link «Весь каталог» → `/books?sort=new` is already documented as a different list.
   - The admin book list ordering is unchanged.
   - No publish time for authors and genres.
   - No admin UI change: the field is exposed for API consumers and future use only.
7. **Spec placement:** the home page has no OpenSpec domain. The ordering rule is an ADDED requirement in `catalog-moderation/entity-status`, because that capability already owns "Public visibility of books", which names the home page. A new home capability for one rule would be premature.

## API

- Admin `bookResponse` (`admin/transport/http/handler.go`) gains `PublishedAt *time.Time \`json:"published_at"\``.
  - No `omitempty`: the field is always present, `null` when unset, matching `AdminBook.published_at: string | null`.
  - It appears in `GET /admin/books`, `GET /admin/books/:id`, the `POST`/`PATCH` responses and the queue rows, if they use `toBookResponse`. Check `book_moderation` queue DTOs and add the field only where `toBookResponse` is reused. Do not add it to queue-specific DTOs.
- Request bodies: no new field. An unknown `published_at` is ignored by the JSON binding.
- Public `bookResponse` (`catalog/transport/http/handler.go`): unchanged.
- Auth: unchanged (admin role + bearer on all admin book routes).

## Database

`backend/migrations/000020_books_published_at.up.sql`:

```sql
ALTER TABLE books ADD COLUMN published_at TIMESTAMPTZ NULL;
UPDATE books SET published_at = created_at WHERE status = 'published';
CREATE INDEX idx_books_published_at ON books (published_at DESC NULLS LAST, id DESC) WHERE status = 'published';
```

`000020_books_published_at.down.sql` (provided per convention, **never run by agents**):

```sql
DROP INDEX IF EXISTS idx_books_published_at;
ALTER TABLE books DROP COLUMN IF EXISTS published_at;
```

- `TIMESTAMPTZ` matches `books.created_at` (`000001_initial`).
- The partial index matches the home query exactly. It is cheap at the current catalog size.

Domain: `catalogDomain.Book` gains `PublishedAt *time.Time` and a method `MarkPublished(now time.Time)`, which sets `PublishedAt` only when `Status == published` and `PublishedAt == nil`.

### Write paths

- `Service.CreateBook`: after building `book`, call `book.MarkPublished(time.Now())`. This covers manual create and file import (published batches). Harvest stays `NULL`.
- `Service.UpdateBook` + `GormRepository.UpdateBook`: see Decision 4. The service does not set the field itself.
- `GormRepository.SetBookStatus`: when `status == published`, add `"published_at": gorm.Expr("COALESCE(published_at, ?)", now)` to the `Updates` map, using the same `now` as `updated_at`. Other statuses: map unchanged.
- Seed: call `item.book.MarkPublished(time.Now())` before `FirstOrCreate`. Existing seeded rows are not updated, and the migration backfill covers them.

## Frontend

- `packages/api-client/src/index.ts`, `AdminBook`: add `published_at: string | null;` after `status`, with a one-line doc comment ("first time the book became published; null if never").
- No rendering change in `apps/admin` or `apps/web`. Fix only the test fixtures or mocks typed as `AdminBook` that fail `tsc`.

## Existing Patterns

- Explicit-column `Updates` map in `SetBookStatus`, which preserves "only the status changes".
- The `gorm.Expr` usage already present in the admin/catalog repositories, used for the `COALESCE`.
- The `*_sqlite_test.go` / Postgres integration test setup already used by the admin repository and home repository tests.

## Risks

- **SQLite-based repository tests:** `COALESCE` and `NULLS LAST` are supported by SQLite ≥ 3.30, but the partial index DDL lives only in Postgres migrations. Check that the test setup does not use AutoMigrate in a way that conflicts.
- **Home tests relying on `created_at` order** must set `published_at` (or rely on `NULLS LAST`). Update them rather than weakening the assertions.
- **Home cache:** a newly published book appears after the existing home cache TTL. That is unchanged behavior, and the scenario wording reflects it.
- **Backfill inaccuracy:** books published from the queue before this change keep their harvest-time position (no history). This is accepted, and new publishes are correct from deploy on.
- **Swagger drift:** the admin response model changes, so regenerate `backend/docs`.
