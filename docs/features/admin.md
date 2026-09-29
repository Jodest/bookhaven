# Admin

Status: implemented

## Goal

Provide a small, role-protected administration panel for maintaining the catalog and moderating reviews without exposing administrative operations through the public catalog API.

## User Stories

- As an administrator, I can see catalog and moderation counts and navigate to each work queue.
- As an administrator, I can create and edit books, authors, and genres and hide books from the public catalog.
- As an administrator or moderator, I can review hidden/visible reviews and hide or restore a review.
- As a non-privileged user, I cannot access the admin UI or admin API operations.

## Acceptance Criteria

- [x] The admin app has a Russian-language shell with navigation and routes `/`, `/books`, `/books/new`, `/books/:id/edit`, `/authors`, `/genres`, and `/reviews`.
- [x] Unauthenticated users are rejected/redirected, and authenticated users without the required role receive a clear forbidden state; the same checks are enforced server-side.
- [x] The dashboard displays counts for books, hidden books, authors, genres, and reviews awaiting moderation.
- [x] Admins can search and paginate all books (including hidden books), create a book, edit all supported book fields, assign existing authors/genres, and set visibility.
- [x] Admins can create and edit authors and genres; identifiers are generated from validated names and remain unique.
- [x] Admins and moderators can filter reviews by visibility, inspect the review content/user/rating context, and hide or restore a review.
- [x] Success, validation, not-found, conflict, loading, and server-error states are visible and destructive/visibility changes require confirmation.
- [x] Public catalog endpoints continue to exclude hidden books and expose only reviews allowed by their existing visibility behavior.

## Business Rules

- `RoleAdmin` is required for dashboard and catalog/author/genre management. `RoleAdmin` or `RoleModerator` may moderate reviews.
- Admin operations must use the authenticated user from the existing access-token middleware; UI-only role checks are insufficient.
- Book title, author name, and genre name are required and trimmed. Slugs must be URL-safe and unique; collisions return a conflict rather than silently replacing data.
- Genre names are normalized on creation: the first letter is lowercased (Unicode-aware; only the first rune changes, e.g. "Роман" -> "роман"), so genres autocreated from harvest/import sources display consistently regardless of the source's original casing. This applies only to creation (manual admin creation, bulk import, and harvest autocreate all funnel through the same `CreateGenre` service method); existing genre names are not retroactively changed, and updating a genre's name does not re-apply this normalization.
- A book may be hidden, but MVP does not hard-delete books, authors, genres, users, or reviews because of existing foreign-key and user-content relationships.
- ISBN, if supplied, must be a valid ISBN-10 or ISBN-13 format after normalization. Published year and page count, if supplied, must be positive and within sensible bounds; language must be a supported non-empty code.
- Relationships may reference only existing authors and genres. Duplicate relationship IDs are rejected or normalized before persistence.
- Moderation changes are idempotent and must not alter review text, rating, ownership, or like counts.

## UX

- Use the existing admin Next.js app and shared UI components, with a persistent sidebar/header and responsive tables/forms.
- Tables show pagination, search/filter controls, visibility badges, and row actions. Forms use explicit labels, inline validation, and preserve entered values after validation errors.
- Hide/restore actions require confirmation and show a toast/banner on completion. A successful save returns to the relevant list and refreshes it.
- Expired/invalid authentication shows an actionable sign-in state; 403 shows an access-denied state rather than an empty table.

## API

Add an admin route group under `/api/v1/admin` using the existing `Auth` middleware followed by `RequireRole`:

- `GET /dashboard` — counts for books, hidden books, authors, genres, and reviews needing moderation.
- `GET /books?search=&visibility=&page=&limit=` — paginated admin list, including hidden records; `POST /books` creates.
- `GET /books/:id` — admin detail; `PATCH /books/:id` updates fields, relationships, and visibility.
- `GET /authors?search=&page=&limit=` and `POST /authors`; `PATCH /authors/:id` updates an author.
- `GET /genres?search=&page=&limit=` and `POST /genres`; `PATCH /genres/:id` updates a genre.
- `GET /reviews?visibility=&page=&limit=` — paginated moderation queue; `PATCH /reviews/:id` updates only `is_hidden`.

Use the repository's existing JSON response style (`items`, `total`, `page`, `limit`) and consistent JSON errors. Return 401 for missing/invalid tokens, 403 for insufficient roles, 404 for missing records, 409 for unique conflicts, 422 for validation errors, and 500 for unexpected failures. Extend the shared API client/types rather than duplicating request models in the app.

