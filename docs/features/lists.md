# Lists: viewing

Status: superseded by openspec/changes/merge-lists-into-shelves (lists were merged into custom shelves; the text below is frozen history)

## Goal

Allow users and guests to view lists safely, with pagination and enough book metadata to open each book.

## User Stories

- As an authenticated user, I can view my own lists and open a list with its books.
- As a guest or authenticated user, I can open a public list from its shareable URL.
- As a non-owner, I cannot view a private list or learn its contents.

## Acceptance Criteria

- [x] `GET /api/v1/lists` returns the authenticated user's lists and book counts; guests receive `401`.
- [x] An owner can view a list and its books through the paginated private endpoint.
- [x] A guest or authenticated user can view a public list through the public endpoint without authentication.
- [x] Private-list access by a non-owner is denied without exposing list data; nonexistent and inaccessible public-list requests use the documented not-found/denial response consistently.
- [x] List views show title, optional description, visibility (where relevant), book count, and ordered books with title, cover, authors, genres, and links to book details.
- [x] Empty lists, loading, invalid IDs, unavailable lists, and pagination boundaries have clear Russian UI states.
- [x] Viewing exposes no owner-only controls or mutation operations.

## Business Rules

- A list is visible publicly only when `is_public = true`.
- The private list endpoint is owner-only; authentication does not grant access to another user's private list.
- Public viewing is read-only and preserves the list's explicit `position` order; ties are broken by `added_at` and then `book_id` to keep ordering deterministic for rows that were defaulted to `position = 0` (see migration `000006`).
- Pagination defaults to 20 items and is capped at 100; clients may request `page` and `limit`.

## UX

- `/lists` is an authenticated user's list index with cards and book counts.
- `/lists/[id]` is the owner's paginated detail view.
- `/lists/public/[id]` is a read-only share view available to guests.
- Book cards link to `/books/[slug]`; private views may show visibility, while public views show a public-list indicator.
- All user-facing text is Russian. Do not render edit, delete, add, remove, or reorder controls in public/guest views.
- On the owner detail page, a failure during the initial load shows a full-page error, while mutation failures keep the loaded list and controls visible and show an inline Russian error.

## API

No new API is required. Use the existing Echo + GORM lists module:

- `GET /api/v1/lists` — required auth; current user's list summaries.
- `GET /api/v1/lists/:id?page=1&limit=20` — required auth; owner-only list detail and ordered book page.
- `GET /api/v1/lists/public/:id?page=1&limit=20` — optional auth; public list detail and ordered book page.

Responses contain list metadata plus `items`, `total`, `page`, and `limit`; book items include author and genre metadata. Invalid IDs return `400`; missing or inaccessible resources must follow the existing handler contract (`404`/`403` as applicable).

The public detail endpoint returns a dedicated public list DTO that includes required metadata (`id`, `title`, `description`, `is_public`, `books_count`, `created_at`, `updated_at`) but never exposes owner-only fields such as `user_id`.

## Database

No schema changes. Read list metadata from `lists` and ordered books from `list_items.position`; preserve the existing composite uniqueness and ownership model. Book queries order by `position ASC, added_at ASC, book_id ASC` so that rows with the same position—most notably existing rows defaulted to `position = 0` by migration `000006`—are returned deterministically by insertion time and, in the unlikely event of identical `added_at`, by book identifier.

## Frontend

- Use the existing `api.lists.list`, `api.lists.get`, and `api.lists.getPublic` client methods.
- Keep list index and owner detail authenticated; public detail must work when no session exists.
- Render responsive book grids, pagination, empty state, loading state, and an error state without leaking private data.

## Existing Patterns

- Backend layering and ownership checks follow `backend/internal/modules/lists/{service,transport/http}` and the shelves module pattern.
- The existing public page at `apps/web/src/app/lists/public/[id]/page.tsx` establishes client fetching, Russian states, book links, and pagination.
- Existing list response shapes already provide the required metadata; avoid changing mutation endpoints.

## Tests

- Backend service/handler tests: authenticated list index, owner detail, public detail without auth, private-list denial, missing/invalid IDs, pagination, ordering, and response metadata.
- Frontend checks: guest public-page load, owner/non-owner control visibility, empty/loading/error states, book links, and first/last-page navigation.

## Documentation

- Keep this contract as the source of truth for list viewing.
- Reconcile `ROADMAP.md` and the parent Lists status after implementation; it currently marks Lists completed although this viewing scope still needs explicit verification.

## Risks

- The existing documentation and implementation differ on whether a private public-route request returns `403` or `404`; settle and test one privacy-preserving contract before implementation.
- Public endpoints must not expose owner-only fields or mutation actions, and must avoid IDOR/data leakage.
