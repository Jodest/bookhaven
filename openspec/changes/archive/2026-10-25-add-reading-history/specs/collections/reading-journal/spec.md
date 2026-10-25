## MODIFIED Requirements

### Requirement: Reading journal
The system SHALL keep, for every user and book, a journal of reading attempts that is separate from the shelves. The default shelf holding the book is its current status, and the journal is its history. Each attempt is in one of three states:
- open: being read;
- finished: it has a finish time;
- abandoned: it has an abandon time.

An attempt MAY have a start time. An attempt MUST NOT be both finished and abandoned, and a user MUST have at most one open attempt per book. Only two things change the journal: the status transitions in «Status transitions update the journal», and the owner's reading-history requests («Adding a past reading», «Editing reading dates», «Deleting a reading»). Nothing ever changes another user's journal. Deleting a user or a book deletes its attempts. Unpublishing a book leaves them unchanged.

#### Scenario: One open attempt at most
- **WHEN** a reader whose book is on «Читаю» sends the request to add it to «Читаю» again
- **THEN** the response is 409 and the reader still has exactly one open attempt for the book

#### Scenario: Account deleted
- **WHEN** a user with finished attempts deletes their account
- **THEN** their attempts are gone and no longer count anywhere

#### Scenario: History edits never touch the shelves
- **WHEN** a reader whose book is on «Хочу прочитать» adds a past reading of it
- **THEN** the book is still on «Хочу прочитать» only, and the reader has one more finished attempt

### Requirement: Status transitions update the journal
Adding a book to a default shelf or removing it from one (`POST /api/v1/shelves/{id}/books`, `DELETE /api/v1/shelves/{id}/books/{bookId}`) SHALL update the caller's journal for that book in the same transaction as the shelf change. The journal MUST change only when the shelf change itself succeeds: a request answered 404 or 409 leaves the journal untouched, so repeating a request (a double click) never records a second attempt. Moving a book to a status changes the journal as follows:
- **To «Читаю».** An attempt starting now is opened, unless one is already open. Earlier attempts are kept.
- **To «Прочитано».** The open attempt is finished now. Without an open attempt, a finished attempt without a start time is recorded.
- **To «Брошено».** The open attempt is closed as abandoned now. Without an open attempt, nothing changes.
- **To «Хочу прочитать».** The open attempt, if there is one, is deleted. Finished and abandoned attempts are kept.

Removing a book from a status changes the journal as follows:
- **From «Читаю».** The open attempt is deleted.
- **From «Прочитано».** This is an undo of the reading that putting the book on «Прочитано» recorded. The system deletes the most recently finished attempt that meets both conditions:
  - it finished when the book was put on «Прочитано» or later;
  - its finish date was never entered or changed by hand in the reading history.

  If no attempt qualifies (for example, the reader already deleted or re-dated that reading in the history), the journal is unchanged. Earlier attempts are always kept, and the book is then on no default shelf.
- **From «Хочу прочитать» or «Брошено».** The journal is unchanged.

Custom shelves never change the journal.

#### Scenario: Re-reading keeps the earlier read
- **WHEN** a reader with one finished attempt moves the book from «Прочитано» to «Читаю»
- **THEN** the book is on «Читаю», the finished attempt is kept, and a new open attempt starting now exists

#### Scenario: Finishing a re-read
- **WHEN** that reader then moves the book to «Прочитано»
- **THEN** the open attempt is finished and the reader has two finished attempts

#### Scenario: Double click on «Прочитано»
- **WHEN** two requests to add the same book to «Прочитано» arrive one right after the other
- **THEN** one succeeds with 201, the other gets 409, and exactly one finished attempt was recorded

#### Scenario: Undoing «Прочитано»
- **WHEN** a reader with two finished attempts of a book, the later one recorded by moving the book to «Прочитано», removes it from «Прочитано»
- **THEN** only the later finished attempt is deleted, one finished attempt remains, and the book has no reading status

#### Scenario: Marking read without reading first
- **WHEN** a reader moves a book from «Хочу прочитать» straight to «Прочитано»
- **THEN** a finished attempt without a start time is recorded

