## ADDED Requirements

### Requirement: Book first-publication time
Every book SHALL carry a first-publication time, which is empty until the book is `published` for the first time. The time MUST be set to the current time on the first transition into `published`, whatever the path: manual creation with status `published` (the default), book file import with batch status `published`, a single status change, or a bulk status change. Once set, it MUST NOT change: unpublishing to `pending` or `rejected` keeps it, and republishing keeps the original time. Editing other book fields MUST NOT change it. A book created `pending` (Wikidata harvest, manual creation or file import with status `pending`) MUST have no first-publication time until it is published. Admin book reads MUST include it as a read-only, nullable `published_at` field; a `published_at` sent in a create or update body MUST be ignored. Public book responses MUST NOT include it.

#### Scenario: Harvested book published from the queue
- **WHEN** an admin publishes a pending harvested book that was created a month ago
- **THEN** its `published_at` is the time of publishing, not its creation time

#### Scenario: Book created published
- **WHEN** an admin creates a book without a `status`
- **THEN** the book is `published` and its `published_at` is set to the creation time

#### Scenario: Pending book has no publication time
- **WHEN** an admin reads a pending book that was never published
- **THEN** its `published_at` is `null`

#### Scenario: Republishing keeps the original time
- **WHEN** an admin unpublishes a book and later publishes it again
- **THEN** its `published_at` still equals the time of its first publication

#### Scenario: Client-sent publication time is ignored
- **WHEN** an admin updates a book with `"published_at": "2000-01-01T00:00:00Z"` in the body
- **THEN** the stored `published_at` is unchanged

### Requirement: New in catalog ordering
The home page row «Новое в каталоге» SHALL list published books ordered by first-publication time, newest first, with ties broken by book id. A book that becomes visible for the first time MUST be able to appear in the row regardless of when it was created. The public book list sorts are not affected.

#### Scenario: Old harvest published today
- **WHEN** a book harvested a month ago is published for the first time today, and no book was published later
- **THEN** it is the first book of «Новое в каталоге» once the home page cache refreshes

#### Scenario: Toggling does not bump a book
- **WHEN** an admin unpublishes and republishes a book first published a year ago
- **THEN** its position in «Новое в каталоге» is the same as before the toggle
