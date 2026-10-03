# Public Shelf Visibility

Status: implemented

## Goal

Show a user's reading-status/custom shelves on their public profile, the same way public lists are already shown, resolving OQ-003 from `docs/ai/features/user-book-shelves/business-analysis.md` (deferred to the profiles phase, which is now complete).

## User Stories

- As a visitor (authenticated or anonymous) viewing another user's public profile, I can see that user's shelves (e.g. "Читаю", "Прочитано", any custom shelf) with book counts, so I can discover what they're reading.
- As a visitor, I can open a shelf from a public profile and browse its books, the same way I can open a public list.
- As a profile owner, I cannot hide a shelf from my public profile — shelf visibility has no per-user or per-shelf opt-out, consistent with the rest of the public-profile model.

## Acceptance Criteria

- [x] `GET /api/v1/users/:id` response includes a `shelves` array alongside the existing `lists` array: for each of the user's shelves (default and custom) that has at least one non-hidden book, `id`, `name`, `type`, `books_count`, `created_at`. Shelves with zero visible books are omitted.
- [x] `books_count` on the public shelves list and on the public shelf detail counts only books with `is_hidden = false`, matching the catalog-visibility pattern used elsewhere (shelf content, `BookExists`, etc.).
- [x] A new `GET /api/v1/shelves/public/:id` endpoint (optional auth) returns the shelf (`id`, `name`, `type`, `books_count`) plus a paginated, visible-only book list (`page`, `limit`, `total`), mirroring `GET /api/v1/lists/public/:id`. No ownership check is performed — any existing shelf ID is viewable, by product decision (see Business Rules).
- [x] An invalid or non-existent shelf ID on the public endpoint returns 404.
- [x] The public profile page (`apps/web/src/views/public-profile/ui/public-profile-view.tsx`) renders a "Полки" section below "Публичные списки", listing each returned shelf with its Russian label (for default types) or custom name, and book count, linking to a new public shelf detail page.
- [x] A new public shelf detail page/view (mirroring `apps/web/src/app/lists/public/[id]/page.tsx` + `public-list-detail-view.tsx`) renders the shelf's books with pagination and Russian loading/empty/error states.
- [x] No edit/remove-book controls appear on the public shelf view for non-owners (read-only, consistent with public lists).
- [x] A profile/shelf with no visible shelves renders an explicit Russian empty state ("Полок пока нет" or similar), not a blank section.
- [x] `@bookhaven/api-client`'s `PublicProfile` type gains a `shelves` field; a new `api.shelves.getPublic(id, params)` method is added, mirroring `api.lists.getPublic`.

## Business Rules

