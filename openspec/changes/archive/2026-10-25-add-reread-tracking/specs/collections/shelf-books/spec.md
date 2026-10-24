## MODIFIED Requirements

### Requirement: One reading status
A book SHALL be on at most one of the default shelves of a user: «Хочу прочитать», «Читаю», «Прочитано» and «Брошено». Adding a book to a default shelf MUST, in the same transaction, remove it from the user's other default shelves and update the reading journal (see `collections/reading-journal`). Custom shelves are independent: a book can be on any number of them and on a default shelf at the same time.

#### Scenario: Moving to «Прочитано»
- **WHEN** a book on «Читаю» is added to «Прочитано»
- **THEN** it is on «Прочитано» only among the default shelves

#### Scenario: Custom shelf alongside a status
- **WHEN** a book on «Читаю» is added to the custom shelf «Отпуск»
- **THEN** it stays on «Читаю» and is also on «Отпуск»

#### Scenario: Abandoning a book in progress
- **WHEN** a book on «Читаю» is added to «Брошено»
- **THEN** it is on «Брошено» only among the default shelves

### Requirement: Shelving from the book page
The book page and book cards SHALL let a signed-in reader pick the reading status and toggle custom shelves («Добавить на полку»). Picking the current status again removes it. Changes apply immediately, and a failure shows «Ошибка обновления полки».
- **Status control on the book page.** The book page has a single status control instead of separate buttons. Its statuses are «Хочу прочитать», «Читаю», «Прочитано» and «Брошено».
  - **No status.** The control offers «Хочу прочитать», which shelves the book there at once. Next to it, «Выбрать статус» opens a chooser with all four statuses.
  - **With a status.** The control shows the current status as marked, with the icon of that status and a visible dropdown arrow, so it is clear that it opens the status chooser. In the chooser:
    - every status has its own icon;
    - the current status is marked with a check mark on the right;
    - «Убрать статус», set apart by a divider, removes the book from its status shelf.
  - **After a pick.** Picking closes the chooser.
- **Re-reading.** When the status is «Прочитано», the button «Читаю снова» is shown next to the status control. It moves the book to «Читаю» and starts a new reading (see `collections/reading-journal`). Picking «Читаю» in the chooser does the same. While the reader is re-reading, the status control shows «Читаю» with the mark «перечитываю»: the book is on «Читаю» and was finished at least once before.
- **Read counter.** When the reader has finished the book at least once, the page shows «Прочитано <N> <раз|раза|раз> · последний раз — <месяц> <год>» under the controls.
  - The word is «раза» when N ends in 2, 3 or 4 but not in 12, 13 or 14 (2, 3, 22 «раза»), and «раз» otherwise (1, 5, 11, 12, 21 «раз»).
  - The month is the lowercase nominative month name («январь» … «декабрь») of the latest finish, in the reader's local time.
  - The counter is not shown when the reader never finished the book.
- **Updates.** The counter, the «перечитываю» mark and «Читаю снова» update after every status change on the page, without a reload.
- **Custom shelves on the book page.** Custom shelves are behind the button «На полку», which opens the checklist «Добавить на полку». Toggling a shelf keeps the checklist open. When the reader has no custom shelves, «На полку» is not shown.
- **Loading.** While the reader's shelves are loading, placeholders of the same size MUST stand in place of the controls, so the controls never disappear and reappear. This applies on the book page and on cards.
- **Not signed in.** A visitor who is not signed in sees «Хочу прочитать» on the book page, leading to sign-in and back to the book. They see no «На полку», no «Читаю снова» and no read counter.
- **Book cards.** Book cards keep their separate status buttons «Хочу прочитать», «Читаю», «Прочитано»; they offer no «Брошено». A book on «Брошено» shows none of them as marked, and pressing one moves the book to that status.

#### Scenario: Changing the status
- **WHEN** a reader whose book is on «Хочу прочитать» picks «Читаю»
- **THEN** the book moves to «Читаю» and «Хочу прочитать» is no longer marked

#### Scenario: First status in one click
- **WHEN** a signed-in reader whose book has no status presses «Хочу прочитать» on the book page
- **THEN** the book is on «Хочу прочитать» and the control shows it as the current status

#### Scenario: Removing the status
- **WHEN** a reader whose book is on «Читаю» opens the status control and picks «Убрать статус»
- **THEN** the book is on none of the default shelves and the control offers «Хочу прочитать» again

#### Scenario: Custom shelves behind «На полку»
- **WHEN** a reader with the custom shelf «Отпуск» opens «На полку» and checks «Отпуск»
- **THEN** the book is added to «Отпуск», its status shelf is unchanged, and the checklist stays open

#### Scenario: Shelves still loading
- **WHEN** a signed-in reader opens a book page and their shelves have not loaded yet
- **THEN** placeholders are shown where the status control and «На полку» will be

#### Scenario: Failure rolls back
- **WHEN** moving a book to «Прочитано» fails
- **THEN** the previous status is marked again and «Ошибка обновления полки» is shown

#### Scenario: Starting a re-read
- **WHEN** a reader whose book is on «Прочитано», finished once in March 2025, presses «Читаю снова»
- **THEN** the control shows «Читаю» with «перечитываю», «Читаю снова» is gone, and the page still shows «Прочитано 1 раз · последний раз — март 2025»

#### Scenario: Finishing a re-read
- **WHEN** that reader then picks «Прочитано» in October 2026
- **THEN** the page shows «Прочитано 2 раза · последний раз — октябрь 2026», the mark «перечитываю» is gone and «Читаю снова» is offered

#### Scenario: Abandoning from the chooser
- **WHEN** a reader whose book is on «Читаю» picks «Брошено» in the status chooser
- **THEN** the control shows «Брошено» as the current status with its icon

#### Scenario: Undoing «Прочитано» on the book page
- **WHEN** a reader whose book shows «Прочитано 2 раза» picks «Убрать статус»
- **THEN** the control offers «Хочу прочитать» and the counter shows «Прочитано 1 раз» with the date of the earlier read

#### Scenario: Card on «Брошено»
- **WHEN** a book card shows a book that is on the reader's «Брошено»
- **THEN** none of its three status buttons is marked

## ADDED Requirements

### Requirement: Read counts in shelf book lists
Each book item in `GET /api/v1/shelves/{id}` and `GET /api/v1/shelves/public/{id}` SHALL carry `reads_count` and `last_read_at`:
- they come from the shelf owner's reading journal for that book (see `collections/reading-journal`), whoever makes the request;
- they are filled on every shelf type, default or custom;
- `reads_count` is 0 and `last_read_at` is `null` for a book the owner never finished.

The endpoints keep their authorization: the own-shelf endpoint is owner-only, and the public one serves public shelves only. On the reader's own shelf page `/shelves/{id}`, a book the reader finished at least once shows «Прочитано <N> <раз|раза|раз>», with the same plural rules as on the book page.

#### Scenario: Re-read book on the «Читаю» shelf page
- **WHEN** a reader re-reading a book they finished once opens their «Читаю» shelf
- **THEN** the book's item has `reads_count: 1` and shows «Прочитано 1 раз»

#### Scenario: Never finished
- **WHEN** a reader opens their «Хочу прочитать» shelf holding a book they never finished
- **THEN** the item has `reads_count: 0` and `last_read_at: null`, and no counter is shown
