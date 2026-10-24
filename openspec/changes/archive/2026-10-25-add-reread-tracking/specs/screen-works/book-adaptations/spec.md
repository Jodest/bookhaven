## MODIFIED Requirements

### Requirement: Adaptation vote endpoints
`PUT /api/v1/books/{id}/adaptations/{screen_work_id}/vote` with `{"value": "book" | "adaptation" | "same"}` SHALL require a signed-in user of any role. `book` means «Книга лучше», `adaptation` means «Экранизация лучше» and `same` means «Одинаково». A user has at most one vote per book and adaptation. The request creates the caller's vote, or replaces its value, and returns 200 with the caller's verdict item for that adaptation (see «Adaptation verdicts endpoint»).

To create or change a vote, the caller MUST have finished reading the book at least once (see `collections/reading-journal`), whatever the book's current status. A reader re-reading the book on «Читаю» may vote. Otherwise the response is 403 «only readers who have read the book can vote» and nothing changes.

Errors:
- no token returns 401;
- a malformed id returns 400 «invalid book id» or «invalid screen work id»;
- a missing or other `value` returns 400 «invalid vote value»;
- an unknown or unpublished book returns 404 «book not found»;
- a work that is not linked to the book, or whose link an admin hid, returns 404 «adaptation not found».

`DELETE /api/v1/books/{id}/adaptations/{screen_work_id}/vote` SHALL require a signed-in user of any role. It removes the caller's vote for that book and adaptation and returns 204, also when there was no vote. It does not check the reading journal, the shelf, the link or the book status. A malformed id returns 400. A user can never change or remove another user's vote.

#### Scenario: First vote
- **WHEN** a reader with the book on «Прочитано» sends `{"value": "book"}` for a visible adaptation of the published book
- **THEN** the response is 200 with `my_vote: "book"`, and `votes_count` has grown by one

#### Scenario: Changing a vote
- **WHEN** that reader then sends `{"value": "same"}` for the same adaptation
- **THEN** the response is 200 with `my_vote: "same"`, and `votes_count` is unchanged

#### Scenario: Book not read
- **WHEN** a signed-in user who has the book on «Хочу прочитать» and never finished it votes for one of its adaptations
- **THEN** the response is 403 «only readers who have read the book can vote» and no vote is stored

#### Scenario: Hidden link
- **WHEN** a reader votes for an adaptation whose link an admin hid
- **THEN** the response is 404 «adaptation not found»

#### Scenario: Invalid value
- **WHEN** a reader sends `{"value": "film"}`
- **THEN** the response is 400 «invalid vote value»

#### Scenario: Guest
- **WHEN** a request without a token sets a vote
- **THEN** the response is 401

#### Scenario: Removing a vote after leaving «Прочитано»
- **WHEN** a reader who voted has since moved the book to «Читаю» and deletes the vote
- **THEN** the response is 204 and the vote is gone

#### Scenario: Re-reader votes
- **WHEN** a reader who finished the book once and moved it back to «Читаю» to re-read it sends `{"value": "adaptation"}`
- **THEN** the response is 200 with `my_vote: "adaptation"`

#### Scenario: Read undone
- **WHEN** a reader who finished the book only once removes it from «Прочитано» and then votes
- **THEN** the response is 403 «only readers who have read the book can vote»

### Requirement: Adaptation verdicts endpoint
`GET /api/v1/books/{id}/adaptations/verdicts` SHALL be open to everyone, with an optional token. It returns 200 with `{can_vote, min_votes, items}`:
- `can_vote` is `true` only for a signed-in caller who has finished reading the book at least once (see `collections/reading-journal`), whatever its current status;
- `min_votes` is 5;
- `items` has one item per visible adaptation of the book, with `{screen_work_id, votes_count, book_better, adaptation_better, same, my_vote}`.

`votes_count` is the number of votes for that book and adaptation. `book_better`, `adaptation_better` and `same` are the vote counts per value when `votes_count` is at least `min_votes`, and MUST be `null` below it. `my_vote` is the caller's value, or `null` for a guest or a caller without a vote.

Hidden links are not returned. A malformed id returns 400 «invalid book id». An unknown or unpublished book returns 404 «book not found». A missing or invalid token is treated as a guest.

#### Scenario: Verdict revealed
- **WHEN** an adaptation of a published book has 18 votes «book», 3 votes «adaptation» and 2 votes «same»
- **THEN** its item has `votes_count: 23`, `book_better: 18`, `adaptation_better: 3` and `same: 2`

#### Scenario: Below the minimum
- **WHEN** an adaptation has 4 votes
- **THEN** its item has `votes_count: 4` and `null` for the three counts

#### Scenario: Guest
- **WHEN** a guest requests the verdicts
- **THEN** `can_vote` is `false` and every `my_vote` is `null`

#### Scenario: Reader of the book
- **WHEN** a signed-in user with the book on «Прочитано» who voted «adaptation» for one adaptation requests the verdicts
- **THEN** `can_vote` is `true` and that item has `my_vote: "adaptation"`

#### Scenario: Re-reader
- **WHEN** a signed-in user who finished the book once and is re-reading it on «Читаю» requests the verdicts
- **THEN** `can_vote` is `true`

### Requirement: Voting on the book page
Under the verdict, each adaptation SHALL offer voting according to the visitor:
- A guest sees the link «Войдите, чтобы сравнить книгу и экранизацию», which leads to the login page and back to the book.
- A signed-in user who may vote sees «Что лучше?» with the buttons «Книга лучше», «Экранизация лучше» and «Одинаково», and the note «Голосуя, вы отмечаете, что смотрели экранизацию.». The button of the reader's current vote is shown as selected. Clicking another button changes the vote, and clicking the selected one removes it. The buttons are disabled while a request runs. A failure shows «Не удалось сохранить голос.» and keeps the previous state.
- A signed-in user who may not vote and has no vote sees «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.».
- A signed-in user who may not vote but has a vote sees «Ваш голос: <Книга лучше|Экранизация лучше|Одинаково>» and the button «Убрать голос».

A user may vote once they have finished the book at least once, whatever its current status. After a vote changes, the verdict line updates without reloading the page. When the reader changes the book's reading status on the same page, the voting controls update without a reload.

#### Scenario: Reader votes
- **WHEN** a reader with the book on «Прочитано» clicks «Экранизация лучше» under an adaptation
- **THEN** that button is shown as selected and the verdict line reflects the new vote

#### Scenario: Reader removes the vote
- **WHEN** the reader clicks the selected «Экранизация лучше» again
- **THEN** no button is selected and the vote is removed

#### Scenario: Guest prompt
- **WHEN** a guest opens a book with an adaptation
- **THEN** the adaptation shows its verdict line and «Войдите, чтобы сравнить книгу и экранизацию», and no voting buttons

#### Scenario: Not read yet
- **WHEN** a signed-in user who never finished the book opens it
- **THEN** each adaptation shows «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.»

#### Scenario: Marking the book as read on the page
- **WHEN** that user marks the book «Прочитано» with the shelf buttons of the same page
- **THEN** the voting buttons appear under each adaptation without a reload

#### Scenario: Vote kept after leaving «Прочитано»
- **WHEN** a reader who voted «Книга лучше» and finished the book only once removes it from «Прочитано» and looks at the adaptation
- **THEN** it shows «Ваш голос: Книга лучше» and «Убрать голос», and no voting buttons

#### Scenario: Re-reading keeps the voting buttons
- **WHEN** a reader who voted «Книга лучше» presses «Читаю снова» on the same page
- **THEN** the voting buttons stay, with «Книга лучше» selected
