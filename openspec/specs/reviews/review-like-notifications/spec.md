# reviews/review-like-notifications Specification

## Purpose

Tells a reader in the web app when someone likes their review, through a notification bell with an unread count.

## Requirements

### Requirement: Notification on a like
When a user adds a like to someone else's review, the system SHALL create a `review_like` notification for the review's author in the same transaction as the like. Liking one's own review MUST NOT notify. There is at most one notification per liker, review and type: liking again after unliking MUST reuse it, marking it unread with a new time, instead of adding a second one. Removing a like does not remove its notification. Deleting the review deletes its notifications.

#### Scenario: Like by another reader
- **WHEN** reader B likes reader A's review
- **THEN** A has a new unread notification naming B and the book

#### Scenario: Like, unlike, like again
- **WHEN** B likes, unlikes and likes A's review again
- **THEN** A has a single notification about it, unread, with the latest time

### Requirement: Notification endpoints
All notification endpoints SHALL require authentication and act only on the caller's own notifications:
- `GET /api/v1/notifications` returns `{items, total, page, limit}`, newest first (limit default 20, max 100); each item is `{id, type, actor: {id, name, avatar_url}, review: {id, book_id, book_slug, book_title}, is_read, created_at}`;
- `GET /api/v1/notifications/unread-count` returns `{count}`;
- `PATCH /api/v1/notifications/{id}/read` marks one read; a notification that does not exist or belongs to another user MUST return the same 404;
- `POST /api/v1/notifications/read-all` marks all of the caller's notifications read.

#### Scenario: Someone else's notification
- **WHEN** a user tries to mark another user's notification as read
- **THEN** the response is 404, exactly as for a notification that does not exist

### Requirement: Notification bell
The web header SHALL show signed-in readers a bell («Уведомления») with the unread count (shown as «9+» above 9), refreshed every 30 seconds. The list shows «{name}: понравилась ваша рецензия на «{book}»» («Пользователь» when the liker has no name). Opening a notification marks it read and goes to the book page; «Отметить все прочитанными» marks all read.

#### Scenario: Opening a notification
- **WHEN** a reader clicks an unread like notification
- **THEN** it is marked read and the book page opens
