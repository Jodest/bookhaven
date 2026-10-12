# catalog-moderation/bulk-status Specification

## Purpose

Lets an admin change the status of many books, authors or genres in one request, typically the selection of one moderation queue page, with a per-entity report.

## Requirements

### Requirement: Bulk status endpoints
The system SHALL accept `POST /api/v1/admin/books/bulk-status`, `POST /api/v1/admin/authors/bulk-status` and `POST /api/v1/admin/genres/bulk-status` with the body `{ids: string[], status}`, where `status` is `pending`, `published` or `rejected`. These endpoints MUST require the `admin` role and a bearer token; a moderator MUST get 403.

#### Scenario: Moderator is refused
- **WHEN** a moderator calls `POST /api/v1/admin/genres/bulk-status`
- **THEN** the response is 403 and no status changes

### Requirement: Request validation
The whole request MUST be rejected with 400, changing nothing, when the status is invalid, `ids` is empty, any id is not a UUID, or there are more than 100 distinct ids. Duplicate ids SHALL be collapsed before the limit is checked and before processing.

#### Scenario: Too many ids
- **WHEN** an admin sends 101 distinct author ids
- **THEN** the response is 400 and no author changes

#### Scenario: Duplicate ids
- **WHEN** an admin sends the same book id twice
- **THEN** the book is processed once and `processed` is 1

### Requirement: Independent processing with a report
Each id SHALL be processed independently: an unknown id (and, for genres, a merged genre) MUST be a row error with the message «book not found», «author not found» or «genre not found», and the other ids MUST still be applied. The response MUST be 200 with `{processed, updated, errors}`, where each error carries the entity id (`book_id`, `author_id` or `genre_id`), a `name` (books: `title`) field that may be empty, and a `message`, even when every id failed.

#### Scenario: One unknown id
- **WHEN** an admin publishes three authors and one id does not exist
- **THEN** the response is 200 with `processed: 3`, `updated: 2` and one error «author not found»

### Requirement: Only the status changes
A bulk change SHALL write only the status (and the update time where the entity has one), plus, for a book published for the first time, its first-publication time (see `catalog-moderation/entity-status`). A book that already has a first-publication time MUST keep it, whatever the new status. Names, slugs, Wikidata QIDs, other fields and the links between books, authors and genres MUST stay unchanged, and no content re-validation runs.

#### Scenario: Rejecting a genre keeps its links
- **WHEN** an admin rejects a genre linked to five books
- **THEN** the five links remain and the genre is only hidden from public book pages

#### Scenario: Bulk publish sets the first-publication time
- **WHEN** an admin bulk-publishes two pending books, one never published and one published before and later unpublished
- **THEN** the first book gets the current time as its first-publication time and the second keeps its earlier time

### Requirement: Bulk report in the admin
After a bulk change the admin SHALL show «Обновлено {updated} из {processed}.» and, when there are errors, «Ошибок: {N} (…)» with up to three errors, each naming the entity by its name or title (falling back to the id) and its message.

#### Scenario: Partial success message
- **WHEN** a bulk publish of 3 books updates 2
- **THEN** the message starts with «Обновлено 2 из 3.» and lists the failed book
