# Review-Like Notifications

Status: implemented

## Goal

When another user likes your review, you find out via an in-app notification (bell icon + unread badge in the header), instead of having to notice the like count changed. Architect the notification storage/API so future event types (new follower, comment on review, etc.) can be added without a redesign.

## User Stories

- As a review author, when someone else likes my review, I want to see a notification telling me who liked it and which review, so I know my content got engagement.
- As a user, I want an unread-count badge on a bell icon in the header so I notice new activity without actively checking.
- As a user, I want to open a dropdown/feed of my recent notifications, mark them read, and jump to the relevant review.

## Acceptance Criteria

- [x] Liking someone else's review creates exactly one notification for the review's author.
- [x] Liking your own review never creates a notification.
- [x] Unliking a review does not delete or retract a previously created notification.
- [x] Repeated like/unlike/like cycles on the same review by the same actor reuse (upsert) a single notification row rather than spamming duplicates; the existing row is refreshed to unread with a fresh timestamp.
- [x] A user can only ever see, count, or mark-read their own notifications (ownership enforced server-side, not just hidden in the UI).
- [x] Header shows a bell icon with an unread-count badge for authenticated users only; badge is absent/zero when there are no unread notifications.
- [x] Opening the bell shows a paginated-enough (first page, e.g. 20) list of recent notifications, newest first, each rendering actor name/avatar, "liked your review of {book title}", relative time, and read/unread state.
- [x] Clicking a notification marks it read and navigates to the relevant book page.
- [x] A "mark all as read" action clears the unread badge.
- [x] Deleting a review (or the acting/recipient user, cascade) removes its notifications — no dangling references to deleted entities.

## Business Rules

- Only the `review_like` event type exists in this increment; the schema and API must not require a breaking change to add a second type later.
- No email notifications in this increment — in-app only (see UX below for why).
- Self-likes (if ever possible) never generate a notification.
- Unread badge counts are per-recipient; no cross-user leakage.
- A notification is "retried"/refreshed on duplicate like, not duplicated — bounds row growth from toggle-spam.

## UX

