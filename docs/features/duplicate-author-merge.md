# Admin Tool to Merge Duplicate Authors

Status: implemented

## Goal

Give admins a safe, dedicated way to consolidate a duplicate author record (a name-spelling variant created either by manual entry or by the harvest auto-create path, `ResolveOrCreateAuthorIDsByNames`) into one canonical author, reassigning all of that author's books and removing the duplicate — without the current only mitigation (manual review of harvest summaries + ad hoc editing) and without introducing new fuzzy/similarity detection logic.

Source: backlog issue #117, flagged by `docs/ai/features/catalog-data-import/business-analysis.md` risk R-002.

## User Stories

- As an admin, I spot two author records that are really the same person (e.g. "Лев Толстой" / "Л. Н. Толстой"). I pick one as "duplicate" and one as "canonical" and merge them, so every book ends up attributed to a single, consistent author record and the duplicate disappears.

## Acceptance Criteria

- [x] Admin can pick any two existing authors (via the existing authors search/list) and merge the "duplicate" into the "canonical".
- [x] After merge, every book previously linked to the duplicate author is linked to the canonical author.
- [x] If a book was already linked to both authors before the merge, afterward there is exactly one `book_authors` row for that book+canonical pair (no unique-constraint violation, no duplicate row).
- [x] After merge, the duplicate author row no longer exists and no `book_authors` row references it.
- [x] Merging an author into itself (`canonical_id == duplicate id`) is rejected (400), no changes made.
- [x] Merging when either author ID does not exist returns 404, no changes made.
- [x] Only `RoleAdmin` can call the merge endpoint — same auth as other admin mutations (401 unauthenticated, 403 other roles).
- [x] The merge is atomic: reassignment + delete happen together in one DB transaction, or neither happens.
- [x] Admin UI exposes the merge action from the existing Authors view, with an explicit confirmation step naming both authors and stating the action is irreversible.

## Business Rules

- **No field reconciliation.** The canonical author's own fields (`name`, `slug`, `biography`, `photo_url`, `birth_date`, `death_date`) are never modified by a merge. If the canonical author is missing data the duplicate had, the admin edits the canonical author manually via the existing edit form, before or after merging. Rationale: a "prefer non-null" / "prefer longer biography" reconciliation policy adds real ambiguity and edge cases for a rare, admin-supervised cleanup action — disproportionate for MVP.
- **No automated duplicate-detection UI (explicit scope call).** The backlog item's title says "detect and merge," but this contract scopes it down to **merge only**. The original catalog-data-import business analysis already evaluated and explicitly rejected fuzzy name matching for the harvest auto-create path itself (Decision 3.b: "избыточно для MVP... риск ложных склеек хуже, чем риск дублей") — the same argument applies here: a similarity algorithm risks false-positive suggestions (two distinct authors with genuinely similar names) and is disproportionate engineering effort for a low-frequency, human-supervised cleanup action. Detection stays manual — the admin finds likely duplicates by browsing/searching the existing Authors list (already supports search-by-name) and by reading harvest job reports (which already list newly auto-created authors per run — R-002's existing, documented mitigation). This feature adds only the merge action; it does not add a "suspiciously similar names" list/report. **Update:** the author moderation queue ([author-moderation-queue.md](author-moderation-queue.md), Business Rule 5) now shows display-only similar-name hints (same normalized surname, no fuzzy matching) next to each queued author, with a "merge into this one" shortcut that opens this feature's dialog prefilled. This supersedes the "no detection UI" call for the queue only; merging still always requires an explicit admin action and nothing is merged automatically.
- **External identity carry-over (added by [author-status.md](author-status.md), rule 11).** The one exception to "no field reconciliation": if the canonical author has no Wikidata QID (`external_source`/`external_id`) and the duplicate has one, the QID moves to the canonical in the same transaction (after the duplicate row is deleted, so the partial unique index is never violated). If both have one, the canonical keeps its own and the duplicate's is dropped (logged). Canonical `status` is never changed. Without this, the next harvest would not find the merged-away QID and would recreate the author as `pending`.
- **Irreversible.** There is no "unmerge." The UI must get an explicit confirmation naming both authors before calling the API.
- **Dedupe on overlap.** A book linked to both authors before the merge must end up linked exactly once to the canonical author afterward — a direct consequence of `book_authors`' composite primary key `(book_id, author_id)` (see Database).