- **Resolved product decision (OQ-003): no granular privacy control.** Shelves are shown on public profiles unconditionally — there is no per-user "hide my shelves" setting and no per-shelf public/private flag. This matches the precedent already set by the top-level public profile (all-or-nothing; no profile-hiding toggle exists) and by reviews (no visibility flag). It deliberately does **not** copy the per-resource `List.IsPublic` pattern: introducing an equivalent flag for shelves would require a new migration and would be the first time this feature adds schema specifically for shelf privacy — something `docs/ai/features/user-book-shelves/business-analysis.md` explicitly scoped out ("Приватность полок... в схеме нет соответствующего поля") and OQ-003 deferred rather than resolved. This contract resolves it as: ship with no toggle; revisit only if a future backlog item makes the case for one.
- A shelf is included in the public listing only if it has at least one visible (`is_hidden = false`) book. Empty shelves (e.g. a new user's un-materialized or just-emptied default shelves) are omitted entirely — this keeps profiles from being cluttered with "Хочу прочитать · 0" for every visitor, consistent with how a user with no public lists simply shows an empty lists section rather than private/empty placeholders.
- Both default (`want_to_read`, `reading`, `read`) and custom shelves are included on equal footing — no shelf type is special-cased or excluded. This mirrors how lists are shown regardless of their user-chosen title/description.
- The public shelf detail endpoint has no owner check: because shelves are unconditionally public by this decision, any existing shelf ID is viewable by anyone, same as `GetPublicList` has no owner check (it checks `is_public = true` instead — shelves have no equivalent flag to check, so the check is simply absence of row-not-found).
- `books_count` must reflect only visible books, reusing the existing `is_hidden` filtering pattern (already used in `ListBooks`, `BookExists` in the shelves repository).

## UX

- Public profile (`apps/web/src/views/public-profile/ui/public-profile-view.tsx`): add a "Полки" section after "Публичные списки", same visual treatment (bordered list items, Russian labels for default shelf types reusing the existing label pattern in `shelve-book-buttons.tsx`/`shelves-view.tsx`: `want_to_read` → «Хочу прочитать», `reading` → «Читаю», `read` → «Прочитано»; custom shelves show their stored name). Each item shows the book count and links to the public shelf detail page.
- New public shelf detail page/view, mirroring `public-list-detail-view.tsx`: shelf name/type as heading, paginated grid of books (same card style as catalog/own shelves), Russian loading/error/empty states, no mutation controls.
- Empty state: if a profile has zero qualifying shelves, show a short Russian message in the shelves section (e.g. "Публичных полок пока нет") instead of hiding the section inconsistently.

## API

- Extend `GET /api/v1/users/:id` (optional auth, existing): add `shelves` to `publicProfileResponse`, each item: `id`, `name`, `type`, `books_count`, `created_at`. Wire a new `ShelfService` interface (analogous to the existing `ListService` interface in `backend/internal/modules/users/transport/http/handler.go`) exposing `ListPublicByUser(ctx, userID) ([]shelvesDomain.Shelf, error)`, implemented by `shelves/service.Service`. Update `users/transport/http.NewHandler` to accept the shelves service, and update its call site in `backend/cmd/api/main.go`.
- Add `GET /api/v1/shelves/public/:id` (optional auth), registered next to the existing `/lists/public/:id` route in `backend/internal/transport/http/router.go`. Response mirrors `shelfDetailResponse` (shelf + paginated `items`/`total`/`page`/`limit`), reusing `toBookListItemResponse`/`bookListItemResponse`. Validation: invalid UUID → 400; missing shelf → 404. No 403 case (no ownership check — see Business Rules).
- New shelves service methods:
  - `ListPublicByUser(ctx, userID uuid.UUID) ([]domain.Shelf, error)`: returns the user's shelves with `BooksCount` computed over visible books only, filtered to shelves with `BooksCount > 0`.
  - `GetPublicShelf(ctx, shelfID uuid.UUID, page, limit int) (*domain.Shelf, []catalogDomain.Book, int64, error)`: like the existing `GetShelf` but without the `userID`/`ErrForbidden` ownership check (reuses `repo.ListBooks`, which already filters `is_hidden`).
- New/updated repository method: a visible-books-aware variant of `ListByUser` (the current `ListByUser` SQL counts all `shelf_items` rows regardless of `is_hidden` — note this as a pre-existing gap relative to FR-014 in the shelves business analysis; this feature must not propagate it into the new public-facing count, even if fixing the authenticated `/shelves` count is out of scope).

## Database

No schema changes. No new migration. This is a deliberate part of the OQ-003 resolution (see Business Rules) — shelves gain no new columns for this feature.

## Frontend

- `packages/api-client/src/index.ts`:
  - Extend `PublicProfile` with `shelves: { id: string; name: string; type: Shelf['type']; booksCount: number; createdAt: string }[]`.
  - Update `api.users.getPublic` to map the new `shelves` array (snake_case → camelCase, same pattern as the existing `lists` mapping).
  - Add `api.shelves.getPublic(id, params?: { page?: number; limit?: number })`, mirroring `api.lists.getPublic`, hitting `GET /api/v1/shelves/public/:id` and returning `{ shelf, items, total, page, limit }` with `normalizeBook`.
- `apps/web/src/views/public-profile/ui/public-profile-view.tsx`: render the new shelves section (see UX).
- New `apps/web/src/views/public-shelf-detail/ui/public-shelf-detail-view.tsx` (naming to match the existing `public-list-detail` convention) + `apps/web/src/app/shelves/public/[id]/page.tsx`, mirroring the public list detail page/view pair.
- No changes to the owner-only `/shelves` page or its controls.

## Existing Patterns

- Mirror `backend/internal/modules/lists/service.ListPublicByUser` / `GetPublicList` and `backend/internal/modules/lists/transport/http.GetPublicList` for the two new shelves service/handler methods — same shape, swap the "is_public" filter for the "has visible books" filter since shelves have no public/private flag.
- Mirror `apps/web/src/app/lists/public/[id]/page.tsx` + `apps/web/src/views/public-list-detail/ui/public-list-detail-view.tsx` for the new public shelf detail page/view.
- Reuse `backend/internal/modules/users/transport/http.Handler`'s existing `ListService` interface pattern for the new `ShelfService` interface (optional-auth, read-only, injected at construction).
- Reuse the shelf-type Russian label mapping already present in `apps/web/src/features/shelve-book/ui/shelve-book-buttons.tsx` (`want_to_read`/`reading`/`read` labels) instead of re-inventing it.
- Reuse the `is_hidden = false` visibility filter already applied in `shelves/repository.ListBooks` and `BookExists`.

## Tests

- Backend:
  - Service tests: `ListPublicByUser` returns only shelves with ≥1 visible book, with correct visible-only counts; a shelf whose only book is hidden is excluded; a user with no shelves/no visible books returns an empty slice (not an error).
  - Service tests: `GetPublicShelf` returns paginated visible books for any shelf ID regardless of owner; unknown shelf ID returns `ErrShelfNotFound`; hidden books are excluded from items and `total`.
  - Handler tests: `GET /api/v1/users/:id` response includes the new `shelves` field with the documented shape; `GET /api/v1/shelves/public/:id` returns 200 for any existing shelf (including one owned by someone other than the caller, and for an anonymous caller), 400 for a malformed ID, 404 for a missing shelf; pagination params behave like the existing `GetShelf`/`GetPublicList` handlers.
- Frontend: public-profile view renders the shelves section with correct labels/counts and an empty state when there are none; public shelf detail view renders paginated books, Russian states, and no owner controls; `api-client` unit/type coverage for the new `shelves` field and `shelves.getPublic` method if existing coverage conventions call for it.

## Documentation

- Update `docs/ROADMAP.md`: move "#95 Public shelf/reading-status visibility on user profiles" from "Up Next" to "Completed" once shipped, linking this contract.
- Consider a short note in `docs/ai/features/user-book-shelves/business-analysis.md` or its state file marking OQ-003 as resolved (no privacy control; always shown) — optional, at the implementer's/reviewer's discretion, not a blocking requirement of this contract.

## Risks

- The issue text describes the current public profile as exposing "lists/reviews," but the shipped `GET /api/v1/users/:id` and `public-profile-view.tsx` only expose public **lists** today — there is no reviews section on the public profile. This contract only extends the lists precedent to shelves; it does not add a reviews section (out of scope, not requested by the ROADMAP item).
- The shelves module in code has already diverged from the original `user-book-shelves` business analysis: only 3 default shelf types are lazily materialized (`want_to_read`, `reading`, `read` — no `favorite`/`abandoned` materialization found in `EnsureDefaultShelves`), and unlimited custom shelves are already implemented, even though ROADMAP's "Proposed" section (#94) still lists custom shelves as an unresolved backlog item. This contract treats "all of a user's shelves, default and custom" as the scope for public visibility regardless of that documentation/code mismatch, since reconciling it is unrelated to this feature.
- The existing authenticated `ListByUser` repository query counts all `shelf_items` rows without filtering `is_hidden`, which likely under-counts hidden-book exclusion on the owner's own `/shelves` page too (a pre-existing gap vs. FR-014 in the original business analysis). This feature must implement its own visible-books-only count for the public listing rather than reusing that query as-is; fixing the owner-facing count is explicitly out of scope here but worth flagging to the developer/reviewer.
- No ownership check on `GET /api/v1/shelves/public/:id` is intentional (shelves are unconditionally public by this contract's resolution of OQ-003), but it does mean any authenticated or anonymous caller can enumerate any shelf's contents by ID if they obtain/guess it — acceptable because it's the explicit product decision, not an oversight; double-check this reasoning survives review before merging.
