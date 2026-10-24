# Design

## Context

Reading status = which exclusive default shelf holds the book. `shelf_items.added_at` is the only timestamp, so read→reading deletes the only record of a read and breaks challenge counting (`reading_challenges/repository.CountReadBooks`) and adaptation vote eligibility (`screenworks/repository/votes.go IsReader`). The product owner fixed the model: a `book_reads` journal next to the shelves (shelf = current status, journal = history). This change delivers the re-read part of #204; manual date editing and a history list are phase 2.

## Decisions

1. **«Брошено» becomes a real default shelf.** The `abandoned` enum value exists since 000001, but no shelf of that type is ever created (`EnsureDefaultShelves` inserts three) and no UI exists. "Moving to abandoned" is in scope, so `abandoned` joins the exclusive group: `isDefaultShelf`, `otherDefaultTypes` (now returns the other three; change the `[2]` array to a slice), `EnsureDefaultShelves` (4th row «Брошено», same `ON CONFLICT ... DO NOTHING`) and the `ListByUser` ORDER BY (abandoned = 4, custom = 5). `favorite` stays unused and out of the group.
2. **No dedicated reread endpoint.** "Читаю снова" is the existing `POST /shelves/{readingShelfId}/books`. Every move to «Читаю» opens an attempt, and the earlier finished attempts are never touched. That move used to destroy them, which was the bug. Picking «Читаю» in the chooser and pressing «Читаю снова» are therefore the same request.
3. **The transition table lives in the shelves service** and runs inside the existing default-shelf transaction. `RemoveBook` on a default shelf gets a transaction too. Let `prev` be the default type the book was on before the change, or none:

   | Action | Journal effect |
   |---|---|
   | add → reading | open attempt `started_at=now` if none open |
   | add → read | close open attempt `finished_at=now`; if none open, insert `{started_at NULL, finished_at now}` |
   | add → abandoned | close open attempt `abandoned_at=now`; none open → no-op |
   | add → want_to_read | delete open attempt (if any) |
   | remove reading | delete open attempt |
   | remove read | delete the single latest finished attempt (`ORDER BY finished_at DESC, created_at DESC LIMIT 1`); the book ends with no status |
   | remove want_to_read / abandoned | no-op |
   | custom shelf add/remove | no-op |

   The effect depends only on the target, never on `prev`. That keeps it simple and leaves no shelf/journal combination that cannot be reached. Invariant: book on «Читаю» ⇔ exactly one open attempt. Accepted quirk: read → want_to_read → read records a second finished attempt (a "want to re-read" round trip). See Risks.
4. **Idempotency and concurrency without advisory locks.** Inside the transaction, the shelf mutation goes first and the journal step runs only if it affected a row. The `shelf_items` PK is the serialization point. A second concurrent `INSERT ... ON CONFLICT DO NOTHING` on the same `(shelf_id, book_id)` waits for the first transaction and then sees the conflict (0 rows → `ErrBookAlreadyOnShelf` → rollback → 409). A second concurrent DELETE blocks on the row lock and then affects 0 rows (→ 404 → rollback). As a backstop for cross-target races, a partial unique index allows only one open attempt. "Open if none" uses an insert guarded by that index (`ON CONFLICT DO NOTHING` with the partial-index target, which works in PostgreSQL and SQLite ≥ 3.24) or check-then-insert inside the tx. No `pg_advisory_xact_lock`, `FOR UPDATE` or other PostgreSQL-only constructs: the repository tests run on SQLite.
5. **Abandoned marker = `abandoned_at TIMESTAMPTZ NULL`** rather than an outcome enum. It carries the date for phase 2 and gives a trivial CHECK (`finished_at IS NULL OR abandoned_at IS NULL`).
6. **Consumers.** Only challenge counting and `IsReader` switch to `book_reads`. Similarity, curated auto collections, home personal blocks and «Вам может понравиться» keep using the shelves (current status), unchanged.
7. **Read counter exposure.** On the book page it rides on the call the shelve-book hook already makes (`GET /shelves?book_id=`), so cache invalidation already works. Shelf book lists get two flat fields per item, filled by one batched query per page.

## API

All existing authorization stays: `/api/v1/shelves*` requires a bearer token and checks ownership (403/404). `/shelves/public/{id}` uses optional auth and returns 404 for a private shelf. No new endpoints.

