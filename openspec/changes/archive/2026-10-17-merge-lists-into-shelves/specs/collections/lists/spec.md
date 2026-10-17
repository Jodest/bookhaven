## REMOVED Requirements

### Requirement: Create, edit and delete a list
**Reason**: Custom shelves now cover user lists, including the title, description and privacy flag (see `collections/shelves`).
**Migration**: Every list was turned into a custom shelf of the same owner. Use `POST /api/v1/shelves` and `PATCH /api/v1/shelves/{id}` with `{name, description?, is_public?}` instead of `/api/v1/lists`.

### Requirement: Listing my lists
**Reason**: Lists no longer exist; former lists are listed among the reader's custom shelves.
**Migration**: Use `GET /api/v1/shelves`. With `?book_id=…`, `has_book` on each shelf replaces `book_in_lists`.

### Requirement: Owner-only access
**Reason**: The `/api/v1/lists` endpoints are removed.
**Migration**: The same owner-only rule applies to `/api/v1/shelves` (see `collections/shelves`).

### Requirement: Lists pages
**Reason**: `/lists` and `/lists/{id}` are removed together with user lists.
**Migration**: Former lists are managed on `/shelves` and `/shelves/{id}`, where a custom shelf's description and privacy can be edited. Old `/lists…` addresses are not redirected.
