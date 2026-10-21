## Context

Issue #509, part 2 of «Книга или экранизация?». Part 1 (#508, archived `2026-10-20-sync-catalog-adaptations`) defined a visible adaptation as a `book_screen_works` row with `NOT hidden` of a published book (`catalog/repository/repository.go` `HasAdaptation` EXISTS). Part 3 (#510, not in scope) will add the automatic collection metrics `book_beats_adaptation` / `adaptation_beats_book`. They will score a book by its most-voted visible adaptation in the nightly auto-collection snapshot, and count a period by the time the signal was given. The table below is shaped for that.

What exists:
- `book_screen_works (book_id, screen_work_id) PK, hidden, source, created_at, updated_at`, from migration 000029, with FKs cascading from `books` and `screen_works`.
- Link deletion: only manual links, via `repo.deleteManualLink`. Work deletion: only works without links (atomic check). Hide/show: `UPDATE ... SET hidden`. The sync is add-only. Work PATCH never touches links. There is no book merge, no screen-work merge or dedup, and no user account deletion today. Author merge only touches `author_screen_works`.
- Default shelf `type = 'read'` («Прочитано»), one per user. Items are in `shelf_items (shelf_id, book_id, added_at)`. Precedent for a cross-module raw read: `reading_challenges/repository` `CountReadBooks`, `curated/repository/auto.go`.
- Ratings (`ratings` module, `POST /books/:id/ratings`, `authMiddleware` only) are the precedent for per-user per-book signals. No rate limiter is applied to ratings or reviews (`pkg/ratelimit` is used for auth and external APIs only).
- The web book page (`views/book-detail/ui/book-detail-view.tsx`) is a server component. It fetches adaptations server-side and renders `entities/screen-work` `BookAdaptations` (server component). Shelf state lives in React Query (`entities/shelf` `shelfKeys.forBook(userId, bookId)`). A shelf toggle calls `features/shelve-book/model/invalidate-shelf-caches.ts` `invalidateShelfContents`.

## Decisions

1. **Open question (author page): no verdict on «Фильмы об авторе».** Author films are biopics and documentaries (`relation` biopic/documentary), not adaptations, and the vote is keyed to a book. Recorded in the spec.
2. **Storage keyed to the link.** Table `adaptation_votes` has PK `(user_id, book_id, screen_work_id)` and a composite FK `(book_id, screen_work_id) REFERENCES book_screen_works ON DELETE CASCADE`. Link delete, book delete (via the link cascade) and any future work delete all remove votes with no code. `user_id REFERENCES users ON DELETE CASCADE` covers a future account deletion. Hide never deletes the row, so votes are kept, and the read queries filter `NOT hidden`.
3. **Values as text**: `value VARCHAR(20) CHECK (value IN ('book','adaptation','same'))`, the same strings as the API. No numeric scale.
4. **Timestamps for #510**: `created_at` is the first vote. `updated_at` is the last change of value, and is "the time the signal was given" for period metrics. A re-PUT with the same value MUST NOT bump `updated_at`: `ON CONFLICT ... DO UPDATE SET value = EXCLUDED.value, updated_at = NOW() WHERE adaptation_votes.value <> EXCLUDED.value`. Index `(book_id, screen_work_id)` serves the per-link aggregate. `updated_at` is indexed for period scans.
5. **Eligibility is checked at write time only.** PUT requires `EXISTS (SELECT 1 FROM shelf_items si JOIN shelves s ON s.id = si.shelf_id WHERE s.user_id = ? AND s.type = 'read' AND si.book_id = ?)`. Leaving «Прочитано» does not touch votes (owner decision), and the shelves module stays unaware of votes. DELETE has no checks and is idempotent (204), so a reader can always retract.
6. **Minimum is 5** (`MinVerdictVotes = 5`, a domain constant, also returned as `min_votes` so the UI does not hardcode it). Per-value counts are `null` below it, so a small sample cannot be read off the API. `votes_count` is always returned (harmless, and useful for a later «ещё N голосов» hint).
7. **Leader, percent and tie are computed on the frontend** from the counts: a pure helper in `entities/screen-work/lib`. #510 defines its own scoring, so the backend does not need a leader field.
8. **Code lives in the screenworks module**: `domain/vote.go` (values, errors `ErrVoteValue`, `ErrNotReader`, `ErrAdaptationNotFound`), a `repository/votes.go` (`UpsertVote`, `DeleteVote`, `ListVerdicts(bookID, userID *uuid)`, `IsReader(userID, bookID)`), `service/votes.go` and `transport/http/votes.go`. Book existence and published status reuse the check of `ListPublic`.
9. **Rate limiting: none new.** This is consistent with ratings and reviews. The PK bounds storage to one row per user and link. Login requires a verified email, and the read-shelf gate raises the cost of sock-puppet voting. Revisit with `pkg/ratelimit` if abuse shows up.
10. **Moderators and admins vote like any reader**, with no extra powers. There is no admin UI in v1.

## API

- `GET /api/v1/books/{id}/adaptations/verdicts`, `optionalAuth` → 200:
  ```json
  {"can_vote": true, "min_votes": 5,
   "items": [{"screen_work_id": "<uuid>", "votes_count": 23, "book_better": 18,
              "adaptation_better": 3, "same": 2, "my_vote": "book"}]}
  ```
  Items cover the visible links in the public order (same ORDER BY as `ListPublic`). Links without votes have `votes_count: 0`. Returns 400 «invalid book id» and 404 «book not found».