- `GET /api/v1/shelves?book_id=<uuid>` → `{items, total, reads}`. `reads` is `{reads_count: int, last_read_at: RFC3339|null, rereading: bool}`, computed for the token's user. It is omitted (`omitempty` pointer) without a valid `book_id`.
- `GET /api/v1/shelves/{id}` and `GET /api/v1/shelves/public/{id}`: each `items[]` gains `reads_count: int` and `last_read_at: RFC3339|null`, computed for **`shelf.UserID`**, not the caller. Use a new response type or extra fields on `bookListItemResponse` that are filled only by these handlers.
- `POST /api/v1/shelves/{id}/books` / `DELETE /api/v1/shelves/{id}/books/{bookId}`: same contract and status codes; journal side effects as in Decision 3.
- `PUT /books/{id}/adaptations/{sw}/vote` and `GET .../verdicts`: same contract. Eligibility is now `EXISTS (SELECT 1 FROM book_reads WHERE user_id=? AND book_id=? AND finished_at IS NOT NULL)`.
- `GET /reading-challenges/{year}`: same contract. Count = `SELECT COUNT(DISTINCT book_id) FROM book_reads WHERE user_id=? AND finished_at >= ? AND finished_at < ?`. Update the comment that calls `added_at` a proxy.
- Update the swagger annotations of ListShelves, GetShelf and GetPublicShelf, and the AddBook/RemoveBook descriptions (journal side effects, undo).

### Backend layering

- `shelves/domain`: `BookRead` struct (table `book_reads`) and `ReadSummary {Count int64; LastReadAt *time.Time; Rereading bool}`.
- `shelves/repository`: add the following to `Repository` so they work on the tx repo:
  - `OpenRead(ctx, userID, bookID, now)`;
  - `FinishRead(ctx, userID, bookID, now)` (update open → else insert);
  - `AbandonOpenRead(...)`;
  - `DeleteOpenRead(...)`;
  - `DeleteLatestFinishedRead(...)`;
  - `ReadSummary(ctx, userID, bookID)`;
  - `ReadCounts(ctx, userID, bookIDs []uuid.UUID) (map[uuid.UUID]ReadCount, error)`, one GROUP BY query.
- `shelves/service`:
  - AddBook (default branch) and a new transactional RemoveBook branch for default shelves apply the table. RemoveBook needs the shelf type, which it already loads via GetByID.
  - Use a `now` from an injectable clock if the service has one; otherwise `time.Now().UTC()`.
  - ListShelves returns the summary when `bookID != nil`. GetShelf and GetPublicShelf return counts keyed by book id.
- `reading_challenges/repository.CountReadBooks` and `screenworks/repository.IsReader`: raw SQL on `book_reads`, following the existing cross-module SQL pattern.
- `cmd/seed/demo_readers.go`: if it inserts `shelf_items` for read/reading directly, also insert matching `book_reads` rows. Otherwise the demo readers' challenge and vote rights come out empty.

## Database

Migration `backend/migrations/000041_book_reads.up.sql` (forward-only; never edit earlier migrations):

```sql
CREATE TABLE book_reads (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    book_id      UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    started_at   TIMESTAMPTZ NULL,
    finished_at  TIMESTAMPTZ NULL,
    abandoned_at TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT book_reads_single_outcome CHECK (finished_at IS NULL OR abandoned_at IS NULL)
);
-- at most one open attempt per user and book
CREATE UNIQUE INDEX uq_book_reads_open ON book_reads (user_id, book_id)
    WHERE finished_at IS NULL AND abandoned_at IS NULL;
-- reads_count / last_read_at / IsReader / undo
CREATE INDEX idx_book_reads_user_book_finished ON book_reads (user_id, book_id, finished_at DESC)
    WHERE finished_at IS NOT NULL;
-- challenge count by year
CREATE INDEX idx_book_reads_user_finished ON book_reads (user_id, finished_at)
    WHERE finished_at IS NOT NULL;
```

Backfill in the same up migration. `shelves.type` is unique per user for default types, so each book yields at most one row per shelf type:

```sql
INSERT INTO book_reads (user_id, book_id, finished_at, created_at)
SELECT s.user_id, si.book_id, si.added_at, si.added_at
FROM shelf_items si JOIN shelves s ON s.id = si.shelf_id WHERE s.type = 'read';

INSERT INTO book_reads (user_id, book_id, started_at, created_at)
SELECT s.user_id, si.book_id, si.added_at, si.added_at
FROM shelf_items si JOIN shelves s ON s.id = si.shelf_id WHERE s.type = 'reading';

INSERT INTO book_reads (user_id, book_id, abandoned_at, created_at)
SELECT s.user_id, si.book_id, si.added_at, si.added_at
FROM shelf_items si JOIN shelves s ON s.id = si.shelf_id WHERE s.type = 'abandoned';
```

