# Tasks

## 1. Backend: schema and backfill

- [x] 1.1 Add `backend/migrations/000041_book_reads.up.sql`: table, CHECK, partial unique open index, two finished indexes, and the backfill from `read` / `reading` / `abandoned` shelf items (design.md → Database). Add `000041_book_reads.down.sql` with `DROP TABLE IF EXISTS book_reads;`. Do not edit earlier migrations. Verify by applying it with the normal forward `migrate up` on the local stack. Never run `migrate down`, drop or reset the DB, and never start a second compose stack.
- [x] 1.2 Check after `migrate up`: the count of finished rows equals the `read` shelf items, and the count of open rows equals the `reading` shelf items (read-only SELECTs).

## 2. Backend: shelves module (journal + «Брошено»)

- [x] 2.1 Domain: `BookRead` and `ReadSummary`/`ReadCount` types. Make `abandoned` a default type: `isDefaultShelf`, `otherDefaultTypes` returns the other three, `EnsureDefaultShelves` inserts «Брошено», and the `ListByUser` order puts abandoned 4th.
- [x] 2.2 Repository: journal methods (open / finish-or-insert / abandon-open / delete-open / delete-latest-finished / summary / batched counts), all usable on the tx repository and SQLite-compatible (no advisory locks, no `FOR UPDATE`).
- [x] 2.3 Service: apply the transition table (design.md Decision 3) in `AddBook` (default branch) and in a new transactional default-shelf branch of `RemoveBook`. The shelf mutation goes first, and the journal step runs only after it succeeded. `ListShelves` returns the summary for `book_id`. `GetShelf`/`GetPublicShelf` return per-book counts of the shelf owner.
- [x] 2.4 Transport: `reads` on `GET /shelves?book_id`, plus `reads_count`/`last_read_at` on items of `GET /shelves/{id}` and `GET /shelves/public/{id}`. Update the swagger annotations.
- [x] 2.5 Tests (service + SQLite repository + handler):
  - every row of the transition table;
  - a double add to read → 409 and still one finished row;
  - a double remove → 404 and only one row deleted;
  - undo deletes only the latest finished;
  - abandoned closes the open attempt;
  - want_to_read deletes the open attempt;
  - custom shelves are a no-op;
  - at most one open row;
  - the summary (`rereading` true/false, null `last_read_at`);
  - public shelf counts use the owner, not the caller;
  - the default list order includes «Брошено», and a 4th shelf is created for an existing user;
  - deleting «Брошено» → 400.
- [x] 2.6 Update `cmd/seed/demo_readers.go` (and any shared test fixtures) so seeded read/reading books also get `book_reads` rows.
- [x] 2.7 Verify: `go build ./...`, `go vet ./...`, `go test ./internal/modules/shelves/...` from `backend/`. Run gofmt on the changed files only.

## 3. Backend: consumers

- [x] 3.1 `reading_challenges/repository.CountReadBooks`: count distinct `book_id` from `book_reads` with `finished_at` in the UTC year, and update the doc comment. Tests: a re-read in a new year counts in both years, twice in one year counts once, read→reading keeps the count, undo lowers only the latest year.
- [x] 3.2 `screenworks/repository.IsReader`: at least one finished `book_reads` row. Update the existing vote and verdict tests that seed the «Прочитано» shelf so they seed `book_reads`. Add: a re-reader on «Читаю» can vote / `can_vote` true; after the only read is undone → 403.
- [x] 3.3 Verify: `go build ./...`, `go vet ./...`, `go test ./...` from `backend/` (the touched packages at minimum: shelves, reading_challenges, screenworks, home, curated).

## 4. Frontend: apps/web + api-client

- [x] 4.1 `packages/api-client`: types for `reads` on the shelves list response and `readsCount`/`lastReadAt` on shelf book items.
- [x] 4.2 `shared/lib`: `formatReadCount` / `formatReadCounter` built on `plural-ru.ts`, with nominative lowercase month names. Unit tests for 1/2/5/11/12/21/22 and for the month/year text.
- [x] 4.3 `features/shelve-book`:
  - status set of 4 in the hook (exclusivity, chooser) with label «Брошено» and an icon; cards stay at 3 buttons;
  - the hook exposes `reads`;
  - `book-shelf-actions.tsx`: «Брошено» in the chooser, «Читаю снова» when on «Прочитано», the «перечитываю» mark, and the counter line.

  Tests: chooser has 4 statuses; «Читаю снова» shows and moves to reading; mark shown when `rereading`; counter text; guest sees none of them; card buttons stay 3 and none is marked for an abandoned book.
- [x] 4.4 Views:
  - `shelves-view` default order includes `abandoned`;
  - `public-profile` and `public-shelf-detail` label maps get «Брошено»;
  - `shelf-detail` and `public-shelf-detail` show «Прочитано N раз» for `readsCount >= 1`.

  Update or extend the existing view tests. Respect FSD: no view-to-view or feature-to-feature imports.
- [x] 4.5 Verify: `tsc --noEmit` and `next lint` in `apps/web` (and typecheck `packages/api-client`). Run the touched vitest suites. Do not run a native `next build`; CI is the typedRoutes gate.

## 5. Docs

- [x] 5.1 `docs/ROADMAP.md`:
  - mark the re-read tracking part of #204 as delivered by this change;
  - add a phase-2 item: manual editing of start/finish dates, a reading-history list UI, and optionally a private «Брошено».
- [x] 5.2 Verify the change folder with `npx -y @fission-ai/openspec@1.13.2 validate add-reread-tracking --strict` (OPENSPEC_TELEMETRY=0).