## UX

- Entry point: existing Authors admin view (`apps/admin/src/views/authors/ui/authors-view.tsx`) — add a "Merge" action per row (alongside the existing "Редактировать").
- Clicking "Merge" on a row opens a dialog with:
  - the clicked author pre-filled as "duplicate (will be deleted)" — read-only in the dialog;
  - a searchable author select to pick the "canonical (keeps this one)" target, reusing the same authors list/search query already powering this view and `book-form.tsx`'s author multi-select.
- Confirm button is disabled until a canonical author is picked and is different from the duplicate; its label makes the destructive/irreversible nature explicit (e.g. "Слить и удалить «<duplicate name>»").
- On success: success message/toast including how many books were reassigned; duplicate row disappears from the list (query invalidation); canonical row is otherwise unchanged.
- On error (400 same-author, 404 one of the two no longer exists — e.g. deleted/merged concurrently): inline error in the dialog, no optimistic removal.

## API

- New endpoint: `POST /api/v1/admin/authors/:id/merge` — body `{ "canonical_id": "<uuid>" }`. `:id` (path) is the duplicate/source author to be deleted; `canonical_id` (body) is the target that survives.
- Auth: same route-group middleware as the existing `/admin/authors` mutations — `adminRole` (`RequireRole(RoleAdmin)`) + `requireBearer`, registered next to `authors.PATCH("/:id", ...)` in `backend/internal/transport/http/router.go`.
- Response `200`: `{ "author": <authorResponse of canonical, unchanged fields>, "books_reassigned": <int> }` — lets the UI show "N books reassigned" without a second request. Reuses the existing `authorResponse`/`toAuthorResponse` shape.
- Errors:
  - `400` — `canonical_id` missing/malformed, or equal to `:id` (new `ErrCannotMergeIntoSelf`).
  - `404` — either author does not exist (reuses existing `ErrAuthorNotFound`).
  - `401`/`403` — standard auth middleware (unauthenticated / non-admin).
