# Proposal

Completes GitHub issue #204 ("Explicit start/finish reading dates and re-read tracking"), phase 2 of epic #658. Phase 1 (`add-reread-tracking`, PR #662) shipped the journal and the read counter. After this change #204 can be closed. A private «Брошено» is out of scope and becomes a separate Proposed ROADMAP item.

## Why

Readers cannot fix the dates the status buttons record, cannot add books they read before BookHaven (for example in 2015), and cannot see or clean up their reading history. The counter and the yearly challenge therefore stay wrong as soon as a status was clicked at the wrong time.

## What Changes

- New owner-only reading-history API under `/api/v1/reads` (bearer required): list the caller's readings of a book, add a past finished reading with dates, edit dates, delete a reading. Other users' readings answer 404.
- Editing rules:
  - An open reading can only change its start date. It is finished or abandoned through the status control, as before.
  - Finished and abandoned readings can change their start date (optional) and their end date (required). A reading cannot switch between finished and abandoned.
  - The open reading cannot be deleted from the history (409). It follows the «Читаю» status.
- Validation: dates are calendar days from 1900-01-01 up to today. The end date is not earlier than the start date, and the start date is optional for finished readings. A book holds at most 100 readings.
- Date rule: a picked day is stored as 12:00 UTC of that day, so it counts for the challenge in that day's year and shows as that day in every Russian time zone.
- Adding, editing or deleting a reading never changes the shelves. Deleting the only finished reading of a book on «Прочитано» keeps the status, and the counter goes down.
- **BREAKING (behavior)**: «Убрать статус» on «Прочитано» now deletes only the reading that this status recorded. It no longer deletes one whose finish date was entered or changed by hand, or one finished before the book was put on «Прочитано». So after deleting the current read from the history, the undo no longer deletes an older read.
- Book page: an expandable «История чтения» list for the signed-in owner («1-е прочтение: март 2019 — апрель 2019», «2-е прочтение: …, брошено», «3-е прочтение: с мая 2026 — читаю»), with «Изменить», «Удалить» (with confirmation) and «Добавить прочтение». The counter, «перечитываю», the challenge, the home card and adaptation vote rights refresh without a reload. Visitors still see only the public counters.
- ROADMAP: #204 is moved to Up Next as completed, and a private «Брошено» gets its own Proposed line.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `collections/reading-journal`: the journal can be edited by its owner, the undo rule for «Прочитано» changes, and new requirements cover the reading-history API, the date rules and the book-page history list.
- `reading-challenges/progress`: the count follows hand-added, re-dated and deleted readings, a picked day counts for its own year, and the home card refreshes after history edits.

## Impact

- backend: shelves module (journal repository/service, new `/api/v1/reads` handlers, router, undo rule in `RemoveBook`), migration `000042_book_reads_manual_end`, swagger.
- frontend: apps/web `features/shelve-book` (history list, add/edit forms, delete), `entities/shelf` (query key, `readsApi`), `shared/lib` date and history formatters; `packages/api-client` (`reads` namespace and types).
- devops: none (the migration runs with the normal migrate step).
- docs: ROADMAP (#204 done, private «Брошено» proposed), swagger annotations.
