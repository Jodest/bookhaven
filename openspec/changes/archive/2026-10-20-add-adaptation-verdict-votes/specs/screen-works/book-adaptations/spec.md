## ADDED Requirements

### Requirement: Adaptation vote endpoints
`PUT /api/v1/books/{id}/adaptations/{screen_work_id}/vote` with `{"value": "book" | "adaptation" | "same"}` SHALL require a signed-in user of any role. `book` means «Книга лучше», `adaptation` means «Экранизация лучше» and `same` means «Одинаково». A user has at most one vote per book and adaptation. The request creates the caller's vote, or replaces its value, and returns 200 with the caller's verdict item for that adaptation (see «Adaptation verdicts endpoint»).

The caller MUST have the book on their «Прочитано» shelf to create or change a vote. Otherwise the response is 403 «only readers who have read the book can vote» and nothing changes.

Errors:
- no token returns 401;
- a malformed id returns 400 «invalid book id» or «invalid screen work id»;
- a missing or other `value` returns 400 «invalid vote value»;
- an unknown or unpublished book returns 404 «book not found»;
- a work that is not linked to the book, or whose link an admin hid, returns 404 «adaptation not found».

`DELETE /api/v1/books/{id}/adaptations/{screen_work_id}/vote` SHALL require a signed-in user of any role. It removes the caller's vote for that book and adaptation and returns 204, also when there was no vote. It does not check the shelf, the link or the book status. A malformed id returns 400. A user can never change or remove another user's vote.

#### Scenario: First vote
- **WHEN** a reader with the book on «Прочитано» sends `{"value": "book"}` for a visible adaptation of the published book
- **THEN** the response is 200 with `my_vote: "book"`, and `votes_count` has grown by one

#### Scenario: Changing a vote
- **WHEN** that reader then sends `{"value": "same"}` for the same adaptation
- **THEN** the response is 200 with `my_vote: "same"`, and `votes_count` is unchanged

#### Scenario: Book not read
- **WHEN** a signed-in user who has the book on «Хочу прочитать» votes for one of its adaptations
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

### Requirement: Adaptation verdicts endpoint
`GET /api/v1/books/{id}/adaptations/verdicts` SHALL be open to everyone, with an optional token. It returns 200 with `{can_vote, min_votes, items}`:
- `can_vote` is `true` only for a signed-in caller who has the book on «Прочитано»;
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

### Requirement: Adaptation vote lifecycle
Votes SHALL belong to the link between a book and a work:
- A vote MUST be kept when the book later leaves the voter's «Прочитано» shelf, and it keeps counting.
- A vote for a link that an admin hid MUST be kept but is neither counted nor shown while the link is hidden. When the link is shown again, the votes count again.
- Deleting the link MUST remove its votes. Deleting the book or the user also removes the votes. A work can only be deleted without links (see `screen-works/admin-editing`), so it never has votes left.
- Editing a work's data and the Wikidata sync leave votes unchanged.
- A book that is no longer published keeps its votes, but its verdicts are not served.

#### Scenario: Book leaves «Прочитано»
- **WHEN** a reader who voted «book» removes the book from «Прочитано»
- **THEN** the adaptation's `votes_count` and `book_better` are unchanged

#### Scenario: Hide and show again
- **WHEN** an admin hides a link with 6 votes and later shows it again
- **THEN** while hidden the adaptation is absent from the verdicts, and afterwards its item has `votes_count: 6` again

#### Scenario: Manual link deleted
- **WHEN** an admin deletes a manual adaptation link with votes and later links the same work to the book again
- **THEN** the new link has `votes_count: 0`

### Requirement: Verdict on the book page
Each adaptation in the «Экранизации» block of the public book page SHALL show the community verdict under its links:
- From `min_votes` votes, «<P>% читателей: <leader> · <N> <голос|голоса|голосов>». The leader is the value with the most votes: «книга лучше», «экранизация лучше» or «одинаково». P is its share of all the votes, rounded to a whole number. When two values share the highest count, the line reads «Мнения разделились · <N> <голос|голоса|голосов>».
- Below `min_votes`, including no votes at all, «Пока мало голосов».

When the verdicts cannot be loaded, no verdict or voting control is shown, and the rest of the block is unaffected. The public author page («Фильмы об авторе») MUST NOT show verdicts or voting controls, because films about an author are not adaptations of a book.

#### Scenario: Verdict shown
- **WHEN** a reader opens a book whose adaptation has 18 votes «book», 3 votes «adaptation» and 2 votes «same»
- **THEN** the adaptation shows «78% читателей: книга лучше · 23 голоса»

#### Scenario: Few votes
- **WHEN** an adaptation has 3 votes
- **THEN** it shows «Пока мало голосов»

#### Scenario: Tie
- **WHEN** an adaptation has 3 votes «book», 3 votes «adaptation» and 0 votes «same»
- **THEN** it shows «Мнения разделились · 6 голосов»

#### Scenario: Verdicts unavailable
- **WHEN** the verdicts request fails
- **THEN** the «Экранизации» block lists the adaptations without verdict lines or voting controls

### Requirement: Voting on the book page
Under the verdict, each adaptation SHALL offer voting according to the visitor:
- A guest sees the link «Войдите, чтобы сравнить книгу и экранизацию», which leads to the login page and back to the book.
- A signed-in user who may vote sees «Что лучше?» with the buttons «Книга лучше», «Экранизация лучше» and «Одинаково», and the note «Голосуя, вы отмечаете, что смотрели экранизацию.». The button of the reader's current vote is shown as selected. Clicking another button changes the vote, and clicking the selected one removes it. The buttons are disabled while a request runs. A failure shows «Не удалось сохранить голос.» and keeps the previous state.
- A signed-in user who may not vote and has no vote sees «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.».
- A signed-in user who may not vote but has a vote sees «Ваш голос: <Книга лучше|Экранизация лучше|Одинаково>» and the button «Убрать голос».

After a vote changes, the verdict line updates without reloading the page. When the reader moves the book onto or off «Прочитано» on the same page, the voting controls update without a reload.

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
- **WHEN** a signed-in user without the book on «Прочитано» opens the book
- **THEN** each adaptation shows «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.»

#### Scenario: Marking the book as read on the page
- **WHEN** that user marks the book «Прочитано» with the shelf buttons of the same page
- **THEN** the voting buttons appear under each adaptation without a reload

#### Scenario: Vote kept after leaving «Прочитано»
- **WHEN** a reader who voted «Книга лучше» moves the book to «Читаю» and looks at the adaptation
- **THEN** it shows «Ваш голос: Книга лучше» and «Убрать голос», and no voting buttons
