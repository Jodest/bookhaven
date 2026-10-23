# Tasks

## 1. Backend

- [x] 1.1 Add `syncReviewRatingInTx(tx, bookID, userID, value)` to the ratings repository (`UPDATE reviews SET rating = ? WHERE book_id = ? AND user_id = ?`, no `updated_at`/`content` change) and call it inside `UpsertAndRecalc`'s transaction before the books recalc; update the interface doc comment. Verify: `go build ./...` and `go vet ./...` from `backend/`.
- [x] 1.2 SQLite test for the helper next to `ratings/repository/book_status_sqlite_test.go`: own review (visible and hidden) gets the new value; other users' and other books' reviews untouched; `content`/`updated_at` unchanged; no review row is created when none exists. Verify: `go test ./internal/modules/ratings/...`.
- [x] 1.3 Forward data migration `backend/migrations/000039_sync_review_ratings.up.sql` (`UPDATE reviews r SET rating = ra.value FROM ratings ra WHERE ra.book_id = r.book_id AND ra.user_id = r.user_id AND r.rating <> ra.value;`) and a no-op `.down.sql` with an explanatory comment. Apply with the normal `migrate up` only; never `migrate down`/drop.
- [x] 1.4 Demo seed (`backend/cmd/seed/demo_readers.go`): reviews insert `ON CONFLICT (book_id, user_id) DO UPDATE SET rating = EXCLUDED.rating`. Verify: `go test ./cmd/seed/...`.
- [x] 1.5 Confirm no rating-delete route is added and review delete still keeps the rating (existing reviews service tests pass). Verify: `go test ./internal/modules/reviews/... ./internal/modules/ratings/...`; gofmt only the changed files.

## 2. Frontend (apps/web)

- [x] 2.1 Add `apps/web/src/entities/rating/` with a non-persisted zustand store `{ byBook, setMyRating(bookId, value) }` and a public `index.ts`; unit test for set/read.
- [x] 2.2 `features/rate-book/ui/book-rating.tsx`: write the store after loading my rating and after a successful save (not on failure); show the store value for the book when present. Keep server/client markup identical. Extend `book-rating.test.tsx`: success updates the store, failure does not, external store change updates «Моя оценка: N».
- [x] 2.3 `widgets/reviews-section/ui/reviews-section.tsx`: on store change patch the own review's rating in `myReview` and `items` (guard "value differs"); push `my_review.rating` into the store after fetching; `router.refresh()` after a review save; confirmation text «Удалить рецензию? Ваша оценка книги сохранится.». Add/extend a test: own card rating follows a store change; confirm text; cancelling the confirm deletes nothing.
- [x] 2.4 Verify: `tsc --noEmit` and `next lint` in `apps/web` (no native `next build`; CI is the typedRoutes gate).

## 3. Manual check (local compose stack)

- [x] 3.1 (deferred to the human reviewer: unchecked item in the dev PR checklist, not performed by the pipeline) Review a book with 5, re-rate it with the stars to 2: the own review shows 2 without reload, and after reload the reviews list, «Моя оценка: 2» and the summary all agree.
- [x] 3.2 (deferred to the human reviewer: unchecked item in the dev PR checklist, not performed by the pipeline) Edit the review to 4: the stars show «Моя оценка: 4» and the summary updates without reload.
- [x] 3.3 (deferred to the human reviewer: unchecked item in the dev PR checklist, not performed by the pipeline) Delete the review: the confirmation shows the new text; after confirming, «Моя оценка» stays and the review count drops.