#### Scenario: Abandoned on the second reading
- **WHEN** a reader with one finished attempt re-reads the book and then moves it to «Брошено»
- **THEN** the open attempt is closed as abandoned, the finished attempt is kept, and the read count stays 1

#### Scenario: Postponing a re-read
- **WHEN** a reader re-reading a book moves it to «Хочу прочитать»
- **THEN** the open attempt is deleted and the earlier finished attempts are kept

#### Scenario: Removing from «Читаю»
- **WHEN** a reader removes a book from «Читаю»
- **THEN** its open attempt is deleted and its finished attempts are kept

#### Scenario: Custom shelf
- **WHEN** a reader adds a book to the custom shelf «Отпуск»
- **THEN** the journal of that book is unchanged

#### Scenario: Undo after the current read was deleted from the history
- **WHEN** a reader finished a book in 2025 and again in 2026 (moving it to «Прочитано» in 2026), deletes the 2026 reading in the history, and then removes the book from «Прочитано»
- **THEN** the 2025 reading is kept, the read count stays 1, and the book has no reading status

#### Scenario: Undo keeps a re-dated reading
- **WHEN** a reader moves a book to «Прочитано», changes that reading's finish date to 10 September 2026 in the history, and then removes the book from «Прочитано»
- **THEN** the reading finished on 10 September 2026 is kept and the book has no reading status

#### Scenario: Undo skips a past reading added by hand
- **WHEN** a reader whose book has been on «Прочитано» since 2024 adds a past reading finished in March 2025 and then removes the book from «Прочитано»
- **THEN** the reading recorded in 2024 is deleted and the March 2025 reading is kept

## ADDED Requirements

### Requirement: Reading history list
`GET /api/v1/reads?book_id=<id>` SHALL return `{items}`: every attempt of the caller for that book. The endpoint requires a bearer token (401 without one), and it only ever returns the caller's own attempts. There is no way to read another user's history. A missing or malformed `book_id` returns 400. A book without attempts, or an unknown book, returns `{items: []}`. Attempts of an unpublished book are still listed.

Each item is `{id, book_id, state, started_at, finished_at, abandoned_at}`. `state` is `open`, `finished` or `abandoned`, and the times are RFC 3339 or `null`. Items are ordered oldest first by start time, or by end time (finish or abandon) when there is no start time. Ties are broken by creation order.

#### Scenario: Three readings
- **WHEN** a reader who finished a book in 2019, abandoned it in 2023 and has been re-reading it since May 2026 requests its history
- **THEN** the items are, in order, the finished, the abandoned and the open attempt, with `state` `finished`, `abandoned`, `open`

#### Scenario: Another user's history cannot be requested
- **WHEN** a reader requests the history of a book that only another user has read
- **THEN** the response is `{items: []}`

#### Scenario: Not signed in
- **WHEN** a request without a token is sent to the history endpoint
- **THEN** the response is 401

### Requirement: Hand-entered dates
Dates entered in the reading history SHALL be calendar days `YYYY-MM-DD`. A day D is stored as 12:00 UTC of D. It therefore counts for the reading challenge in the year of D, and the site shows it as D in every Russian time zone. A date is rejected with 400 when:
- it is not a valid `YYYY-MM-DD` day;
- it is earlier than 1900-01-01;
- it is later than the current day in the earliest time zone (UTC+14), so a reader's own «today» is always accepted.

The end date (finish or abandon) MUST NOT be earlier than the start date. Both are compared as UTC calendar days, so the same day is allowed. A violation is answered 400. Error bodies are `{"error": "<message>"}`, and nothing is changed.

#### Scenario: Date in the future
- **WHEN** a reader sends a finish date one week ahead
- **THEN** the response is 400 and nothing is changed

#### Scenario: Finish before start
- **WHEN** a reader sends a start date of 10 May 2020 and a finish date of 1 May 2020
- **THEN** the response is 400 and nothing is changed

#### Scenario: Same day
- **WHEN** a reader sends 3 May 2020 as both start and finish date
- **THEN** the request succeeds

