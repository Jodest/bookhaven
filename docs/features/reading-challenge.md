# Reading Challenge

Status: implemented

## Goal

Let an authenticated user set an annual reading goal ("read N books in YYYY") and see live progress toward it, computed from books they have marked as read — similar to Goodreads' yearly reading challenge.

## User Stories

- As an authenticated user, I can set or change my reading goal (number of books) for a given year.
- As an authenticated user, I can see my progress for the current year: books read vs. goal, as a progress bar/percentage.
- As an authenticated user, I can see my goal and progress for past years I set one for (history).
- As an authenticated user without a goal set yet for the current year, I can see how many books I've already read this year and am prompted to set a goal.
- As a user, I cannot see or modify another user's reading challenge.

## Acceptance Criteria

- [x] `PUT /api/v1/reading-challenges/:year` creates or updates (upserts) the caller's goal for that year; returns the goal and current computed progress.
- [x] `GET /api/v1/reading-challenges/:year` returns the caller's goal (nullable if unset) and computed progress (books read, percent, completed flag) for that year.
- [x] `GET /api/v1/reading-challenges` returns the caller's challenge history (all years with a goal, newest first), each with computed progress.
- [x] Progress is always computed live from the user's "read" shelf items for the requested year — never a stale stored counter.
- [x] Goal validation: integer, 1–10000; year validation: 2000 up to current year + 1. Invalid input returns `400` with a Russian-mappable error.
- [x] All endpoints require authentication and operate only on the authenticated user's own data (no user-ID route/query parameter exists to request another user's challenge).
- [x] Frontend page shows the current year's goal, a progress bar, "X of Y books read," a form to set/update the goal, and a year selector for history.
- [x] Setting a goal for the current year is reachable from primary navigation.
- [x] All new user-facing text is in Russian, including validation and empty/prompt states.

## Business Rules

- One challenge row per `(user_id, year)` — enforced by a unique constraint; `PUT` is an upsert, not a duplicate-create.
- `goal` must be a positive integer between 1 and 10000.
- `year` must be between 2000 and `current_year + 1` (server clock, UTC), allowing users to plan next year's goal slightly ahead but preventing garbage data.
- Ownership is implicit: the resource key is `(authenticated user_id, year)`, taken from the auth token, never from a route/body user ID. There is no cross-user lookup endpoint for this MVP, which removes the IDOR surface by construction.
- Progress = number of distinct books currently on the user's `read`-type shelf whose `shelf_items.added_at` falls within `[year-01-01T00:00:00Z, year-12-31T23:59:59Z)`.
  - Known limitation (see Risks): BookHaven has no explicit "date finished" field. `added_at` on the `read` shelf is set at the moment a book is moved onto that shelf (see `shelves` service `AddBook`/`RemoveBook` transaction, which re-inserts a fresh `shelf_items` row — and thus a fresh `added_at` — whenever a book moves onto the `read` shelf). This is used as the completion-date proxy, consistent with how the existing shelves feature already works.
  - If a book is later removed from the `read` shelf (e.g., moved back to `reading`, or removed entirely), it no longer counts toward that year's progress on the next read — progress is not cached or frozen at goal-completion time.
- `progressPercent = min(100, round(booksRead / goal * 100))` when a goal exists; `null` when no goal is set for that year (the endpoint still returns `booksRead` so the UI can prompt goal creation).
- `completed = booksRead >= goal` when a goal exists.
- A user may set/update a goal for a past year (e.g., to backfill), but there is no retroactive notification/achievement feature — out of scope.

## UX

- New authenticated-only page (e.g. `apps/web/src/app/challenge/page.tsx`), linked from the header nav next to "Мои полки" / "Мои списки" (e.g. "Читательский вызов").
- Default view: current year, showing a progress bar, "Прочитано X из Y книг" and percent, plus an edit affordance for the goal (inline number input + save button, following the shelves rename inline-edit pattern).
- If no goal is set for the current year: show the current read count (if any) and a prompt/form to set a goal instead of a progress bar.
- Year selector (simple `<select>` or link list) to view past years' goals/progress; years with no stored goal but a nonzero read count may still be listed read-only.
- Russian loading, empty (no goal yet), validation-error, and save-error states, following the profile page's state-handling pattern.
- Not adding a public/other-user view for this MVP (goal-setting/progress is private to the owner, unlike public lists/profile activity).

## API