Codebase check: there is no existing user-facing async/real-time feedback mechanism anywhere in the app — no toast library, no WebSockets/SSE, no polling-based notification concept, nothing. The one existing precedent for "live-ish" UI updates is `docs/features/harvest-progress.md`, which explicitly chose **5-second fixed-interval polling via `useQuery({ refetchInterval })`** over WebSockets/SSE for the admin harvest progress bar, and documents that decision as deliberate and not revisited. Follow the same approach here: a bell icon in `widgets/header` with an unread-count badge, polled via `useQuery({ refetchInterval: 30_000 })` against a lightweight `GET /notifications/unread-count` endpoint (30s instead of harvest's 5s — this is a non-urgent social signal, not an active job progress bar, so a less aggressive interval is appropriate and cheaper). The dropdown list itself loads on open (not polled continuously).

No toast/snackbar on arrival — the badge is sufficient and avoids introducing a new UI primitive for one feature.

- Bell icon (lucide-react, already a dependency) rendered in `Header`, next to the profile link, only when `isAuthenticated`.
- Badge shows unread count (cap display at e.g. "9+" if desired — minor, dev's call).
- Dropdown/panel lists notifications with actor avatar/name, message, relative time, unread items visually distinguished (e.g. bold/background tint).
- Click a notification → mark it read (optimistic) → navigate to `/books/{slug}` for the liked review's book.
- "Mark all as read" link/button in the dropdown footer.
- Empty state: "No notifications yet" (or Russian equivalent, matching the app's Russian-language UI — see existing strings like "Поиск книг...", "Мои полки").

## API

New `notifications` module (`backend/internal/modules/notifications`), auth-required, user-scoped:

- `GET /api/v1/notifications?page=&limit=` — paginated list of the current user's notifications, newest first. Default limit 20, max 100 (matches reviews pagination convention).
- `GET /api/v1/notifications/unread-count` — `{ "count": number }`, cheap query (`COUNT(*) WHERE user_id = ? AND is_read = false`), the one polled on an interval.
- `PATCH /api/v1/notifications/:id/read` — marks one notification read. 404 if it doesn't exist *or* doesn't belong to the current user (don't leak existence of other users' notifications).
- `POST /api/v1/notifications/read-all` — marks all of the current user's unread notifications read.

Response item shape (`review_like` is the only `type` today, but the shape should read naturally for future types):

```json
{
  "id": "uuid",
  "type": "review_like",
  "actor": { "id": "uuid", "name": "...", "avatar_url": "..." },
  "review": { "id": "uuid", "book_id": "uuid", "book_slug": "...", "book_title": "..." },
  "is_read": false,
  "created_at": "..."
}
```

All four routes go under a `requireAuth`-gated `api.Group("/notifications", authMiddleware)` in `router.go`, same pattern as `/shelves`, `/lists`, `/reading-challenges`.

No changes to the existing `POST /reviews/:id/likes` response contract — notification creation is a side effect of that handler's service call, not a new field in `toggleLikeResponse`.

## Database

New migration (next sequential number, `000010_notifications`):

```sql
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,   -- recipient
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- who triggered it
    type VARCHAR(32) NOT NULL,                                      -- 'review_like' today
    review_id UUID REFERENCES reviews(id) ON DELETE CASCADE,        -- type-specific payload; nullable so future types that aren't review-related don't need it
    is_read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (actor_id, review_id, type)
);

CREATE INDEX idx_notifications_user_unread ON notifications(user_id, is_read, created_at DESC);
```

Extensibility note for the next event type (e.g. "new follower"): add its own nullable FK column (e.g. `followed_user_id`) rather than redesigning this table into a generic polymorphic/JSON payload — keeps queries simple and typed, matches this codebase's preference for explicit columns over generic blobs (see `reviews`/`ratings`/`shelves` schemas, none of which use JSON payload columns).

The `UNIQUE (actor_id, review_id, type)` constraint is what makes repeated like/unlike/like cycles idempotent: the insert becomes `ON CONFLICT (actor_id, review_id, type) DO UPDATE SET is_read = false, created_at = NOW()`.

## Backend Design

- New module `notifications` (domain / repository / service / transport), Clean Architecture, same shape as `reading_challenges` (closest comparable small module).
- **Notification creation must happen inside the existing like-toggle transaction, not as a separate best-effort call.** Precedent: `reviews/repository.GormRepository.ToggleLike` and `.Create`/`.Update`/`.Delete` already perform direct cross-module writes inside their `db.Transaction(...)` blocks (updating `ratings` and `books` tables via raw SQL — see `upsertRatingInTx`/`recalcInTx`). Follow the same precedent: extend `reviews/repository.Repository.ToggleLike` to accept the review's author ID (`revieweeID uuid.UUID` — the caller, `reviews/service.Service.ToggleLike`, already has it from its prior `GetByID` call, no extra query needed), and within the same transaction, when a *new* like is created (`liked == true`) and `revieweeID != userID`, upsert the `notifications` row with the `ON CONFLICT` shown above.
- This keeps the notification write atomic with the like it describes, with no risk of a crash leaving a like with no notification (or a stray notification with no like).
- `notifications` module itself stays independent of `reviews` (no import of reviews' domain/service) — the coupling is one-directional (reviews → notifications' table, via SQL, same style as reviews → ratings/books today), so there's no import cycle risk.
- `notifications/service.Service` exposes `List`, `UnreadCount`, `MarkRead(ctx, userID, notificationID)`, `MarkAllRead(ctx, userID)` — all taking the authenticated `userID` explicitly and scoping every query by it (ownership is enforced in the query itself, e.g. `WHERE id = ? AND user_id = ?`, returning not-found rather than forbidden to avoid existence leakage, consistent with how `reviews` handles `ErrForbidden` vs `ErrReviewNotFound` today... except here even "forbidden" should read as 404, per the point above).
- List query joins `notifications n`, `users actor ON actor.id = n.actor_id`, `reviews r ON r.id = n.review_id`, `books b ON b.id = r.book_id` — same raw-SQL-join style as `reviews/repository.ListByBook`.

## Frontend

- `apps/web/src/entities/notification/` — new FSD entity:
  - `api/index.ts`: thin wrapper over `api.notifications.*` (list, unreadCount, markRead, markAllRead), mirroring `entities/review/api/index.ts`.
  - Types/normalization added to `packages/api-client/src/index.ts` (`Notification`, `NotificationsListResponse`, `UnreadCountResponse`), following the existing snake_case-raw → camelCase-normalized pattern used for `Review`/`List`/etc. where applicable (the nested `actor`/`review` objects can stay close to wire shape like `Review.user` does today — no need to over-normalize).
- `apps/web/src/widgets/notifications/` — new widget:
  - `ui/notification-bell.tsx`: bell icon + badge, `useQuery(['notifications','unread-count'], ..., { refetchInterval: 30_000 })`, only rendered for authenticated users.
  - A dropdown/panel (can live in the same widget) that fetches the list on open (`useQuery` with `enabled: isOpen`), renders items, and wires click-to-mark-read-and-navigate plus "mark all as read" (`useMutation` + query invalidation of both the list and unread-count queries, same `@tanstack/react-query` patterns already used elsewhere, e.g. `features/rate-book`).
- `widgets/header/ui/header.tsx`: render `<NotificationBell />` next to the profile link, inside the existing `isAuthenticated && user` branch.

## Existing Patterns

- Polling over WebSockets/SSE for near-real-time UI: `docs/features/harvest-progress.md` (`useQuery({ refetchInterval })`), explicitly the established and re-confirmed choice in this codebase.
- Cross-module transactional writes via raw SQL inside a repository `Transaction`: `backend/internal/modules/reviews/repository/repository.go` (`upsertRatingInTx`, `recalcInTx`, and `ToggleLike`'s own likes_count update).
- Small, single-purpose module shape to copy: `backend/internal/modules/reading_challenges/*` (domain/repository/service/transport, one simple resource, `authMiddleware`-gated group in `router.go`).
- Raw-SQL join-with-flags listing style: `reviews/repository.GormRepository.ListByBook` (the `liked_by_me` EXISTS-subquery pattern is analogous to how `is_read` will be surfaced here, and shows how this codebase joins in author/actor display fields rather than a separate N+1 fetch).
- FSD entity + widget split: `entities/review` (api + ui) and `widgets/header` (HeaderSearch as a self-contained header widget) are the closest existing precedents for `entities/notification` and `widgets/notifications`.

## Tests

Backend (Go, run via `docker compose run` per the backend-is-Docker-only dev workflow):
- `notifications/service` unit tests: list/unread-count/mark-read/mark-all-read all correctly scoped to the requesting user; mark-read on someone else's notification returns not-found.
- `reviews/service` `ToggleLike` tests (extend existing `reviews/service/service_test.go`): liking another user's review creates a notification; liking your own review does not; unliking does not delete an existing notification; repeated like/unlike/like reuses (does not duplicate) the notification row.
- Repository-level test for the `ON CONFLICT` upsert behavior (integration-style, matching existing repository test conventions, e.g. `catalog/repository/repository_test.go`, `admin/repository/repository_test.go`).

Frontend:
- `notification-bell` component test (badge renders/hides correctly, mark-all-read triggers the right mutation/invalidation) — pattern per `apps/web/src/features/auth/ui/login-form.test.tsx`.

## Documentation

- Move the "Notifications for review-like activity" line from `docs/ROADMAP.md`'s "Proposed" section to "Completed", linking this contract, once shipped.

## Risks

- **30-second poll delay**: a liked-review author won't see the notification instantly; acceptable per the deliberate no-WebSockets/SSE stance already established for this codebase, and this is a lower-urgency signal than harvest job progress.
- **Unbounded table growth**: mitigated by the upsert-dedupe constraint, but there's still one row per (actor, review) pair ever liked; no pruning/archival is in scope for this increment — flag as a later cleanup if it ever matters at scale.
- **Cross-module coupling**: `reviews/repository` gains a direct write to the `notifications` table, mirroring its existing direct writes to `ratings`/`books`. This is consistent with the codebase's existing style but does mean `notifications`' schema can't change without checking `reviews/repository` call sites — acceptable tradeoff for transactional correctness over strict module isolation.
- **Hidden reviews**: if a review is hidden by moderation (not deleted), its notification is left as-is (links to a book whose review is no longer publicly listed) — minor, not addressed in this increment; only hard deletion cascades.