- `PUT /api/v1/books/{id}/adaptations/{screen_work_id}/vote`, `authMiddleware`, body `{"value": "book"|"adaptation"|"same"}` → 200 with one verdict item (as above). Errors: 400 «invalid book id» / «invalid screen work id» / «invalid vote value», 401, 403 «only readers who have read the book can vote», 404 «book not found» / «adaptation not found».
- `DELETE /api/v1/books/{id}/adaptations/{screen_work_id}/vote`, `authMiddleware` → 204 (also with no vote). Errors: 400 and 401.
- Register in `RegisterRoutes` next to `ListBookAdaptations`. Swagger annotations on the three handlers (tag `screen-works`), then regenerate `backend/docs`.

## Database

`backend/migrations/000035_adaptation_votes.up.sql`:
```sql
CREATE TABLE adaptation_votes (
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    book_id        UUID NOT NULL,
    screen_work_id UUID NOT NULL,
    value          VARCHAR(20) NOT NULL CHECK (value IN ('book','adaptation','same')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, book_id, screen_work_id),
    FOREIGN KEY (book_id, screen_work_id)
        REFERENCES book_screen_works (book_id, screen_work_id) ON DELETE CASCADE
);
CREATE INDEX idx_adaptation_votes_link ON adaptation_votes (book_id, screen_work_id);
CREATE INDEX idx_adaptation_votes_updated ON adaptation_votes (updated_at);
```
Down: `DROP TABLE adaptation_votes;` (file only; never run it in tasks). Aggregate query: `SELECT l.screen_work_id, COUNT(v.*) , COUNT(*) FILTER (WHERE v.value='book') ... FROM book_screen_works l LEFT JOIN adaptation_votes v ON (v.book_id, v.screen_work_id) = (l.book_id, l.screen_work_id) WHERE l.book_id = ? AND NOT l.hidden GROUP BY l.screen_work_id`, plus the caller's vote by a second small query or a `MAX(v.value) FILTER (WHERE v.user_id = ?)`.

## Frontend

All in `apps/web`. Admin is untouched.
- `packages/api-client`: types `AdaptationVoteValue = 'book' | 'adaptation' | 'same'`, `AdaptationVerdict` (item) and `AdaptationVerdicts` (`{can_vote, min_votes, items}`). Methods `api.books.adaptationVerdicts(bookId)`, `api.books.voteAdaptation(bookId, screenWorkId, value)` (PUT) and `api.books.unvoteAdaptation(bookId, screenWorkId)` (DELETE). The fetcher already sends the session token client-side.
- `entities/screen-work`:
  - `model/keys.ts` exports `screenWorkKeys = { all: ['screen-works'], verdicts: (userId, bookId) => [...all, 'verdicts', userId ?? 'guest', bookId] }`;
  - `api` adds `verdicts`, `vote` and `unvote`;
  - `lib/verdict-label.ts` is a pure `verdictLabel(item, minVotes)` that returns «…% читателей: … · N голос(а/ов)», «Мнения разделились · …» or «Пока мало голосов», using `shared/lib/plural-ru`.
  - `BookAdaptations` gains an optional slot prop `renderExtra?: (item: BookAdaptation) => ReactNode`, rendered at the bottom of each card. It stays a server component, so the entity does not import the feature.
- New feature `features/vote-adaptation`:
  - `model/use-adaptation-verdicts.ts`: a `useQuery` keyed by `screenWorkKeys.verdicts(user?.id, bookId)`. Every card calls it and React Query dedupes. It also holds the vote/unvote mutations (`onSuccess`: `setQueryData` for PUT using the returned item, invalidate for DELETE; `onError`: «Не удалось сохранить голос.»).
  - `ui/adaptation-verdict.tsx` (`'use client'`, props `bookId`, `slug`, `screenWorkId`): renders the verdict line and the controls per the spec. It renders nothing while loading or on error, or when the item is absent. Buttons get `aria-pressed`. Guest link via `loginHref(`/books/${slug}`)`.
- `views/book-detail/ui/book-detail-view.tsx`: `<BookAdaptations items={…} renderExtra={(a) => <AdaptationVerdict bookId={book.id} slug={book.slug} screenWorkId={a.id} />} />`.
- `features/shelve-book/model/invalidate-shelf-caches.ts`: `invalidateShelfContents` also invalidates `screenWorkKeys.all`, so `can_vote` refreshes after a shelf toggle (feature → entity import, FSD-legal).
- `entities/screen-work/ui/author-films.tsx` stays unchanged (no verdict on the author page).

## Existing Patterns

- Per-user per-book signal handler: `ratings/transport/http/handler.go` (`getUserID`, 401/400/404 mapping).
- Public read with optional auth: `GET /books/:id/reviews` with `optionalAuth`.
- Read-shelf membership query: `reading_challenges/repository` `CountReadBooks`.
- Upsert: the `ON CONFLICT` inserts in `screenworks/repository/repository.go`.
- React Query keys plus cross-feature invalidation: `entities/shelf` `shelfKeys`, `features/shelve-book/model/use-book-shelves.ts`.
- Pure label helper with tests: `entities/screen-work/lib/kind-label.ts`.

## Risks

- **Future merges/dedup**: no book merge or screen-work merge exists today. If one is added, it MUST move `adaptation_votes`, keeping one vote per user on a PK conflict (the most recent `updated_at`). The cascade would silently delete them otherwise. Note this in the screenworks repository comment.
- **Self-declared «watched»**: we cannot verify the reader watched the film. The vote note says so, and that is acceptable for v1.
- **Sock puppets**: mitigated only by verified email and the read-shelf gate, with no rate limiter (decision 9).
- **Stale SSR list vs client verdicts**: the adaptation list is server-rendered while verdicts are client-fetched. A link hidden between the two loads simply has no verdict item, and the card renders without a verdict.
- **Hydration**: `AdaptationVerdict` must render nothing on the server and the first client pass (query pending), which avoids hydration mismatches for guests vs signed-in users.