The exclusivity invariant means a book is never on two default shelves, so the open-attempt index cannot conflict. Challenge counts after the backfill equal the old `added_at` counts by construction.

Down migration `000041_book_reads.down.sql`: `DROP TABLE IF EXISTS book_reads;` (indexes go with it). It does not delete `abandoned` shelves created after the rollout: they are harmless to the old code, which lists them after the default shelves and treats them as non-default. The down migration exists for completeness only; agents never run it.

## Frontend

- `packages/api-client`: `Shelf['type']` already includes `abandoned`. Add `reads?: {readsCount, lastReadAt, rereading}` to the shelves list response and `readsCount`/`lastReadAt` to the shelf book item type, following the client's existing snake→camel mapping. `shelvesApi.list` callers that only take `.items` keep working.
- `features/shelve-book/model/use-book-shelves.ts`:
  - Split the constants. `statusTypeOrder` = 4 types (exclusivity in the optimistic update, `isDefaultType`, chooser). The card set stays the 3 types `['want_to_read','reading','read']`, so `ShelveBookButtons` keeps 3 buttons.
  - Add a label «Брошено» and an icon (e.g. lucide `BookX`).
  - The query keeps the whole response (or selects `{items, reads}`), and the hook exposes `reads`.
  - `onMutate`/`onError` keep patching `items` only. `reads` refreshes through the existing `invalidateShelfContents` refetch (`shelfKeys.all` covers `forBook`).
- `features/shelve-book/ui/book-shelf-actions.tsx`:
  - Four statuses in the chooser.
  - «Читаю снова» (secondary button next to the control) when the current shelf is `read`; it calls `toggleDefault(readingShelf)`.
  - The mark «перечитываю» inside the «Читаю» trigger when `reads.rereading`; it also goes into the aria-label.
  - The counter line under the controls when `reads.readsCount >= 1`.
- Shared formatter in `shared/lib`: `formatReadCount(n)` → `Прочитано N ${pluralRu(n, ['раз','раза','раз'])}` (reuse `shared/lib/plural-ru.ts`), and `formatReadCounter(n, lastReadAt)` adding ` · последний раз — <month> <year>`. Use a fixed nominative lowercase month array (Intl `ru` gives genitive/«г.»). Format in local time.
- Views:
  - `views/shelves/ui/shelves-view.tsx`: default order + `abandoned`.
  - `views/public-profile` and `views/public-shelf-detail`: add `abandoned: 'Брошено'` to the label maps.
  - `views/shelf-detail` and `views/public-shelf-detail`: show `formatReadCount(readsCount)` under each book with `readsCount >= 1`.
  - Views use `shared/lib` only; no view imports a view, no feature imports a feature.
- The verdict block already refetches via `screenWorkKeys.all` on every shelf change, so no change is needed there beyond what the backend returns.

## Existing Patterns

- Exclusive default-shelf transaction: `shelves/service.AddBook` + `repo.Transaction`.
- Partial unique index + `ON CONFLICT ... WHERE`: `EnsureDefaultShelves` / migration 000003.
- Cross-module SQL reads: `screenworks/repository/votes.go IsReader`, `reading_challenges` `CountReadBooks`.
- SQLite repository tests: `shelves/repository/private_shelf_sqlite_test.go`, `book_status_sqlite_test.go`.
- Cache invalidation after shelf changes: `features/shelve-book/model/invalidate-shelf-caches.ts`.

## Risks

- **Read → want_to_read → read** adds a second finished read, which inflates the counter. Accepted for MVP. Phase 2 (manual journal editing) is the correction path. The «Убрать статус» undo deletes it.
- **«Брошено» is public** like every default shelf, so it appears on the public profile once it holds a published book. That follows the existing rule (default shelves are always public). Readers who mind can avoid the status. A private-abandoned option would be phase 2.
- **Behavior change for existing users:** moving read → reading no longer lowers the challenge count, and re-readers can vote. Both are intended. The backfill keeps existing numbers identical at rollout.
- **Undo is not a perfect undo:** removing from «Прочитано» does not reopen the attempt or restore «Читаю». This is intentional and specified.
- **Seed data and fixtures** that write `shelf_items` directly (seed, tests of curated/home/screenworks) bypass the journal. Screenworks vote tests that relied on the «Прочитано» shelf must now seed `book_reads`.
- **Down migration** drops all journal history. It never runs in tasks.