#### Scenario: Last day of the year
- **WHEN** a reader in UTC+3 enters a finish date of 31 December 2015
- **THEN** the history line shows «декабрь 2015», the edit form shows 31 December 2015, and the reading counts for 2015

### Requirement: Adding a past reading
`POST /api/v1/reads` `{book_id, started_on?, finished_on}` SHALL record a finished attempt for the caller and return it with 201. `finished_on` is required. `started_on` is optional and may be `null`. The rules are:
- The book must exist and be published, otherwise the response is 404.
- The dates follow «Hand-entered dates».
- A book already holding 100 attempts of the caller returns 409.
- The request never changes the caller's shelves or their other attempts, and it never opens or closes an attempt.

The endpoint requires a bearer token (401). Readings added this way are not removed by the «Прочитано» undo.

#### Scenario: Reading from before BookHaven
- **WHEN** a reader whose book has no status adds a reading finished on 20 April 2015
- **THEN** the response is 201, the book still has no status, and `reads.reads_count` of the book goes up by one with `last_read_at` in April 2015 if that is the latest finish

#### Scenario: Added while re-reading
- **WHEN** a reader whose book is on «Читаю» with no finished attempt adds a past reading
- **THEN** the book is still on «Читаю», its open attempt is unchanged, and `reads.rereading` is `true`

#### Scenario: Unpublished book
- **WHEN** a reader adds a past reading of a book that is not published
- **THEN** the response is 404

### Requirement: Editing reading dates
`PATCH /api/v1/reads/{id}` SHALL change the dates of one of the caller's attempts and return the updated attempt with 200. Fields left out stay unchanged. The editable fields depend on the state:
- **Open.** Only `started_on`, which cannot be `null`. An open attempt is finished or abandoned only through the status control.
- **Finished.** `started_on`, which may be `null` to clear it, and `finished_on`, which cannot be `null`.
- **Abandoned.** `started_on`, which may be `null`, and `abandoned_on`, which cannot be `null`.

The rules are:
- An attempt never changes its state: sending `finished_on` for an abandoned attempt, `abandoned_on` for a finished one, or an end date for an open one returns 400.
- A body without any of the fields returns 400.
- The dates follow «Hand-entered dates», checked against the attempt's resulting start and end.
- Changing a finish date by hand removes the attempt from the «Прочитано» undo.
- An unknown attempt, or one belonging to another user, returns 404. The response does not reveal whether it exists.
- If the attempt changed state between reading it and saving the edit (for example, the book was moved to «Прочитано» at that moment), the response is 409 and nothing is changed.
- Editing never changes the shelves.

The endpoint requires a bearer token (401).

#### Scenario: Correcting a finish date
- **WHEN** a reader changes the finish date of a reading recorded on 3 January 2026 to 28 December 2025
- **THEN** the response is 200 and the reading counts for 2025 instead of 2026

#### Scenario: Correcting the start of the current reading
- **WHEN** a reader whose book is on «Читаю» changes the start date of the open attempt to 1 May 2026
- **THEN** the attempt is still open with that start date, and the book is still on «Читаю»

#### Scenario: Finishing through the history is refused
- **WHEN** a reader sends `finished_on` for an open attempt
- **THEN** the response is 400 and the attempt is still open

#### Scenario: Turning an abandoned reading into a finished one is refused
- **WHEN** a reader sends `finished_on` for an abandoned attempt
- **THEN** the response is 400 and the attempt is unchanged

#### Scenario: Someone else's reading
- **WHEN** a reader sends an edit for an attempt id of another user
- **THEN** the response is 404 and the attempt is unchanged

### Requirement: Deleting a reading
`DELETE /api/v1/reads/{id}` SHALL delete one finished or abandoned attempt of the caller and return 200. The open attempt MUST NOT be deleted this way: the response is 409 and the attempt is kept. It disappears when the reader changes the «Читаю» status. Deleting never changes the shelves. Other rules:
- An unknown attempt, or one belonging to another user, returns 404. The response does not reveal whether it exists.
- The endpoint requires a bearer token (401).

