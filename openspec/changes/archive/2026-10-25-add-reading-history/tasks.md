# Tasks

## 1. Backend: schema

- [x] 1.1 Add `backend/migrations/000042_book_reads_manual_end.up.sql` (`ALTER TABLE book_reads ADD COLUMN manual_end BOOLEAN NOT NULL DEFAULT false;`) and `.down.sql` (`DROP COLUMN IF EXISTS manual_end`). Do not edit 000041. Apply it with the normal forward `migrate up` on the local stack. Never run `migrate down`, drop or reset the DB, and never start a second compose stack.
- [x] 1.2 Add `manual_end` to the `book_reads` schema used by the SQLite test harness of the shelves module (and any other package that creates `book_reads` in SQLite: reading_challenges, screenworks).

## 2. Backend: shelves module

- [x] 2.1 Domain: `BookRead.ManualEnd` and a read-state helper (open / finished / abandoned).
- [x] 2.2 Repository: `ListReads`, `GetRead`, `CountReads`, `InsertFinishedRead`, `UpdateReadDates` (conditional on the loaded state), `DeleteClosedRead`, `ShelfItemAddedAt`. Change `DeleteLatestFinishedRead` to take `since` and skip `manual_end` rows (design.md, Decision 3). All of them must be SQLite-compatible: no `FOR UPDATE`, no advisory locks.
- [x] 2.3 Service `reads.go`: `ListReads` / `AddPastRead` / `UpdateRead` / `DeleteRead` with the date rules (`YYYY-MM-DD` → 12:00 UTC, ≥ 1900-01-01, ≤ date of `now+14h`, end ≥ start on UTC dates), the editable-field rules per state, 409 for deleting the open row, the 100-row cap, the published-book check on add, and `manual_end` set on add and on an end-date edit. Change the `RemoveBook` read branch: read `added_at` before the delete, then call `DeleteLatestFinishedRead(since = added_at - 1m)`.
- [x] 2.4 Transport: new `reads_handler.go` (GET/POST/PATCH/DELETE, the absent-vs-null `optionalDate` type for PATCH, error mapping per the design.md API table, foreign or unknown id → 404). Register `api.Group("/reads", authMiddleware)` in `internal/transport/http/router.go`. Add swagger annotations and regenerate `backend/docs` the usual way.
- [x] 2.5 Tests (service + SQLite + handler). Update the fakes in `service/fake_journal_test.go` and `transport/http/fake_journal_test.go`.
  - **List.** It returns only the caller's rows, in the spec order. A malformed `book_id` → 400, no token → 401.
  - **Add.** 201 for a finished row with and without a start. The shelves are unchanged, and an open row stays open. Unpublished book → 404, the 101st row → 409.
  - **Dates.** A future date → 400, but `today+0` at UTC+14 is accepted (clock injected). Pre-1900 → 400. End before start → 400, the same day is OK. The 31 Dec 2015 day is stored as `2015-12-31T12:00Z`.
  - **Edit.** On an open row only `started_on` is accepted (end → 400, null → 400). On a closed row, start null clears it, and the other outcome → 400. An empty body → 400. A foreign id → 404. A state change between load and update → 409.
  - **Delete.** A closed row → 200. The open row → 409. A foreign id → 404. Deleting the only finished row of a book on «Прочитано» keeps the shelf and gives `reads_count` 0.
  - **Undo.** Every existing phase-1 transition test still passes. After the current read is deleted in the history, the undo deletes nothing. A re-dated row is kept. A hand-added read dated after the shelf add is kept, and the status-recorded one is deleted. A row with `finished_at = added_at` (backfill) is undone, and so is a row whose Go-clock time is slightly before `added_at`.