## Database

No new tables are required. Reuse `books.is_hidden`, `reviews.is_hidden`, existing catalog relationship tables, and user roles. Add a new forward-only migration only if implementation needs a missing constraint/index; do not modify existing migrations. Create/update operations must be transactional when changing a book and its author/genre relationships.

## Frontend

- Replace the current placeholder page with the dashboard and admin shell.
- Add typed admin API calls and query/mutation state using the existing frontend data-fetching conventions.
- Add list, detail/edit, and create views for books; author and genre management views; and the review moderation queue.
- Attach the existing bearer/cookie authentication mechanism to requests and handle 401/403 globally for protected routes. Never treat a client-provided role as authorization.

## Existing Patterns

- Reuse `middleware.Auth` and `middleware.RequireRole` in `backend/internal/transport/http/middleware/auth.go`.
- Follow catalog pagination, filtering, response mapping, service, repository, and Echo handler conventions in `backend/internal/modules/catalog`.
- Use the existing `User.Role` values (`user`, `contributor`, `moderator`, `admin`) and existing UUID/domain models; do not introduce a second permission system.
- Build the UI in the existing `apps/admin` Next.js app and use shared `@bookhaven/ui` components.

## Tests

- Backend handler/service/repository tests for role enforcement, pagination and visibility filtering, valid create/update flows, relationship replacement, validation, conflicts, not-found cases, idempotent moderation, and transaction rollback.
- Integration tests proving public catalog queries never return hidden books and non-admin users cannot invoke admin endpoints.
- Admin frontend tests for route access states, form validation, list filters/pagination, confirmation flows, and 401/403/error rendering.
- Run focused backend tests and admin type-check/lint; do not require the full repository suite for this feature.

## Documentation

- Keep this document as the source of truth while implementing the MVP.
- After implementation and review, mark this feature complete in `docs/ROADMAP.md` and link to this contract; document any new admin API conventions in the relevant API documentation if such documentation exists.

## Implementation Notes

- Backend admin module lives in `backend/internal/modules/admin` with repository/service/handler layers following existing catalog patterns.
- Admin routes are registered under `/api/v1/admin` with `middleware.Auth` followed by `middleware.RequireRole`.
- `RoleAdmin` is required for dashboard, books, authors, and genres; `RoleAdmin` or `RoleModerator` may moderate reviews.
- Book create/update uses transactions to replace `book_authors`/`book_genres` relationships.
- Slug generation and ISBN-10/13 validation are implemented in the service layer; slug collisions return HTTP 409.
- A new migration (`000007_admin_visibility_indexes`) adds partial-query support indexes on `books.is_hidden` and `reviews.is_hidden`.
- Admin Next.js app uses a Russian shell with persistent sidebar and routes for dashboard, books, authors, genres, and reviews.
- Shared `@bookhaven/api-client` and `@bookhaven/types` include admin request/response models.
- `@bookhaven/ui` was extended with `Input`, `Textarea`, `Label`, `Select`, `Badge`, `Card`, and `Table` components.
- State-changing admin API endpoints require an `Authorization: Bearer` header in addition to the role check; cookie-only authenticated requests are rejected to mitigate CSRF. The admin app stores the access token returned at login in `localStorage` and attaches it to `/api/v1/admin` mutations, while reads continue to use the existing HttpOnly cookie.
- The admin app uses a centralized React Query `QueryCache`/`MutationCache` error handler for `/api/v1/admin` requests: 401 clears the auth state and redirects to `/login` with a safe `redirect` parameter; 403 sets a global forbidden path and the shell renders a dedicated access-denied state instead of generic list/mutation errors. SSR never accesses `window` or `localStorage` directly; client-only checks are guarded with `typeof window !== 'undefined'`.

## Risks

- Existing catalog services are public-read oriented, so admin reads must not accidentally inherit `is_hidden = false` filters.
- Relationship updates and slug uniqueness can leave partial catalog data unless handled transactionally and mapped to 409 responses.
- Cookie-based authentication in a separate admin app requires careful CSRF handling if state-changing requests rely on cookies; prefer the established bearer-token flow or add the repository's standard CSRF protection before enabling cookie mutations.
- Review moderation affects public content and counts; updates must preserve aggregate counters and be covered by regression tests.
