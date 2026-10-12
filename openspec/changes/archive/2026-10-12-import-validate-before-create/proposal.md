# Proposal

## Why

A book file import row that fails validation or is skipped as an existing book can still create authors (published) and genres (pending). Those entities are left in the catalog as orphans. Each row should leave the catalog untouched unless it actually creates its book.

## What Changes

- Each row is fully validated before any author or genre is created for it. This covers every single-book creation rule plus "at least one non-blank author name". The duplicate-slug check (`skipped_existing`) also runs before any creation.
- A row that ends as a row error or as `skipped_existing` creates no authors and no genres.
- The row's author and genre creation and its book creation run in one database transaction. If the book creation fails, the authors and genres created for that row are rolled back too. Examples are a concurrent duplicate slug, a database error, or an author name whose slug is unusable.
- **BREAKING** (report semantics only, the shape is unchanged): `created_authors` and `created_genres` list only entities from rows that produced a book. The scenarios "Entity created by a failed row" and "Entity created by a skipped row" keep their names, but their expected outcome is rewritten: nothing is created and nothing is listed.
- Dedup across rows is unchanged. If a new author's first row fails, a later row that succeeds creates the author and is listed as the creating row.
- The order of checks, and therefore each row's outcome (created / `skipped_existing` / which error), is unchanged. For example, a row whose slug exists but whose ISBN is invalid is still `skipped_existing`, as today. Only the side effects change.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `catalog-import/book-file-import`: a new requirement covers validation before creation and per-row atomicity. The requirement "Created authors and genres in the report" is modified so that it lists only entities from rows that produced a book.

## Impact

- backend: admin service (file import row processing, a shared pre-creation book check extracted from book creation) and admin repository (a new per-call transaction helper). The 4 test fakes of the admin repository each get a one-line implementation. No migration and no API shape change.
- frontend: none. The report shape and the UI copy are unchanged, and no text says that failed rows create entities.
- devops: none.
- docs: none beyond this change. The spec delta is merged into `openspec/specs/catalog-import/book-file-import/spec.md` at archive time.