- [x] 2.6 Consumers, tests only: in `reading_challenges`, a hand-added 2015 read counts for 2015 and re-dating moves the year. In `screenworks`, a hand-added read grants `can_vote`, and deleting the only read removes it (403 on vote).
- [x] 2.7 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./internal/modules/shelves/... ./internal/modules/reading_challenges/... ./internal/modules/screenworks/...`, then `go test ./...`. Run gofmt on changed files only.

## 3. Frontend: api-client and apps/web

- [x] 3.1 `packages/api-client`: the `BookRead` type and the `reads` namespace (`list`, `create`, `update` keeping `null` vs `undefined`, `remove`) with snake/camel mapping. Typecheck the package.
- [x] 3.2 `entities/shelf`: export `readsApi` and add `shelfKeys.readsForBook(userId, bookId)` under `shelfKeys.all`.
- [x] 3.3 `shared/lib`: genitive month names, `formatReadingLine` (finished with or without a start, abandoned «, брошено», open «с <мая> <год> — читаю», «N-е прочтение: »), and the `toLocalDateInput` / `todayLocalDateInput` helpers. Unit tests for every form, all 12 genitive months and a null start.
- [x] 3.4 `features/shelve-book`: `model/use-reading-history.ts` (query enabled only while expanded; mutations call `invalidateShelfContents` on success) and `ui/reading-history.tsx`:
  - the disclosure «История чтения»;
  - the lines with «Изменить» / «Удалить», with no «Удалить» on the open row;
  - the inline form with date inputs (`max` = local today), «Начало» optional except on the open row, «Окончание» / «Брошено»;
  - «Добавить прочтение» and the empty text «Вы ещё не отмечали прочтения этой книги.»;
  - the confirmation «Удалить это прочтение из истории? Статус книги не изменится.»;
  - the error texts from the spec.

  Render it in `BookShelfActions` for signed-in, loaded readers only. Respect FSD: no feature-to-feature or view-to-view imports.
- [x] 3.5 Tests (vitest + RTL):
  - the guest sees no «История чтения»;
  - the list loads on expand and shows the three example lines;
  - the open row has no «Удалить»;
  - add sends `finished_on`, and the counter refreshes through invalidation (mocked API);
  - edit sends only the changed fields, and clearing the start sends `null`;
  - an invalid range shows the error and sends nothing;
  - a cancelled confirm sends nothing;
  - a confirmed delete calls `remove` and invalidates the shelf, challenge, home and screen-work keys;
  - a failed request shows «Не удалось сохранить прочтение» / «Не удалось удалить прочтение».

  Existing `book-shelf-actions` tests still pass.
- [x] 3.6 Verify: `tsc --noEmit` and `next lint` in `apps/web`, typecheck `packages/api-client`, and run the touched vitest suites. Do not run a native `next build`; CI is the typedRoutes gate.

## 4. Docs

- [x] 4.1 `docs/ROADMAP.md`:
  - remove the Proposed line «Reading journal, phase 2 - manual editing of start/finish dates, …»;
  - add a completed Up Next entry after «Re-read tracking»: `- [x] Reading history - phase 2 of epic #658 (completes #204): owner-only reading-history API /api/v1/reads (list / add past finished reading / edit dates / delete; open reading only changes its start and cannot be deleted; outcome is fixed), dates are days stored at 12:00 UTC (≤ today, ≥ 1900, end ≥ start), the «Прочитано» undo deletes only the reading the status recorded (migration 000042_book_reads_manual_end), book page «История чтения» with edit/delete/«Добавить прочтение»; OpenSpec change add-reading-history`;
  - in the «Re-read tracking» entry, replace «Phase 2 stays in Proposed» with «Phase 2 done, see the next entry»;
  - add a Proposed line: `- [ ] Private «Брошено» shelf - let a reader hide their «Брошено» status from the public profile and public shelves (deferred from #204 phase 2; today every default shelf is public) [code]`.
- [x] 4.2 Validate the change folder: `npx -y @fission-ai/openspec@1.13.2 validate add-reading-history --strict` (OPENSPEC_TELEMETRY=0).