- New service method: `Service.MergeAuthors(ctx, duplicateID, canonicalID uuid.UUID) (*catalogDomain.Author, int64, error)`, in a new `backend/internal/modules/admin/service/merge_author.go` (one-file-per-action, matching `bulk_publish.go`'s convention rather than growing `service.go`), delegating the transactional reassignment to a new repository method.

## Database

- **No schema/migration changes.** Reuses the existing `book_authors` composite primary key `(book_id, author_id)` (`backend/migrations/000001_initial.up.sql`) as the mechanism that makes "no duplicate row after merge" enforceable.
- New repository method `GormRepository.MergeAuthors(ctx, duplicateID, canonicalID uuid.UUID) (booksReassigned int64, err error)`, implemented as a single GORM transaction (style matching `replaceBookAuthors`'s existing raw-SQL-inside-`tx.Transaction` pattern):
  1. Reassign rows the canonical author doesn't already have:
     `UPDATE book_authors SET author_id = :canonical WHERE author_id = :duplicate AND book_id NOT IN (SELECT book_id FROM book_authors WHERE author_id = :canonical)`.
  2. Delete the remaining duplicate rows (books already linked to both — now a redundant duplicate-author row):
     `DELETE FROM book_authors WHERE author_id = :duplicate`.
  3. Delete the duplicate author row:
     `DELETE FROM authors WHERE id = :duplicate`.
  - `booksReassigned` is the row count affected by step 1 (used for the API's `books_reassigned`).
  - Step 3's existing `ON DELETE CASCADE` on `book_authors.author_id` is not relied upon for correctness (step 2 already leaves nothing to cascade) — the explicit order avoids depending on cascade timing.

## Frontend

- `apps/admin/src/views/authors/ui/authors-view.tsx` — add "Merge" button per row, dialog open/close state, mutation calling the new API client method, query invalidation on success.
- New feature module matching `manage-author`'s FSD placement: `apps/admin/src/features/merge-author/ui/merge-author-dialog.tsx`, consuming `adminAuthorsApi.list` for the canonical-author search/select (same query already used by `authors-view.tsx` and `book-form.tsx`'s author multi-select).
- `packages/api-client/src/index.ts`: add a response type (e.g. `AdminAuthorMergeResult { author: AdminAuthor; books_reassigned: number }`) and `adminAuthorsApi.merge(duplicateId: string, canonicalId: string)`, following the exact `bulkPublish`/`update` fetcher pattern already in the file.

## Existing Patterns

- `backend/internal/modules/admin/service/bulk_publish.go` — standalone-file-per-action service convention already established in this module; `MergeAuthors` follows it instead of growing `service.go`.
- `backend/internal/modules/admin/repository/repository.go`'s `replaceBookAuthors` — existing precedent for a raw-SQL `book_authors` rewrite inside a GORM transaction; `MergeAuthors`' repository method follows the same style, adding the `NOT IN` de-dupe guard that `replaceBookAuthors` never needed (it always fully replaces one book's author set, so it never hits the "book already linked to both" case this feature must handle).
- `backend/internal/transport/http/router.go`'s `authors` route group (`adminRole` + `requireBearer` on mutations) — the exact middleware the new `/merge` route reuses.
- `packages/api-client/src/index.ts`'s `bulkPublish`/`update` fetchers — reused verbatim as the shape for the new merge client call.
- `apps/admin/src/views/authors/ui/authors-view.tsx` + `apps/admin/src/features/manage-author` — existing list/search/edit scaffolding the merge dialog slots into, rather than a new authors screen.
- `backend/internal/modules/admin/service/harvest_resolve.go`'s `ResolveOrCreateAuthorIDsByNames` — the exact-normalized-name-match auto-create path that is this feature's root cause (R-002); unchanged by this feature, referenced here only as context for *why* duplicates occur.

## Tests

- Backend, service-level (`merge_author_test.go`): merge with no overlapping books (simple reassignment, `books_reassigned` count correct); merge with one or more overlapping books (dedupe path — assert exactly one `book_authors` row survives per overlapping book); merge into self (error, no changes); either ID missing (error, no changes); duplicate author row gone after merge; canonical author's own fields unchanged after merge.
- Backend, handler-level (mirroring existing author tests in `handler_test.go`): `POST /admin/authors/:id/merge` — 200 success shape incl. `books_reassigned`; 400 self-merge; 404 missing author (either side); 401/403 auth.
- Frontend: interaction test for the merge dialog — duplicate pre-filled/read-only, confirm disabled until a distinct canonical author is picked, mutation called with correct duplicate/canonical IDs, success path shows the reassigned-books count and removes the duplicate from the list.

## Documentation

- This contract: `docs/features/duplicate-author-merge.md`.
- `docs/ROADMAP.md`: move `#117` from "Up Next" to "Completed" with a link, once implemented (existing convention for every other shipped roadmap item).
- Optional, not blocking: `docs/ai/features/catalog-data-import/business-analysis.md` R-002's mitigation line currently says only "существующий UI редактирования авторов для последующей ручной консолидации" — could be updated to point at this feature's merge action as that consolidation mechanism, closing the loop this backlog item opened.

## Risks

- `book_authors` lookups by `author_id` alone (`WHERE author_id = ?`) do not hit the table's composite primary key efficiently, since `book_id` is the leading column — these become sequential scans. Acceptable at current catalog scale (admin-triggered, low-frequency, dozens/hundreds of books per author); would need a supporting index on `(author_id)` if the catalog grows by orders of magnitude. Not a blocker for MVP.
- No undo: a wrong merge (wrong canonical choice, or merging two authors that were not actually duplicates) is only recoverable by manually recreating the author and re-attributing books by hand. Mitigated by the required confirmation step in the UI naming both authors before the irreversible action runs.
- Explicit scope call (see Business Rules): no fuzzy-matching detection is built, matching the original harvest feature's own rejection of fuzzy matching (Decision 3.b). If product later decides manual detection is too burdensome, that is a separate, consciously deferred feature request, not an oversight of this contract.
