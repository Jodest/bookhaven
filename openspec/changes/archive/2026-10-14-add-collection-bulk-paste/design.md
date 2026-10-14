## Context

Epic #413, feature 2. Feature 1 (archived as `2026-10-14-add-curated-collections`) built the `curated` module (`backend/internal/modules/curated`), the admin routes under `/admin/collections` (admin role, `requireBearer` on non-GET), and the admin edit page (`views/collection-edit` → `widgets/collection-books`, features `manage-collection` / `edit-collection-books`). Single add is `POST /admin/collections/:id/books`, and its repository transaction locks the collection row (`lock(tx, id)`). This change adds paste → preview → bulk add. Books are never created from a collection: a book missing from the catalog has to be added there first.

## Decisions

1. **Two endpoints, a stateless preview.** The preview takes the raw text and returns the report; nothing is stored server-side. Apply sends only the chosen `book_ids` in pasted order, and the server re-validates everything. The client maps `book_id` → line for the result report. There is no preview token or session state.
2. **Parsing and matching are pure Go in `curated/domain`** (`paste.go`): `ParseLine(raw) (title, author string, ok bool)`, `MatchAuthor(pasted string, authorNames []string) bool`, and `Classify(...)`. The DB only supplies title candidates. All rules from the spec are unit-tested here, table-driven.
3. **Shared normalizer.** Extract admin's `normalizeGenreName` (lowercase, ё→е, non-letter/digit → space, collapse) into `backend/internal/pkg/textnorm` (`textnorm.Key(s string) string`). `admin/service` `normalizeGenreName` and `normalizeAuthorName` delegate to it, if their bodies are identical; check before changing. Existing admin tests must stay green. `curated` imports only `pkg/textnorm`, never `admin`.
4. **Candidate retrieval = word index + exact compare in Go.** A new GIN index `to_tsvector('simple', replace(lower(title), 'ё', 'е'))` finds books whose title contains all words of the normalized key. Query: `plainto_tsquery('simple', key)` with the `textnorm.Key` of the pasted title, `ORDER BY length(title), id LIMIT 50`, with Authors preloaded. Go then keeps only `textnorm.Key(book.Title) == key`. Ordering by length puts exact titles first, so a short common title («Мать») is not crowded out by longer titles that contain it. The existing `russian` FTS index includes the description and stems words, so it would bring noise and is not reused. One query per distinct key; at most 500 lines, plus the title-only retry.
5. **Line parsing order.** (a) trim; (b) tab split: 3 or more cells with a numeric first cell → [num, title, author]; 2 cells → [title, author]; (c) otherwise strip the numbering regex `^\s*(№\s*)?\d{1,4}\s*[.)]\s*`, then split at the **last** ` — `/` – `/` - `/` / `; (d) trim quotes `«»"„“”'` and trailing `.,;:` from both parts; (e) empty title → `invalid`. If the split title has zero title matches (rejected ones included), retry with the whole post-numbering line as the title and no author.
6. **Author rule.** Split the pasted author on `,`, `;`, `&` and the word ` и `. A pasted name matches a book author when each of its `textnorm.Key` words equals a word of the author key, or is one letter equal to such a word's first letter. Book matches when any pasted name matches any of its authors. Plural surname forms («Стругацкие») will not match «Стругацкий». The line then becomes `ambiguous` with the title candidates, and the admin picks one: a deliberate safe fallback, not fuzzy matching.
7. **Status algorithm per line.** T = exact-title books. N = T narrowed by author (or T when no author). Rejected books are kept out of counting. Then, in order: no T → retry title-only (Decision 5), else `not_found`. N empty but non-rejected T exists → `ambiguous` (candidates = T). Only rejected in N∪T → `rejected`. Then on the non-rejected set R: if every item is in the collection → `in_collection`; |R| = 1 → `found`; otherwise `ambiguous`. Candidates are capped at 10 and ordered published, pending, rejected, then title and year. Duplicates: after classification, an entry whose resolved `book.id` (found / in_collection), or whose `textnorm.Key` of the raw line text, equals an earlier entry's becomes `duplicate` with `duplicate_of` = that earlier `line`. The frontend handles picks made in ambiguous rows (Decision 10).
8. **Bulk apply is all-or-nothing on capacity and per-id on validity.** In the service, load the books by ids and classify each as not_found, rejected, duplicate-in-request or addable. In one repository transaction: `lock`, read the collection's current book ids (concurrent adds become `already_in_collection`), count k, and return `ErrCollectionFull` if `len(addable) > MaxBooks - k`. Then shift `position >= pos` by `m`, and insert the `m` rows with positions `pos..pos+m-1`. `pos = clamp(position, 1, k+1)`, default `k+1`. This is the same pattern as `AddBook`, with no unique index on position.
9. **Limits.** At most 500 non-blank lines per paste (consistent with the book file import's 500 rows; a Top-1000 is two pastes). Text at most 100 000 characters (400). `book_ids` 1..500 (400). Lines over 500 characters are truncated to 500 runes before parsing (the `text` echo stays truncated too).
10. **Client-side resolution.** The preview is held in the paste feature's local state. Picking a candidate on an ambiguous row sets that row's chosen book. Ticked rows are deduplicated by book id, the first in pasted order winning (a later row is unticked and labelled «Повтор строки N»). Apply sends the ticked rows' book ids in line order.
11. **Placement.** The panel is a feature UI rendered by `widgets/collection-books`, behind a «Вставить список» toggle. A widget may import features; the new feature imports only entities/shared, never `edit-collection-books`.

## API

Both routes sit in the existing `collections` admin group (`adminRole`) with `requireBearer`.

| Method | Path | Body | Success | Errors |
|---|---|---|---|---|
| POST | `/admin/collections/:id/books/paste/preview` | `{text}` | 200 `PastePreview` | 400 `text: …` / bad uuid, 404 `collection not found`, 422 `too many lines (max 500)` |
| POST | `/admin/collections/:id/books/bulk` | `{book_ids: uuid[], position?: int}` | 200 `BulkAddResult` | 400 `book_ids: …` / `invalid position`, 404, 422 `collection is full` |

- `PastePreview = {books_count, free_slots, counts: {found, ambiguous, not_found, rejected, in_collection, duplicate, invalid}, lines: PasteLine[]}`
- `PasteLine = {line, text, title, author|null, status, book: PasteBook|null, candidates: PasteBook[], duplicate_of: int|null}`. `book` is set for `found` and for a single-match `in_collection`. `candidates` is `[]` unless the status is `ambiguous` or `rejected`.
- `PasteBook = {id, title, slug, status, cover_url|null, published_year|null, authors: [{id, name}], in_collection}`
- `BulkAddResult = {added: [{book_id, position}], skipped: [{book_id, reason: "not_found"|"rejected"|"already_in_collection"|"duplicate"}], books_count}`
- Register the static path `/books/paste/preview` and `/books/bulk` so they cannot collide with `/books/:bookId` (DELETE only, and the methods differ, but keep them explicit). Swagger annotations in Russian, like the existing handlers; regenerate `backend/docs` the usual way.

## Database

Forward-only migration `backend/migrations/000026_books_title_words_index.{up,down}.sql`:

```sql
-- up
CREATE INDEX IF NOT EXISTS idx_books_title_words
    ON books USING GIN (to_tsvector('simple', replace(lower(title), 'ё', 'е')));
-- down
DROP INDEX IF EXISTS idx_books_title_words;
```

All functions are immutable with an explicit regconfig, so the index can be used. The query expression must match the index expression byte-for-byte. The down file exists by convention and is never run by agents. No table changes.

## Frontend

- `packages/api-client`: types `PasteBook`, `PasteLine`, `PasteLineStatus`, `PastePreview`, `BulkAddResult`; `api.admin.collections.previewPaste(id, text)` and `bulkAddBooks(id, bookIds, position?)` via the bearer-sending admin fetcher.
- `apps/admin/src/features/paste-collection-books/`:
  - `lib/status-labels.ts`: status → Russian label, `duplicate` → «Повтор строки {n}», book status → «Опубликована»/«На модерации»/«Отклонена».
  - `lib/selection.ts`: pure reducer. It builds the initial selection (found ticked), handles pick and toggle, dedups by book id, and derives the ordered `bookIds`, `selectedCount`, `overCapacity`, and the not-found lines text for copying.
  - `lib/paste-error-text.ts`: 422 full → «Не хватает мест в подборке (максимум 1000 книг)»; 422 lines → «Слишком много строк — не больше 500»; others generic.
  - `model/use-paste-books.ts`: `usePastePreview(id)` and `useBulkAdd(id)` mutations. Bulk add invalidates `COLLECTIONS_QUERY_KEY`.
  - `ui/paste-books-panel.tsx`: textarea, «Найти в каталоге», counts, table, candidate radio list, position choice («В конец» / «На позицию»), «Добавить выбранные (N)», capacity note, result report, «Скопировать ненайденные» (`navigator.clipboard.writeText`). «Добавить в каталог» links to `/import/books/manual` with `target="_blank"`.
- `widgets/collection-books`: the «Вставить список» toggle renders `<PasteBooksPanel collectionId booksCount />`.
- Tests: Vitest for `selection.ts`, labels and error text; an RTL test of the panel with a mocked api (found ticked, ambiguous pick, not_found without a checkbox, over-capacity disabled, text edit hides the preview, result report); widget test for the toggle.

## Existing Patterns

- Collection lock and dense positions: `curated/repository/repository.go` `AddBook` / `lock` / `clamp`.
- Normalizer to extract: `admin/service/genre_moderation.go` `normalizeGenreName`; slug extraction precedent `internal/pkg/slug`.
- Per-row report and the 500-row cap: `catalog-import/book-file-import` (`admin/service/import.go`, `transport/http/import_parse.go`).
- Admin error mapping and mutations: `features/edit-collection-books` (`book-error-text.ts`, `use-collection-books.ts`).
- Router auth tests (403 moderator, 401 cookie-only): the existing curated router tests and `router_ai_descriptions_test.go`.

## Risks

- **FTS query is Postgres-only.** The SQLite repository tests cannot run it. Cover the matching rules in domain and service tests with a fake repository. Make the candidate method small, and when SQLite is detected (as other Postgres-only search paths do), use a `LIKE` fallback on `replace(lower(title),'ё','е')`, or skip that test. Bulk insert stays SQLite-tested.
- **`simple` parser quirks.** Hyphenated words index as the compound plus its parts, and the Go key splits on the hyphen. The query uses the parts, so it still matches. Very short or stop-like titles are fine with `simple`, which has no stop words.
- **Candidate cap (50 per key).** A title contained in more than 50 longer titles is still safe, because of the length ordering.
- **Author heuristics.** Plural or declined surnames and Latin transliterations do not match. These lines fall back to `ambiguous` (one click), never to a wrong `found`.
- **Payload size.** 500 lines × up to 10 candidates with authors is acceptable for an admin-only call.
- **typedRoutes.** The external `/import/books/manual` link must be typed; the CI build is the gate (no native `next build`).
