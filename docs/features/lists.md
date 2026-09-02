# Lists

Status: completed

## Goal

Allow users to create named, shareable book collections (reading lists, wishlists, favorites, etc.) that are separate from the system shelves.

## User Stories

- As a user I can create a list with a title and optional description
- As a user I can add/remove books to/from my lists
- As a user I can reorder books within a list via a position field
- As a user I can make a list public so others can view it
- As a user I can share a public list via URL
- As a user I can view any public list without authentication
- As a user I can edit list title, description, and visibility
- As a user I can delete a list

## Acceptance Criteria

- [ ] Users can create, rename, update description, toggle visibility, and delete lists
- [ ] Books can be added to and removed from lists
- [ ] Books within a list maintain explicit ordering via a `position` column
- [ ] Public lists are accessible at `/lists/public/:id` without authentication
- [ ] Private lists return 403 to non-owners
- [ ] Book detail page shows an "Add to list" dropdown
- [ ] Lists page shows all user's lists with book counts
- [ ] List detail page shows books with author/genre info and remove button

## Business Rules

- List title: 2–255 characters, trimmed
- List description: optional, max 2000 characters
- `is_public` defaults to `true`
- A list belongs to exactly one user
- Books within a list are unique (composite PK on `list_id, book_id`)
- Book order within a list is controlled by the `position` integer column
- New books added to a list get `position = max(existing) + 1`
- Removing a book does NOT reindex remaining positions
- Deleting a list cascades to list_items
- No limit on the number of lists per user (soft cap: warn at 50)
- No limit on the number of books per list

## UX

- `/lists` — user's lists page: grid of list cards (title, description preview, book count, visibility badge)
- `/lists/[id]` — list detail: paginated book grid, edit/delete controls, share button
- `/lists/public/[id]` — public list view: same as detail but without edit controls
- Book detail page: "Add to list" section below shelf buttons, shows user's lists as checkboxes
- All text in Russian

## API

