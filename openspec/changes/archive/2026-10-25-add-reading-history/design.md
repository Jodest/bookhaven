# Design

## Context

Phase 1 (`openspec/changes/archive/2026-10-25-add-reread-tracking/`, PR #662, migration 000041) added `book_reads`. The shelf is the current status, and the journal is the history, changed only by the transition table in `shelves/service` (`journalOnAdd` / `journalOnRemove`). The counter, the challenge (`reading_challenges` `CountReadBooks`, UTC year) and adaptation voting (`screenworks` `IsReader`) all read from the journal. This phase lets the owner add, edit and delete journal rows, and shows the history on the book page. All consumers are derived live, so they need no change.

## Decisions

1. **Outcome is immutable, open rows follow the shelf.** These rules keep the CHECK and the partial unique index trivially safe, and they keep the invariant "book on «Читаю» ⇔ exactly one open row":
   - An open row can only change `started_at`, and it cannot be cleared because every open row has one today. It cannot be deleted through the history (409). Finishing or abandoning stays a status transition.
   - Closed rows can change `started_at` (nullable) and their own end column. They never switch between finished and abandoned. That would change the read count and the challenge without any status change, and a book on «Брошено» could end with a finished last attempt. To correct a wrong outcome, the reader deletes the row and adds a past reading.
   - Manual add always creates a **finished** row (owner decision), so it cannot touch the open-row index.
2. **Dates are days, stored at 12:00 UTC.** The API takes `YYYY-MM-DD` and stores `D 12:00:00Z`.
   - The challenge counts by UTC year, so a picked day always counts for its own year.
   - The frontend shows dates in local time (as the existing counter does). Noon UTC is the same calendar day for every offset from UTC−11 to UTC+11, which covers Russia except UTC+12 (Kamchatka, which then sees the next day).
   - The "future" check accepts D ≤ the calendar date of `now + 14h` (UTC+14), so a reader east of UTC never gets their own today rejected. The date input sets `max` to the local today.
   - The lower bound is 1900-01-01.
   - End ≥ start is compared on the UTC calendar dates of the resulting values. An unedited status-recorded start such as 18:00 UTC is compared by its date, so picking the same day as the end is allowed.
3. **The «Прочитано» undo targets only the reading the status recorded.** Phase 1 deletes "the latest finished row". With manual edits that becomes wrong in two cases. First, a reader deletes the current read in the history and then presses «Убрать статус», which would delete an older read. Second, a reader adds a hand-entered read dated after the shelf add, and the undo would delete it. New rule: delete the latest finished row with `manual_end = false` and `finished_at >= added_at(read shelf item) - 1 minute`. If none qualifies, do nothing.
   - The one-minute slack covers phase-1 rows, whose `finished_at` (Go clock) and `shelf_items.added_at` (DB `now()`) come from the same transaction but different clocks.
   - Backfilled rows have `finished_at = added_at`.
   - `manual_end` is set when a row is created by `POST /reads` or its `finished_at`/`abandoned_at` is changed by `PATCH`. Changing only `started_at` does not set it. That way the open row, edited before finishing, still gets undone normally.
   - `RemoveBook` on the `read` shelf must read `added_at` inside the transaction **before** deleting the shelf item.
4. **History never changes shelves**, in both directions. The asymmetry is accepted: after deleting the last finished read, the book can be on «Прочитано» with `reads_count` 0. The counter is then hidden, and «Читаю снова» stays, because it depends on the status only.
5. **Concurrency without locks.** This follows phase 1, with SQLite-compatible SQL:
   - `PATCH` loads the row by `id AND user_id`, validates it, then runs `UPDATE ... WHERE id=? AND user_id=? AND <state predicate of the loaded state>`. Zero rows means the row was deleted (404) or changed state (409 `reading changed`). Tell them apart by re-reading the row.
   - `DELETE` runs `DELETE ... WHERE id=? AND user_id=? AND NOT (finished_at IS NULL AND abandoned_at IS NULL)`. With zero rows, re-read to choose 404 or 409.
   - The 100-row cap is check-then-insert in one transaction. A race can exceed it by one, which is harmless.
6. **Abuse.** No rate limit is needed. The only effect of mass inserts is on the caller's own counter, which is public on public shelves, and their own challenge. The cap of 100 rows per (user, book) bounds the table and the unpaginated list. The global 1 MiB body limit applies.
7. **API lives in the shelves module** (it owns `book_reads`) under a new top-level group `/api/v1/reads`. The alternative `/shelves/reads` would sit next to `/shelves/:id` and read like a shelf, so it was not used. The adaptation verdict (`screen-works/book-adaptations`) needs no spec change: its eligibility already reads "finished at least once", so hand-added or deleted readings grant or remove vote rights automatically.

## API

All routes are in `api.Group("/reads", authMiddleware)`: bearer required, 401 without one. The caller is always the token's user, and no route takes a user id. Errors are `{"error": "..."}`.

| Method | Path | Body | Success | Errors |
|---|---|---|---|---|
| GET | `/api/v1/reads?book_id=<uuid>` | — | 200 `{items: readResponse[]}` | 400 `invalid book_id` |
| POST | `/api/v1/reads` | `{book_id, started_on?: "YYYY-MM-DD"\|null, finished_on: "YYYY-MM-DD"}` | 201 `readResponse` | 400 (validation), 404 `book not found` (unknown/unpublished), 409 `too many readings` |
| PATCH | `/api/v1/reads/{id}` | any of `started_on`, `finished_on`, `abandoned_on` (`null` allowed only for `started_on` of a closed row) | 200 `readResponse` | 400 (validation, `field not editable`, `nothing to update`), 404 `reading not found`, 409 `reading changed` |
| DELETE | `/api/v1/reads/{id}` | — | 200 `{status: "ok"}` | 404 `reading not found`, 409 `open reading cannot be deleted` |

`readResponse` = `{id, book_id, state: "open"|"finished"|"abandoned", started_at, finished_at, abandoned_at}`, with times as RFC 3339 or `null`. `manual_end` is internal and not exposed. A malformed `{id}` returns 404.

Validation messages (400):
- `invalid date`: format, or a date before 1900-01-01;
- `date in the future`;
- `finish date before start date`;
- `finished_on is required`;
- `start date required for an open reading`;
- `field not editable`: an end date on an open row, or the other outcome's column.

Order of `GET`: `COALESCE(started_at, finished_at, abandoned_at) ASC, created_at ASC`.

Swagger annotations for all four handlers. Regenerate `backend/docs` the way the repo already does it (swag).

### Backend layering

- `shelves/domain`:
  - `BookRead` gains `ManualEnd bool` (`manual_end`);
  - a `ReadState` helper (open/finished/abandoned);
  - `DateOnly`-style parsing helpers live in the service.
- `shelves/repository` (add to `Repository`, all on the tx repo, SQLite-compatible):
  - `ListReads(ctx, userID, bookID) ([]domain.BookRead, error)`, ordered as above;
  - `GetRead(ctx, userID, readID) (*domain.BookRead, error)`, `gorm.ErrRecordNotFound` when missing or not owned;
  - `CountReads(ctx, userID, bookID) (int64, error)`;
  - `InsertFinishedRead(ctx, read *domain.BookRead) error`;
  - `UpdateReadDates(ctx, userID, readID, loadedState, fields) (int64, error)`, a conditional update;
  - `DeleteClosedRead(ctx, userID, readID) (int64, error)`;
  - `ShelfItemAddedAt(ctx, shelfID, bookID) (time.Time, bool, error)`;
  - change `DeleteLatestFinishedRead(ctx, userID, bookID, since time.Time)` to filter `manual_end = false AND finished_at >= since`.
- `shelves/service` (new file `reads.go`):
  - `ListReads`, `AddPastRead`, `UpdateRead`, `DeleteRead`;
  - errors `ErrReadNotFound`, `ErrInvalidDate`, `ErrDateInFuture`, `ErrFinishBeforeStart`, `ErrFinishRequired`, `ErrStartRequired`, `ErrFieldNotEditable`, `ErrNothingToUpdate` (reuse), `ErrOpenReadNotDeletable`, `ErrReadChanged`, `ErrTooManyReads`;
  - the injected `s.now` drives the future check (`s.now().Add(14*time.Hour)` date);
  - `AddPastRead` checks `BookExists` (published) like `AddBook`;
  - `RemoveBook` read branch: `ShelfItemAddedAt` → `remove` → `DeleteLatestFinishedRead(..., addedAt.Add(-time.Minute))`.
- `shelves/transport/http` (new `reads_handler.go`):
  - the PATCH body needs absent-vs-null. Use a small `optionalDate` value type whose `UnmarshalJSON` sets `Set=true`; `encoding/json` calls it for an explicit `null` too, but not for an absent key;
  - map service errors to the table above.
- `internal/transport/http/router.go`: register the group.
- Test fakes (`service/fake_journal_test.go`, `transport/http/fake_journal_test.go`) implement the new methods.

## Database

Migration `backend/migrations/000042_book_reads_manual_end.up.sql` (forward-only; do not edit 000041):

```sql
ALTER TABLE book_reads ADD COLUMN manual_end BOOLEAN NOT NULL DEFAULT false;
```

Existing rows get `false`, which is correct because all of them were recorded by status moves or the backfill. The existing indexes are enough, since history lists are per (user, book) and use `idx_book_reads_user_book_finished` / a small scan. Down: `ALTER TABLE book_reads DROP COLUMN IF EXISTS manual_end;` (completeness only, never run by agents). The SQLite test harness schema for `book_reads` (wherever the phase-1 tests create it) needs the column too.

## Frontend

- `packages/api-client`:
  - type `BookRead {id, bookId, state, startedAt, finishedAt, abandonedAt}`;
  - namespace `reads`: `list(bookId)`, `create({bookId, startedOn?, finishedOn})`, `update(id, {startedOn?, finishedOn?, abandonedOn?})`, which must keep `null` vs `undefined` (`undefined` keys are omitted, `null` is sent), and `remove(id)`;
  - snake↔camel mapping as in `shelves`.
- `entities/shelf`:
  - export `readsApi = api.reads`;
  - add `shelfKeys.readsForBook(userId, bookId)` = `[...shelfKeys.user(userId), 'reads', bookId]`. It sits under `shelfKeys.all`, so every existing `invalidateShelfContents` call (status toggles) also refreshes an open history list.
- `shared/lib`:
  - extend `format-read-count.ts`, or add `format-reading-history.ts` next to it, with a genitive month array («января» … «декабря»);
  - `formatReadingLine(index, read)`, which returns «N-е прочтение: …» per the spec, using local time and nominative months for ranges;
  - `toLocalDateInput(iso)` → `YYYY-MM-DD` (local) and `todayLocalDateInput()`;
  - unit tests for each line form, genitive months, the year boundary and null start.
- `features/shelve-book`:
  - `model/use-reading-history.ts`: a query on `readsForBook` with `enabled: isAuthenticated && expanded`, plus create/update/delete mutations. On success they call `invalidateShelfContents(queryClient)`, which covers the history, the `reads` summary on `forBook`, shelf pages, the challenge, home and `screenWorkKeys` for `can_vote`. Optimistic updates are not needed;
  - `ui/reading-history.tsx`: the disclosure button «История чтения» (`aria-expanded`), the list, the inline edit/add form with native `<input type="date" max={today}>`, client-side range validation and `window.confirm` for delete, following the existing `reviews-section` / `shelves-view` pattern. Error texts are as in the spec;
  - render it inside `BookShelfActions` below the counter, only in the signed-in and ready branch. Guests and the loading state show nothing. The feature does not import other features, and the view stays unchanged.
- PATCH from the form sends only changed fields. Clearing «Начало» on a closed row sends `startedOn: null`.

## Existing Patterns

- Transactional journal transitions and the SQLite-compatible conditional SQL: `shelves/service/service.go` (`AddBook`, `RemoveBook`, `journalOnAdd/OnRemove`) and `shelves/repository/repository.go` (journal methods).
- SQLite service/repository tests: `shelves/service/reading_journal_sqlite_test.go`, `shelves/repository/*_sqlite_test.go`.
- Ownership 404 vs 403: shelves use 403 for a foreign shelf, but this change uses **404** for foreign readings, as the spec requires, so nothing leaks.
- Cache invalidation: `features/shelve-book/model/invalidate-shelf-caches.ts`.
- Confirm dialogs: `window.confirm` in `widgets/reviews-section`, `views/shelves`.
- Russian month formatting: `shared/lib/format-read-count.ts`.

## Risks

- **Undo semantics change** (Decision 3) for rare cases only: the status-recorded row is still undone as before, and phase-1 rows qualify through the one-minute slack. Covered by tests for the backfill-like row (`finished_at = added_at`) and the Go-clock row.
- **UTC+12 display shift**: a hand-entered day shows as the next day in Kamchatka. Accepted. The challenge year is still the picked one.
- **Status/journal asymmetry**: «Прочитано» with 0 reads after a history delete. This is intended (owner decision) and specified.
- **Overlapping readings** (a hand-added read inside the open attempt's span) are allowed. No cross-row validation is done, to keep it minimal.
- **Public counters reveal edits**: `reads_count` on public shelf pages reflects hand-added reads. That matches "the counter stays public".
- The private «Брошено» is not delivered. «Брошено» stays a public default shelf (follow-up in ROADMAP Proposed).
