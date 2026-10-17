## MODIFIED Requirements

### Requirement: Public visibility of books
A book SHALL be publicly visible only when its status is `published`. A `pending` or `rejected` book MUST return 404 on its public page. It MUST be excluded from public lists, search, the home page, similar books, the list of languages and the contents of shelves. It MUST NOT be possible to add it to a shelf, rate it or review it.

#### Scenario: Pending book page
- **WHEN** a visitor opens the public page of a pending book
- **THEN** the response is 404

#### Scenario: Rejected book on a shelf
- **WHEN** a book on a user's shelf is rejected
- **THEN** the shelf no longer shows it