#### Scenario: Deleting the only read of a book on «Прочитано»
- **WHEN** a reader whose book is on «Прочитано» with one finished attempt deletes that attempt
- **THEN** the book stays on «Прочитано» and `reads` is `{reads_count: 0, last_read_at: null, rereading: false}`

#### Scenario: Deleting the open reading is refused
- **WHEN** a reader whose book is on «Читаю» sends a delete for its open attempt
- **THEN** the response is 409 and the open attempt is kept

#### Scenario: Deleting someone else's reading
- **WHEN** a reader sends a delete for an attempt id of another user
- **THEN** the response is 404 and the attempt is kept

### Requirement: Reading history on the book page
On the book page, a signed-in reader SHALL see the link «История чтения» under the shelving controls. It expands a list of their own attempts for the book, in the order of «Reading history list». Visitors who are not signed in never see it, and the history is never shown on public pages: there, only the existing counters are public. The list loads when it is first expanded. Each attempt is one line «<N>-е прочтение: <period>», numbered from 1. Months are shown as lowercase month and year in the reader's local time:
- finished, with a start: «март 2019 — апрель 2019»;
- finished, without a start: «апрель 2019»;
- abandoned: the same forms followed by «, брошено»;
- open: «с <month in genitive> <year> — читаю», for example «с мая 2026 — читаю».

Every line has the actions «Изменить» and «Удалить»; the open attempt has only «Изменить».
- **Editing.** «Изменить» opens an inline form with date fields that cannot go past today:
  - «Начало», which is optional except for the open attempt;
  - «Окончание», for a finished attempt, or «Брошено», for an abandoned one.

  «Сохранить» sends only the changed fields, and «Отмена» closes the form.
- **Deleting.** «Удалить» asks «Удалить это прочтение из истории? Статус книги не изменится.» and deletes only after confirmation.
- **Adding.** «Добавить прочтение» at the end of the list opens the same form with «Начало» (optional) and «Окончание» (required), and the button «Добавить».
- **Empty list.** Without attempts, the list shows «Вы ещё не отмечали прочтения этой книги.» and «Добавить прочтение».
- **Errors.** The form refuses an end date earlier than the start date with «Дата окончания не может быть раньше даты начала» before sending. A failed request shows «Не удалось сохранить прочтение», or «Не удалось удалить прочтение» for a deletion, and the list stays as before.
- **Updates.** After every successful add, edit or deletion, the following update without a page reload: the list, the read counter, the «перечитываю» mark, the reader's shelf pages, the reading-challenge figures, the home challenge card and the adaptation vote rights on the book page.

#### Scenario: Owner sees the history
- **WHEN** a reader who finished a book in March–April 2019, abandoned it in 2023 and has been re-reading it since May 2026 expands «История чтения»
- **THEN** the lines are «1-е прочтение: март 2019 — апрель 2019», «2-е прочтение: …, брошено» and «3-е прочтение: с мая 2026 — читаю», and the third line has no «Удалить»

#### Scenario: Visitor sees no history
- **WHEN** a visitor who is not signed in opens the book page
- **THEN** there is no «История чтения»

#### Scenario: Adding a past reading on the page
- **WHEN** a reader with no reading of a book adds one with «Окончание» 20 April 2015
- **THEN** the list shows «1-е прочтение: апрель 2015» and the page shows «Прочитано 1 раз · последний раз — апрель 2015» without a reload

#### Scenario: Deleting the last read on the page
- **WHEN** a reader whose book is on «Прочитано» with one reading confirms «Удалить» on it
- **THEN** the status control still shows «Прочитано», the read counter is no longer shown, and the list shows «Вы ещё не отмечали прочтения этой книги.»

#### Scenario: Cancelled deletion
- **WHEN** a reader presses «Удалить» and dismisses the confirmation
- **THEN** nothing is deleted

#### Scenario: Invalid range in the form
- **WHEN** a reader enters «Начало» 10 May 2020 and «Окончание» 1 May 2020
- **THEN** the form shows «Дата окончания не может быть раньше даты начала» and sends nothing
