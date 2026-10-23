## Why

`books_count` on `GET /api/v1/shelves` counts every book on a shelf, including pending and rejected ones, while the shelf pages and the public profile show published books only. The owner sees «5 книг» on `/shelves` and four books inside (proposal #384).

## What Changes

- `GET /api/v1/shelves` counts published books only, like the public profile already does.
- The public profile listing reuses the same query instead of its own copy.

## Impact

- Specs: `collections/shelves` (Listing my shelves).
- Code: `backend/internal/modules/shelves/repository` only. No API shape change, no migration, no frontend change.
- Issue: #613 (supersedes proposal #384).
