# Tasks

Ground rules: no destructive DB commands (never `migrate down`, `DROP`, or a reset of the local DB); gofmt only the files you changed (never the whole tree); no native `next build` (CI is the typedRoutes gate).

## 1. Backend

- [x] 1.1 Add `backend/internal/pkg/textnorm` with `Key(s)` (lowercase, ё→е, non-letter/digit → space, collapse), plus unit tests. Make admin `normalizeGenreName` (and `normalizeAuthorName` if identical) delegate to it. Verify with `go test ./internal/pkg/textnorm/... ./internal/modules/admin/...`.
- [x] 1.2 Add the forward migration `backend/migrations/000026_books_title_words_index.{up,down}.sql` (GIN index from design.md). Apply it with the usual migrate **up** only. Verify with `\d books` showing `idx_books_title_words`.
- [x] 1.3 Add `curated/domain/paste.go`: line parsing (numbering, tab, last spaced dash or slash, quotes and trailing punctuation, 500-rune truncation), author matching (several names, initials), status classification and duplicate marking, per design Decisions 5–7. Add table-driven tests for every spec scenario of «Pasted line parsing» and «Catalog matching of pasted lines».
- [x] 1.4 Repository: add `TitleCandidates(ctx, key, limit)` (FTS `simple` query matching the index expression, `ORDER BY length(title), id`, Authors preloaded), `CollectionBookIDs(ctx, id)`, `BooksByIDs(ctx, ids)`, and `AddBooks(ctx, id, bookIDs, position)`. `AddBooks` runs in one transaction: `lock`, re-check membership, capacity all-or-nothing → `ErrCollectionFull`, shift, insert the block; it returns added positions and the ids that were already present. Add SQLite tests for `AddBooks` (append, insert at a position, clamp, dense positions, full → nothing written, already present skipped) and `BooksByIDs`.
- [x] 1.5 Service: `PreviewPaste(ctx, id, text)` (blank-line skipping with original line numbers, 500-line cap → 422 error, 100 000-char cap → validation error, one candidate query per distinct key, title-only retry, counts, `free_slots`) and `BulkAdd(ctx, id, bookIDs, position)` (1..500 ids, position ≥ 1, classify not_found / rejected / duplicate, then `AddBooks`). Unit tests with a fake repository cover every scenario of «Bulk paste preview» and «Bulk add of chosen books».
- [x] 1.6 Transport: handlers and response shapes per design.md API. Register `POST /admin/collections/:id/books/paste/preview` and `POST /admin/collections/:id/books/bulk` in the existing admin `collections` group with `requireBearer`. Add Russian swagger annotations and regenerate `backend/docs` the usual way. Handler tests cover 200 / 400 / 404 / 422. Router tests: moderator → 403 and cookie-only → 401 on both routes.
- [x] 1.7 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./...`.

## 2. Frontend

- [x] 2.1 `packages/api-client`: add the `PasteBook`, `PasteLine`, `PasteLineStatus`, `PastePreview` and `BulkAddResult` types, plus `admin.collections.previewPaste` and `admin.collections.bulkAddBooks` (bearer admin fetcher). Verify with `tsc --noEmit` for the package.
- [x] 2.2 `apps/admin/src/features/paste-collection-books`: `lib/status-labels.ts`, `lib/selection.ts` (pure selection reducer per design Decision 10), `lib/paste-error-text.ts`, `model/use-paste-books.ts` (preview and bulk mutations; bulk invalidates `COLLECTIONS_QUERY_KEY`), `ui/paste-books-panel.tsx`, and `index.ts`. Respect FSD: only entities and shared imports, no import of `edit-collection-books`. Vitest unit tests for the labels, the selection reducer (initial ticks, ambiguous pick, dedup first-wins, over-capacity, not-found copy text) and the error text.
- [x] 2.3 Panel RTL tests: a `found` row is ticked; picking an ambiguous candidate ticks its row; a `not_found` row has no checkbox and shows «Не найдено — сначала добавьте книгу в каталог» and «Добавить в каталог»; rejected and in-collection candidates cannot be picked; «Добавить выбранные (N)» is disabled when over the free places, with «Свободно мест: {free} — снимите лишние отметки»; editing the text hides the preview; the result report shows «Добавлено» and «Пропущено» with reasons; the 422 full message.
- [x] 2.4 `widgets/collection-books`: add the «Вставить список» toggle that renders the panel, and extend `collection-books.test.tsx`.
- [x] 2.5 Verify: `tsc --noEmit` and `next lint` in `apps/admin` (and `tsc --noEmit` in `packages/api-client`), and the admin Vitest suite. No native `next build`; wait for the CI frontend job.

## 3. Docs

- [x] 3.1 Mark epic #413 feature 2 progress in `docs/ROADMAP.md` if it tracks the epic's features. Swagger is regenerated in 1.6. The OpenSpec delta is archived by the orchestrator; do not edit `openspec/specs/` directly.
