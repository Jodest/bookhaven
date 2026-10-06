# Search and Filters

Status: completed

## Goal

Enable users to search the book catalog by text and filter results by genre, author, rating, language, and publication year. All filter state should be reflected in the URL so results are shareable and bookmarkable.

## User Stories

1. As a user, I can type a search query in a search bar to find books by title or ISBN.
2. As a user, I can filter books by one or more genres.
3. As a user, I can filter books by author (search and select).
4. As a user, I can filter books by minimum rating.
5. As a user, I can filter books by language (e.g., ru, en).
6. As a user, I can filter books by publication year range.
7. As a user, I can sort results by rating, newest, or title.
8. As a user, I can see active filters as chips/tags and remove them individually.
9. As a user, I can clear all filters at once.
10. As a user, I can see filter options populated from actual catalog data (genre count, etc.).

## Acceptance Criteria

- [ ] Search input on the `/books` page sends `search` query param to the API
- [ ] Genre filter: multi-select from available genres, sends `genre_id` params
- [ ] Author filter: search-as-you-type from `/api/v1/authors` endpoint, sends `author_id` param
- [ ] Rating filter: minimum rating selector (1-5), sends `rating_min` param
- [ ] Language filter: dropdown with available languages, sends `language` param
- [ ] Year range filter: from/to inputs, sends `year_from` and `year_to` params
- [ ] Sort selector: rating (default), newest, title ascending, sends `sort` param
- [ ] All filter values are reflected in URL searchParams (shareable/bookmarkable)
- [ ] Active filters displayed as removable chips above the results grid
- [ ] "Clear all" button removes all filters
- [ ] Pagination preserves all active filters
- [ ] Empty state shown when no books match the filters
- [ ] Filter panel is responsive: sidebar on desktop, drawer on mobile
- [ ] Backend uses existing GIN full-text search index for text queries (not ILIKE)
- [ ] Author search endpoint supports partial name matching

## Business Rules

- Search matches against book title, ISBN, and description using PostgreSQL full-text search with Russian language support (existing `idx_books_search` GIN index).
- Author search matches against author name using the existing `idx_authors_search` GIN index.
- When no filters are applied, books are sorted by rating descending, then created_at descending (existing default).
- Language filter defaults to all languages; when set, only books with that `language` field value are returned.
- Rating filter is minimum rating: books with rating >= the selected value are returned.
- Year range filter is inclusive: books with `published_year` between `year_from` and `year_to` (inclusive) are returned.
- Maximum results per page: 100 (existing limit).
- Default results per page: 20 (existing default).

## UX

- **Search bar**: Positioned at the top of the `/books` page. Uses a text input with a search icon. Debounced input (300ms) to avoid excessive API calls. Submit on Enter.
- **Filter panel**: Left sidebar on desktop (>1024px), bottom sheet/drawer on mobile. Contains all filter groups stacked vertically.
- **Genre filter**: List of checkboxes with genre names. Number of books per genre shown in parentheses (requires backend support or separate query).
- **Author filter**: Search input with dropdown results. Select an author to apply the filter. Shows selected author as a chip.
- **Rating filter**: 5-star clickable selector or radio buttons (1-5 stars + "Any").
- **Language filter**: Dropdown select with options: All, Русский, English, etc. Populate from distinct `language` values.
- **Year range filter**: Two number inputs (from/to) with reasonable constraints (e.g., 1000-2099).
- **Sort selector**: Dropdown or button group: По рейтингу (default), По дате (newest), По названию (A-Z).
- **Active filters bar**: Horizontal row of removable chips above the book grid. Shows "Сбросить все" button.
- **Book grid**: Existing card layout, unchanged.
- **Pagination**: Existing pagination, updated to preserve all filter params in links.
- **Loading state**: Skeleton placeholders while filters are loading or results are fetching.
- **Empty state**: Friendly message when no books match: "По вашему запросу ничего не найдено. Попробуйте изменить фильтры."

## API

### Modified: `GET /api/v1/books`

Query parameters (all optional):

| Parameter    | Type   | Description                              |
|-------------|--------|------------------------------------------|
| search      | string | Full-text search query                   |
| genre_id    | uuid   | Filter by genre ID (can be repeated)     |
| author_id   | uuid   | Filter by author ID                      |
| rating_min  | int    | Minimum rating (1-5)                     |
| language    | string | Filter by language code (e.g., "ru")     |
| year_from   | int    | Minimum published year                   |
| year_to     | int    | Maximum published year                   |
| sort        | string | Sort: "rating" (default), "new", "title" |
| page        | int    | Page number (default: 1)                 |
| limit       | int    | Results per page (default: 20, max: 100) |

Response: unchanged `listBooksResponse` structure.

### New: `GET /api/v1/authors`

Returns only `published` authors (list, search and `total`); see [author-status.md](author-status.md). In public book payloads a non-published author is embedded by name only with `"slug": null`.

Query parameters:

| Parameter | Type   | Description                     |
|-----------|--------|---------------------------------|
| search    | string | Partial name match              |
| limit     | int    | Max results (default: 20)       |

Response:
```json
{
  "items": [
    { "id": "...", "name": "...", "slug": "...", "photo_url": "..." }
  ],
  "total": 10
}
```

### New: `GET /api/v1/languages`

No parameters. Returns distinct language values from the books table.

Response:
```json
{
  "items": [
    { "code": "ru", "name": "Русский" },
    { "code": "en", "name": "English" }
  ]
}
```

## Database

### Migration: add index on `books.language`

```sql
CREATE INDEX idx_books_language ON books(language);
```

No other schema changes required. Existing tables, columns, and GIN indexes (`idx_books_search`, `idx_authors_search`) are sufficient.

### GORM Model Changes