- `PUT /api/v1/reading-challenges/:year` (auth required) — body `{ "goal": number }`. Upserts the caller's goal for `:year`. Returns the same shape as `GET /:year`.
- `GET /api/v1/reading-challenges/:year` (auth required) — returns `{ year, goal: number | null, booksRead: number, progressPercent: number | null, completed: boolean }`.
- `GET /api/v1/reading-challenges` (auth required) — returns `{ items: [...same shape as above], total: number }`, one entry per year the caller has a stored goal, newest first.
- Derive `user_id` exclusively from the auth middleware context (`c.Get("user_id")`), matching the existing `getUserID` helper pattern in shelves/lists/reviews handlers. No endpoint accepts a user ID.
- Validation errors → `400`; missing/invalid `:year` (non-integer or out of range) → `400`; unauthenticated → `401`. No `404` case is needed since `GET`/`PUT` on an unset year is valid (goal is simply `null` until first `PUT`).

## Database

New module table, added via a new migration (next sequence number after `000007`):

```sql
CREATE TABLE reading_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    goal INTEGER NOT NULL CHECK (goal > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, year)
);

CREATE INDEX idx_reading_challenges_user_id ON reading_challenges(user_id);
```

No changes to `shelves`/`shelf_items` — progress is computed via a query joining `shelf_items` to the user's `read`-type shelf and filtering `added_at` by calendar year (see Business Rules), not stored.

## Frontend

- Add a `readingChallenges` client to `packages/api-client/src/index.ts` following the `shelves`/`users` client pattern (raw snake_case DTO + `normalize*` mapping to camelCase, matching `normalizeUser`).
- New types: `ReadingChallenge { year: number; goal: number | null; booksRead: number; progressPercent: number | null; completed: boolean }` and `UpsertReadingChallengeInput { goal: number }`.
- New page `apps/web/src/app/challenge/page.tsx` (owner-only; redirect/prompt like `apps/web/src/app/profile/page.tsx` when unauthenticated).
- Add nav link in `apps/web/src/components/header.tsx` alongside the existing authenticated links.
- Progress bar: simple Tailwind div (width via inline style/percent), no new shared UI primitive required; only add a `Progress` component to `packages/ui` if reused a second time.

## Existing Patterns

- Backend module layout: mirror `backend/internal/modules/shelves` (domain/repository/service/transport-http), including transaction-free simple CRUD since there's no multi-shelf exclusivity concern here.
- Auth/ownership: reuse `getUserID(c)` helper pattern from `backend/internal/modules/shelves/transport/http/handler.go`; register routes under `authMiddleware` only, like the `/shelves` group in `backend/internal/transport/http/router.go`.
- Wiring: add `readingchallengerepo`/`readingchallengeservice` construction in `backend/cmd/api/main.go` next to `shelvesRepo`/`shelvesService`, and add the handler + routes to `RegisterRoutes` in `backend/internal/transport/http/router.go`.
- Progress-by-join query style: follow `ListByUser` in `backend/internal/modules/shelves/repository/repository.go`, which already joins `shelf_items` to compute `books_count` per shelf — same idea, filtered by shelf type `read` and a year range on `added_at`.
- Frontend data-fetch/mutate/re-fetch and inline-edit pattern: follow `apps/web/src/app/shelves/page.tsx` and `apps/web/src/app/profile/page.tsx` (loading/auth-guard states, Russian error mapping, `useForm` + `zodResolver` for the goal form).

## Tests

- Backend service tests: upsert creates then updates the same `(user_id, year)` row (no duplicates); goal validation boundaries (0, negative, >10000, non-integer); year validation boundaries (before 2000, more than current_year+1 ahead); progress computed correctly from `read`-shelf `shelf_items` within the requested calendar year (including a book added to `read` in a different year not counting); a book removed from `read` no longer counts; goal-unset year returns `goal: null` and correct `booksRead`.
- Backend handler tests: `401` when unauthenticated on all three endpoints; a user's request never reads/writes another user's row (no user-ID param exists to attempt IDOR, but verify two distinct authenticated users get independent results against the same year).
- Frontend: current-year default view with progress bar and correct percentage/text; goal form validation and save/error states; no-goal-yet prompt state; year-history selection rendering past years; unauthenticated guard state.

## Documentation

- Add `- [ ] Reading Challenge` under an appropriate Roadmap section in `docs/ROADMAP.md` pointing to this contract; mark completed when shipped.

## Risks

- No explicit "date finished" field exists on books/shelves; using `shelf_items.added_at` on the `read` shelf as a completion-date proxy is a reasonable reuse of existing data but means: (a) re-shelving a book (e.g., re-reading, or moving it off and back onto `read`) changes its counted year, and (b) users who marked books read before this feature existed will have their historical `added_at` values counted for whatever year they happen to fall in, which may not match when they actually finished the book. This should be called out in the UI copy rather than solved with new schema for this MVP.
- If BookHaven later adds an explicit "finished reading" date (a natural future enhancement), this feature's progress query will need to be updated to prefer that field over shelf `added_at` — worth a short code comment at the query site.
