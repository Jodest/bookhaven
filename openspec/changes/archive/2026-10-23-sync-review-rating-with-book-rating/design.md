# Design

## Context

`ratings` (one row per user+book, unique `(book_id, user_id)`) is the source of book aggregates. `reviews.rating` is a duplicated copy written only by the reviews module (`reviews/repository.Create/Update` call `upsertRatingInTx` + `recalcInTx`). The ratings module (`ratings/repository.UpsertAndRecalc`) writes `ratings` and `books` but never `reviews`, so a direct re-rate leaves the review stale (#381). Review delete (`reviews/repository.Delete`) deletes only the review and recalcs — rating kept (#380, product decision: keep, just tell the user).

There is no endpoint or UI to remove a rating (routes: `POST /books/:id/ratings`, `GET /books/:id/ratings/me` only); the widget has no "clear". So "review without a rating" is reachable only through account deletion (both rows cascade) — no new handling needed; the spec makes "cannot be removed" explicit so a future clear-rating feature has to resolve the review case.

`reviews.rating` is read in: reviews list (`reviews/repository.ListByBook`), GORM `GetByID`/`GetByUserAndBook` (`my_review`, edit/delete paths), admin review moderation (`admin/repository/repository.go` ~l.865), home review rows (`home/repository/repository.go` `reviewsByIDsQuery`), possibly users profile.

## Decisions

1. **Sync the duplicated column on write (chosen)** rather than reading the rating from `ratings` / dropping `reviews.rating`.
   - Only two write paths change the rating (`ratings` upsert and review create/update; plus the demo seed), versus 4+ read paths (reviews list, GORM model reads, admin moderation, home rows) that would all need a join and a model change. One extra `UPDATE` in an existing transaction is the smallest, lowest-risk change, keeps API shapes and the GORM model untouched.
   - Atomic: done inside the existing `UpsertAndRecalc` transaction, so rating and review never diverge after commit.
   - Dropping the column is a schema migration with a non-restorable down and touches every reader — not worth it for a solo-maintained app. Rejected.
2. In `ratings/repository.UpsertAndRecalc`, after the `INSERT ... ON CONFLICT` and before the books recalc, call a new helper `syncReviewRatingInTx` that runs
   `UPDATE reviews SET rating = ? WHERE book_id = ? AND user_id = ?` (value, bookID, userID). Applies to hidden reviews too (they come back as `my_review`). Do **not** touch `updated_at` — the review was not edited by its author (avoid any "edited" signal / reordering). Do not touch `reviews_count`. Raw SQL against another module's table mirrors the existing precedent (`reviews` repo writes `ratings`/`books`/`notifications` directly). Update the interface doc comment.
3. Review create/update already write `ratings` from the review — unchanged; together with (2) both directions are covered.
4. **Backfill** existing drift with a forward data-only migration `000039_sync_review_ratings`:
   - up: `UPDATE reviews r SET rating = ra.value FROM ratings ra WHERE ra.book_id = r.book_id AND ra.user_id = r.user_id AND r.rating <> ra.value;`
   - down: no-op comment (old stale values are not restorable), same style as `000031_*.down.sql`.
   - Reviews without a rating row (should not exist) are left as is; no aggregate recalc needed (aggregates come from `ratings`).
5. **Demo seed** (`backend/cmd/seed/demo_readers.go`): the reviews insert uses `ON CONFLICT (book_id, user_id) DO NOTHING`; change to `DO UPDATE SET rating = EXCLUDED.rating` so a re-seed with changed demo ratings cannot reintroduce drift (content untouched).
6. **Delete keeps the rating** — backend unchanged; only the confirmation text changes.
7. **Frontend sync on the book page** via a tiny shared store in the entities layer (FSD: `features/rate-book` and `widgets/reviews-section` may both import entities, never each other).

## API

No new endpoints, no response shape changes.
- `POST /api/v1/books/{id}/ratings` (bearer, any role; book must be published): additionally updates the caller's review rating in the same transaction.
- `DELETE /api/v1/books/{id}/ratings` stays unregistered (Echo answers 405); do not add it.

## Database

- New migration `backend/migrations/000039_sync_review_ratings.{up,down}.sql` (data-only, see Decision 4). Never edit existing migrations.

## Frontend

apps/web only.

- New `apps/web/src/entities/rating/` (`model/my-rating-store.ts`, `index.ts`): zustand store (same lib as `entities/session/model/auth-store.ts`) `{ byBook: Record<string, number>; setMyRating(bookId, value) }`, not persisted.
- `features/rate-book/ui/book-rating.tsx`:
  - after `api.ratings.my` resolves, and after a successful `api.ratings.set`, call `setMyRating(bookId, value)`; on failure do not touch the store (rollback stays local as today);
  - derive the shown value from the store when it has an entry for `bookId` (so a review save updates «Моя оценка: N»). Keep SSR/client markup identical (store starts empty on both → 0 → «Оцените книгу», as today).
- `widgets/reviews-section/ui/reviews-section.tsx`:
  - subscribe to `byBook[bookId]`; when it changes and differs from the own review's rating, patch `myReview.rating` and the matching own item in `items` (match by `user_id === user.id`) locally — no refetch;
  - after `fetchReviews` returns `my_review`, call `setMyRating(bookId, my_review.rating)` (covers create/edit; equal values are a no-op);
  - in the review save success path call `router.refresh()` (next/navigation) so the server rating summary updates (deletion does not change the rating, no refresh needed);
  - confirm text: `confirm('Удалить рецензию? Ваша оценка книги сохранится.')`.
- The review edit form opens with `initialReview = myReview`, so after a star change it pre-fills the new rating automatically.

## Existing Patterns

- Transactional raw SQL across tables: `reviews/repository.upsertRatingInTx`, `recalcInTx`, `upsertNotificationInTx`.
- Data-only migration with no-op down: `backend/migrations/000031_curated_collections_unalias_genres.*`.
- zustand store in entities: `apps/web/src/entities/session/model/auth-store.ts`.
- Optimistic star update + rollback: `features/rate-book/ui/book-rating.tsx`.

## Risks

- Other readers of `reviews.rating` (admin moderation list, home review rows) become correct automatically after the backfill — no code change there, but verify nothing caches review payloads (home payload may be cached in Redis; stale for at most the cache TTL — acceptable).
- Store/state ping-pong in ReviewsSection: guard the effect with "value differs" to avoid loops.
- Testing: there is no real-Postgres harness (see the comment in `reviews/repository/repository_test.go`), and `UpsertAndRecalc` uses Postgres-only SQL (`xmax`, `gen_random_uuid`). Put the sync in a small helper `syncReviewRatingInTx(tx, bookID, userID, value)` with portable SQL and test it on SQLite the way `ratings/repository/book_status_sqlite_test.go` does (review row updated, other users' / other books' reviews untouched, `updated_at` and `content` unchanged, no row created when there is no review). Manual end-to-end check on the local compose stack; never run destructive DB commands.
