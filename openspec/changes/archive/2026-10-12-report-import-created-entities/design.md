## Context

`ImportBooks` (`backend/internal/modules/admin/service/import.go`) processes each row by calling `importRow`. For every row, `importRow` resolves authors with `ResolveOrCreateAuthorIDsByNames` and genres with `ResolveOrCreateGenreIDsByNames` (both in `admin/service/harvest_resolve.go`), and only then calls `CreateBook`. Resolution therefore happens before validation and before the duplicate-slug check, so a row that ends as an error or as `skipped_existing` can still have created authors or genres. Both resolvers return only IDs today; they do not say which IDs were newly created.

## Decisions

1. **Report entities created by any row, whatever the row's outcome.** The orphans left by failed or skipped rows are exactly what the admin needs to notice. Not creating them (validate first, or resolve only after the slug check) would change behavior and is out of scope. It can be a follow-up.
2. **Entry shape `{id, name, row}`.** `id` drives the edit link, `name` is the stored name (trimmed input), and `row` is the first data row that created the entity, which lets the admin trace a near-duplicate back to the file. Status is not included because it is fixed by the rules (authors are published, genres are pending).
3. **Report-wide arrays, not per-row ones.** A per-row field would only add value on the error rows. Report-wide arrays fit the existing flat report shape.
4. **The public resolver signatures stay unchanged.** Add unexported variants that also return the created entities. The existing exported functions become thin wrappers that drop the extra return value, so their callers and tests (`harvest_resolve_test.go`, `genre_status_test.go`, `genre_moderation_test.go`) are untouched.
   - `resolveOrCreateAuthorsByNames(ctx, names) (ids []string, created []*catalogDomain.Author, err error)`
   - `resolveOrCreateGenresByNames(ctx, names) (ids []string, created []*catalogDomain.Genre, err error)`
   - A genre counts as created only on the not-found branch. An alias match is never created.
   - **The variants MUST return the entities created so far together with a non-nil error.** For example, if the first author was created and the second creation failed, the first is still reported.
5. **`importRow` collects the created entities.** `importRow` returns its created entities alongside its outcome and error. `ImportBooks` appends them with `Row: i + 1`. A second row naming the same entity matches it as existing, so no extra dedup is needed. Still, dedupe by ID inside `ImportBooks` as cheap insurance.

## API

`POST /api/v1/admin/books/import`: the route, authorization (admin role and bearer) and status codes are unchanged. The response gains two fields:

```go
type ImportCreatedEntity struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    Row  int    `json:"row"`
}
// ImportReport additions; both initialized to empty slices, never nil:
CreatedAuthors []ImportCreatedEntity `json:"created_authors"`
CreatedGenres  []ImportCreatedEntity `json:"created_genres"`
```

Regenerate Swagger with `docker compose exec backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal` (see `Makefile`). The handler (`admin/transport/http/handler.go`) needs no change. In `handler_test.go`, update `fakeAdminService.ImportBooks` to return the empty slices so that its JSON matches the contract.

## Frontend

- `packages/api-client/src/index.ts`: add `ImportCreatedEntity { id: string; name: string; row: number }` and the fields `created_authors: ImportCreatedEntity[]` and `created_genres: ImportCreatedEntity[]` on `ImportReport`.
- `apps/admin/src/widgets/book-file-import/ui/book-file-import-panel.tsx`: below the badges and the error table, render one small list section per non-empty array. Headings are «Созданные авторы (N)» and «Созданные жанры (N)». Each item is a `next/link` to `/catalog/authors/${encodeURIComponent(id)}/edit` or `/catalog/genres/…/edit` (with the same `as any` typedRoutes cast used in `views/import-authors/ui/author-create-section.tsx`), followed by «строка N». Under the genres heading, show the note «Новые жанры созданы на модерации и не видны на сайте до публикации.» Keep it inside the widget. No new feature or entity slice is needed, and this respects FSD boundaries.
- Guard against an older backend that omits the fields: `report.created_authors ?? []`.

## Existing Patterns

- `ResolveHarvestAuthorRefs` / `HarvestAuthorsResolution.CreatedWithQID` in `harvest_resolve.go` already makes "resolve and also report what was created" the precedent to follow.
- Edit links after a manual creation: `author-create-section.tsx`, `genre-create-section.tsx`.

## Risks

- Orphan authors and genres from failed or skipped rows are now visible, but they are still created. This change does not prevent them; a follow-up could validate a row before resolving names.
- A partial resolver failure must still report what was already created (Decision 4). Cover this with a test, or that entity becomes the silent case again.
- `name` is the stored name. If `CreateAuthor` or `createGenre` normalizes it (for example by trimming), report the stored entity's `Name`, not the raw input.