Follow existing Echo + GORM pattern (see shelves module).

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/lists` | required | List current user's lists |
| `POST` | `/api/v1/lists` | required | Create a new list |
| `GET` | `/api/v1/lists/:id` | required | Get list with paginated books (owner only) |
| `PATCH` | `/api/v1/lists/:id` | required | Update list (title, description, is_public) |
| `DELETE` | `/api/v1/lists/:id` | required | Delete a list |
| `POST` | `/api/v1/lists/:id/books` | required | Add a book to a list |
| `DELETE` | `/api/v1/lists/:id/books/:bookId` | required | Remove a book from a list |
| `PATCH` | `/api/v1/lists/:id/books/reorder` | required | Update book positions in a list |
| `GET` | `/api/v1/lists/public/:id` | optional | Get a public list (anyone) |

### Request/Response

**POST /api/v1/lists**
```json
{ "title": "string", "description": "string?", "is_public": true }
```
Response: 201 with list object

**GET /api/v1/lists**
```json
{ "items": [...], "total": 0 }
```

**GET /api/v1/lists/:id?page=1&limit=20**
```json
{
  "list": { "id", "user_id", "title", "description", "is_public", "books_count", "created_at", "updated_at" },
  "items": [Book],
  "total": 0,
  "page": 1,
  "limit": 20
}
```

**POST /api/v1/lists/:id/books**
```json
{ "book_id": "uuid" }
```
Response: 201

**DELETE /api/v1/lists/:id/books/:bookId**
Response: 200

**PATCH /api/v1/lists/:id/books/reorder**
```json
{ "book_ids": ["uuid1", "uuid2", ...] }
```
Positions derived from array index. Response: 200

**GET /api/v1/lists/public/:id?page=1&limit=20**
Same structure as GET /api/v1/lists/:id but returns 403 if list is private.

## Database

### Migration 000006_lists_position.sql

```sql
ALTER TABLE list_items ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_list_items_book_id ON list_items(book_id);
```

### Existing tables (no changes)

`lists` table from `000001_initial.up.sql` — no schema changes needed.

### Domain model

```go
type List struct {
    ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID      uuid.UUID `gorm:"type:uuid;not null"`
    Title       string    `gorm:"not null;size:255"`
    Description string    `gorm:"type:text"`
    IsPublic    bool      `gorm:"not null;default:true"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    BooksCount  int64     `gorm:"-"`
}

type ListItem struct {
    ListID   uuid.UUID `gorm:"type:uuid;primaryKey"`
    BookID   uuid.UUID `gorm:"type:uuid;primaryKey"`
    Position int       `gorm:"not null;default:0"`
    AddedAt  time.Time `gorm:"not null;default:now()"`
}
```

## Frontend

### Pages

1. **`/lists/page.tsx`** — My lists (client component)
   - Fetches user's lists
   - Grid of ListCard components
   - "Create list" form (title, description, visibility toggle)

2. **`/lists/[id]/page.tsx`** — List detail (client component)
   - Fetches list with paginated books
   - Edit controls (title, description, visibility)
   - Share button (copy link)
   - Book grid with remove buttons
   - Pagination

3. **`/lists/public/[id]/page.tsx`** — Public list view (client component)
   - Same as detail but no edit controls
   - Works without auth

### Components

1. **`/components/add-to-list-button.tsx`** — Dropdown on book detail pages
   - Fetches user's lists
   - Checkboxes to add/remove book from each list
   - "Create new list" inline option

### API Client

Add to `packages/api-client/src/index.ts`:

```typescript
api.lists = {
  list: () => fetcher<{ items: BookList[]; total: number }>('/api/v1/lists'),
  create: (data: { title: string; description?: string; is_public?: boolean }) =>
    fetcher<BookList>('/api/v1/lists', { method: 'POST', body: JSON.stringify(data) }),
  get: (id: string, params?: { page?: number; limit?: number }) => { ... },
  update: (id: string, data: { title?: string; description?: string; is_public?: boolean }) =>
    fetcher<BookList>(`/api/v1/lists/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
  remove: (id: string) =>
    fetcher<{ status: string }>(`/api/v1/lists/${id}`, { method: 'DELETE' }),
  addBook: (id: string, bookId: string) =>
    fetcher<{ status: string }>(`/api/v1/lists/${id}/books`, { method: 'POST', body: JSON.stringify({ book_id: bookId }) }),
  removeBook: (id: string, bookId: string) =>
    fetcher<{ status: string }>(`/api/v1/lists/${id}/books/${bookId}`, { method: 'DELETE' }),
  reorder: (id: string, bookIds: string[]) =>
    fetcher<{ status: string }>(`/api/v1/lists/${id}/books/reorder`, { method: 'PATCH', body: JSON.stringify({ book_ids: bookIds }) }),
  getPublic: (id: string, params?: { page?: number; limit?: number }) => { ... },
};
```

## Existing Patterns

- **Module structure**: `domain/ → repository/ → service/ → transport/http/` (follow shelves pattern)
- **Repository**: `GormRepository` implementing a `Repository` interface
- **Service**: Business logic with domain errors (`ErrListNotFound`, `ErrForbidden`, `ErrBookAlreadyOnList`, etc.)
- **Handler**: Echo handler with request/response structs, validation via `validate` tags
- **Tests**: Service tests with a `fakeRepository` (see shelves service_test.go)
- **Frontend**: `'use client'` pages using `api.*` from `@bookhaven/api-client`, `useForm` from react-hook-form + zod, `@bookhaven/ui` Button
- **Routes**: Registered in `router.go`, service passed via `RegisterRoutes` params
- **DI**: Wiring in `cmd/api/main.go`

## Tests

### Backend

- **Service tests** (`lists/service/service_test.go`):
  - Create list (valid, invalid name)
  - Update list (title, description, visibility)
  - Delete list (owner, non-owner forbidden)
  - Add book (valid, duplicate, book not found)
  - Remove book (valid, not in list)
  - Reorder books (valid, empty list, invalid IDs)
  - Get public list (public, private non-owner forbidden)

- **No repository integration tests** (follow existing pattern — no integration tests in shelves)

### Frontend

- **No automated frontend tests** (follow existing pattern — no tests in shelves frontend)

## Documentation

- Update `ROADMAP.md`: move Lists from "Next" to "Completed"
- No new documentation files needed beyond this feature doc

## Risks

- **Position management**: Reordering requires batch UPDATE queries. Keep it simple — accept full ordered list of book IDs from frontend, derive positions from array index.
- **Concurrent reorders**: Low risk for typical usage. Accept eventual consistency.
- **Public list discovery**: No browse/search for public lists in this iteration (can be added later).
