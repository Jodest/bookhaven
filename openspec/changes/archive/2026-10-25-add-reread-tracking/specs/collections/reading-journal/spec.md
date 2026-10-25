## Purpose

Keeps a per-reader history of reading attempts for every book, so that re-reading, abandoning or undoing «Прочитано» never erases earlier reads, and exposes the read count derived from it.

## ADDED Requirements

### Requirement: Reading journal
The system SHALL keep, for every user and book, a journal of reading attempts that is separate from the shelves. The default shelf holding the book is its current status, and the journal is its history. Each attempt is in one of three states:
- open: being read;
- finished: it has a finish time;
- abandoned: it has an abandon time.

An attempt MAY have a start time. An attempt MUST NOT be both finished and abandoned, and a user MUST have at most one open attempt per book. Only the status transitions in «Status transitions update the journal» change the journal. No endpoint creates, edits or deletes attempts directly. Deleting a user or a book deletes its attempts. Unpublishing a book leaves them unchanged.

#### Scenario: One open attempt at most
- **WHEN** a reader whose book is on «Читаю» sends the request to add it to «Читаю» again
- **THEN** the response is 409 and the reader still has exactly one open attempt for the book

#### Scenario: Account deleted
- **WHEN** a user with finished attempts deletes their account
- **THEN** their attempts are gone and no longer count anywhere

### Requirement: Status transitions update the journal
Adding a book to a default shelf or removing it from one (`POST /api/v1/shelves/{id}/books`, `DELETE /api/v1/shelves/{id}/books/{bookId}`) SHALL update the caller's journal for that book in the same transaction as the shelf change. The journal MUST change only when the shelf change itself succeeds: a request answered 404 or 409 leaves the journal untouched, so repeating a request (a double click) never records a second attempt. Moving a book to a status changes the journal as follows:
- **To «Читаю».** An attempt starting now is opened, unless one is already open. Earlier attempts are kept.
- **To «Прочитано».** The open attempt is finished now. Without an open attempt, a finished attempt without a start time is recorded.
- **To «Брошено».** The open attempt is closed as abandoned now. Without an open attempt, nothing changes.
- **To «Хочу прочитать».** The open attempt, if there is one, is deleted. Finished and abandoned attempts are kept.

Removing a book from a status changes the journal as follows:
- **From «Читаю».** The open attempt is deleted.
- **From «Прочитано».** Only the most recently finished attempt is deleted, as an undo. Earlier attempts are kept, and the book is then on no default shelf.
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
- **WHEN** a reader with two finished attempts of a book removes it from «Прочитано»
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

### Requirement: Reading summary for a book
`GET /api/v1/shelves?book_id=<id>` SHALL also return `reads`, the caller's reading summary for that book: `{reads_count, last_read_at, rereading}`.
- `reads_count` is the number of finished attempts.
- `last_read_at` is the finish time of the most recent one, or `null` when there is none.
- `rereading` is `true` when the book has an open attempt and at least one finished attempt.

The summary always describes the caller's own journal. The endpoint keeps requiring authentication (401 without a token). Without `book_id`, or with a malformed one (which is ignored as before), `reads` is absent.

#### Scenario: Book read twice
- **WHEN** a reader who finished a book in March 2024 and in May 2025 lists their shelves with `?book_id` of that book
- **THEN** `reads` is `{reads_count: 2, last_read_at: <May 2025 finish time>, rereading: false}`

#### Scenario: Re-reading
- **WHEN** a reader with one finished attempt has the book on «Читаю» and lists their shelves with its `book_id`
- **THEN** `reads.reads_count` is 1 and `reads.rereading` is `true`

#### Scenario: Never read
- **WHEN** a reader lists their shelves with `?book_id` of a book they never finished
- **THEN** `reads` is `{reads_count: 0, last_read_at: null, rereading: false}`

#### Scenario: Without book_id
- **WHEN** a reader lists their shelves without `book_id`
- **THEN** the response has no `reads`

### Requirement: Journal seeded from existing shelves
When the journal is introduced, existing shelves SHALL seed it once:
- every book on a «Прочитано» shelf gets a finished attempt, finished when it was added to that shelf;
- every book on «Читаю» gets an open attempt, started when it was added;
- every book on «Брошено» gets an abandoned attempt at the time it was added;
- «Хочу прочитать» and custom shelves seed nothing.

Right after the rollout, every reader's challenge counts and adaptation voting rights MUST be the same as before it.

#### Scenario: Book on «Прочитано» before the rollout
- **WHEN** a book was added to a reader's «Прочитано» on 10 June 2025 before the rollout
- **THEN** after the rollout the reader has one finished attempt for it finished on 10 June 2025, and it still counts for 2025

#### Scenario: Book on «Читаю» before the rollout
- **WHEN** a book was on a reader's «Читаю» before the rollout
- **THEN** after the rollout it has one open attempt and `reads_count` 0