Extend `BookFilter` struct in `backend/internal/modules/catalog/repository/repository.go`:

```go
type BookFilter struct {
    Search     string
    GenreID    *uuid.UUID   // change to slice for multi-genre
    AuthorID   *uuid.UUID
    RatingMin  *int
    Language   *string
    YearFrom   *int
    YearTo     *int
    Sort       string       // "rating" (default), "new", "title"
    Limit      int
    Offset     int
}
```

### Repository Changes

In `ListBooks`:
- Replace `ILIKE` with `to_tsvector`/`to_tsquery` using the existing GIN index for `search`.
- Add conditions for `RatingMin`, `Language`, `YearFrom`, `YearTo`.
- Support multiple `GenreID` values (change to `[]uuid.UUID`).
- Apply `Sort` parameter to ORDER BY clause.

New repository methods:
- `ListAuthors(ctx, search string, limit int) ([]domain.Author, int64, error)` — partial name match using `idx_authors_search`.
- `ListLanguages(ctx) ([]string, error)` — `SELECT DISTINCT language FROM books WHERE is_hidden = false`.

## Frontend

### Components to create

| Component             | Path                                          | Description                              |
|----------------------|-----------------------------------------------|------------------------------------------|
| `SearchBar`          | `src/components/search-bar.tsx`               | Text input with search icon, debounced   |
| `FilterPanel`        | `src/components/filter-panel.tsx`             | Sidebar/drawer with all filter groups    |
| `GenreFilter`        | `src/components/filters/genre-filter.tsx`     | Genre checkboxes                         |
| `AuthorFilter`       | `src/components/filters/author-filter.tsx`    | Author search + select                   |
| `RatingFilter`       | `src/components/filters/rating-filter.tsx`    | Star rating selector                     |
| `LanguageFilter`     | `src/components/filters/language-filter.tsx`  | Language dropdown                        |
| `YearFilter`         | `src/components/filters/year-filter.tsx`      | Year range inputs                        |
| `SortSelector`       | `src/components/sort-selector.tsx`            | Sort dropdown/button group               |
| `ActiveFilters`      | `src/components/active-filters.tsx`           | Removable filter chips                   |
| `EmptyState`         | `src/components/empty-state.tsx`              | "No results" message                     |

### Pages to modify

| Page              | Change                                                             |
|------------------|--------------------------------------------------------------------|
| `src/app/books/page.tsx` | Add SearchBar, FilterPanel, ActiveFilters, SortSelector. Change from server component to client component (or use URL searchParams with a client wrapper). Wire all filter state to URL searchParams. |

### State management

All filter state lives in URL searchParams (via `useSearchParams` / `useRouter`). This keeps filters shareable and browser-history navigable. No separate state store needed.

### API client changes

Add to `packages/api-client/src/index.ts`:
- `api.authors.list({ search?, limit? })` — returns author search results.
- `api.languages.list()` — returns available languages.
- Extend `api.books.list()` params with `rating_min`, `language`, `year_from`, `year_to`, `sort`.

### Key patterns

- Use `useSearchParams` from `next/navigation` for reading/writing URL state.
- Debounce search input (300ms) using a simple `useEffect` + `setTimeout` or a `useDebounce` hook.
- Use existing `@bookhaven/ui` Button component for filter actions.
- Use shadcn/ui Select, Checkbox, Input components (install if not present).

## Existing Patterns

- **Repository pattern**: `Repository` interface → `GormRepository` implementation. Filter structs passed to repository methods.
- **Service layer**: Thin pass-through from handler to repository.
- **HTTP handlers**: Echo handlers parse query params, build filter structs, call service.
- **API client**: Central `api` object with methods per resource. snake_case → camelCase normalization.
- **Pagination**: `page`/`limit` params, `PaginatedResponse<T>` type.
- **URL-driven state**: Books page already uses `searchParams` for `page` and `search`.

## Tests

### Backend

- Unit test for `ListBooks` with each new filter parameter.
- Unit test for `ListBooks` with combined filters.
- Unit test for `ListBooks` sort parameter.
- Unit test for `ListAuthors` partial name matching.
- Unit test for `ListLanguages` distinct values.
- Integration test: full HTTP request → response for `GET /api/v1/books` with all filter params.

### Frontend

- Component test: `SearchBar` calls onChange after debounce.
- Component test: `FilterPanel` renders all filter groups.
- Component test: `ActiveFilters` shows chips for active filters and removes them.
- Integration test: navigating to `/books?search=term&genre_id=...` renders correct filter state and results.
- E2E test: apply filters → URL updates → results update → clear filters → results reset.

## Documentation

- Update `docs/ROADMAP.md`: mark "Search and Filters" as in-progress, then completed.
- Update `docs/features/search-and-filters.md`: set status to `in-progress` during development, `completed` when done.

## Risks

- **Performance**: Full-text search with multiple JOIN filters may be slow without proper indexing. The existing GIN index covers text search; the new `idx_books_language` index covers language filtering. Rating and year filters scan the books table directly (indexed by rating).
- **Multi-genre filter complexity**: Supporting "books that match ALL selected genres" vs "books that match ANY selected genre" requires different JOIN strategies. Recommendation: start with ANY (OR) semantics — a book appears if it belongs to at least one selected genre.
- **Author search without full-text**: If `idx_authors_search` uses `to_tsvector` on name+biography, partial prefix matching (e.g., "Толст" → "Толстой") works with `plainto_tsquery`. Verify this matches user expectations.
- **Mobile UX**: Filter drawer interaction on mobile needs careful design to avoid blocking content. Use a slide-in panel from the right, not a modal.
- **Language list staleness**: `GET /api/v1/languages` returns current distinct values. If new books are added, the list updates automatically. No caching concern for v1.
