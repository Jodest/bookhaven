## REMOVED Requirements

### Requirement: Add and remove a book
**Reason**: User lists are merged into custom shelves.
**Migration**: Use `POST /api/v1/shelves/{id}/books` and `DELETE /api/v1/shelves/{id}/books/{bookId}` (see `collections/shelf-books`). Books of former lists were moved to the new shelves with their time of adding.

### Requirement: Reorder
**Reason**: Shelves have no manual order; the agreed model sorts books by the time they were added.
**Migration**: None. List positions are dropped and the reorder endpoint is removed without a replacement.

### Requirement: Books of a list
**Reason**: User lists are merged into custom shelves.
**Migration**: Use `GET /api/v1/shelves/{id}`, which lists the shelf's published books most recently added first.

### Requirement: Adding from the book page
**Reason**: The book page lists control is removed together with user lists.
**Migration**: Readers put a book on a custom shelf with «На полку» on the book page (see `collections/shelf-books`).
