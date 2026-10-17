# Design

## Context

User lists (module `backend/internal/modules/lists`, tables `lists` / `list_items`, web `/lists…`) duplicate custom shelves (`shelves` with `type = 'custom'`, `shelf_items`). The user decided to merge them:
- custom shelves get `description` and `is_public`;
- list order (`list_items.position`) is dropped;
- curated «Подборки» (module `curated`, `/top`) are untouched.

There is no production; only the local dev DB holds data.

## Decisions

1. **Merge into the existing shelves module.** We add no new abstraction: the shelves domain, repository, service and handler grow two fields, and the `lists` module is deleted outright.
2. **Reuse list ids as shelf ids** in the data migration (`INSERT INTO shelves (id, …) SELECT l.id, …`). The item copy then needs no mapping table, since `list_items.list_id` is the new `shelf_items.shelf_id`. A collision between list and shelf UUIDs is practically impossible. The migration still fails loudly if one happens, because the PK conflict aborts the transaction, so data is never silently merged.
3. **Description is a non-null `TEXT` with default `''`.** The API always returns a string, empty meaning "no description". This matches what lists returned (`description: string`) and keeps the TS type simple.
4. **Default shelves reject any PATCH with 400.** Rename is already blocked; `description` / `is_public` on a default shelf is blocked too. A DB `CHECK (is_public OR type = 'custom')` backs this up.
5. **PATCH is partial.** `{name?, description?, is_public?}` uses pointer fields. If all are absent, the response is 400 `"nothing to update"`. Validation: the name is trimmed, 2–100 characters; the description is trimmed, ≤ 2000 characters.
6. **Count characters, not bytes.** The shelves service checks `len(name)`, which counts bytes, so a 51-letter Cyrillic name is rejected by the service. The validator tag `max=100` counts runes. Migrated list titles cut to 100 Cyrillic characters (`LEFT()` counts characters) would then be un-renamable. Switch the service checks to `utf8.RuneCountInString` for both name and description, as the spec says.
7. **Privacy filtering lives in the service.**
   - `GetPublicShelf` returns `ErrShelfNotFound` when `!shelf.IsPublic`. It does no owner check and makes no owner exception.
   - `ListPublicByUser` keeps shelves with `IsPublic && BooksCount > 0`; alternatively it adds `AND s.is_public` to the `ListByUserVisible` SQL, which is equivalent.
   - Owner endpoints (`GetShelf`, `AddBook`, …) are unchanged.
8. **Public profile drops `lists`.** It is removed from the response, the `ListService` dependency is removed from the users handler, and `description` is added to the shelf items.
9. **No redirects.** The `/lists…` routes are deleted, so Next serves its not-found page.
10. **Main specs at archive time.** After archiving, `openspec/specs/collections/lists/` and `openspec/specs/collections/list-books/` hold no requirements and should be deleted. The `## Purpose` lines of `collections/public-collections` and `users/public-profile` still mention lists and need a one-line edit. The orchestrator does both during archive, since deltas cannot change Purpose.

## API

| Method | Path | Auth | Change |
|---|---|---|---|
| GET | `/api/v1/shelves` | bearer/cookie, owner | items add `description` (string), `is_public` (bool) |
| POST | `/api/v1/shelves` | bearer/cookie | body `{name, description?, is_public?}`. Responses: 201 `shelfResponse` with the new fields; 400 on validation |
| GET | `/api/v1/shelves/{id}` | bearer/cookie, owner (403 others) | `shelf` adds `description`, `is_public` |
| PATCH | `/api/v1/shelves/{id}` | bearer/cookie, owner (403 others) | body `{name?, description?, is_public?}`. Responses: 400 for a default shelf, an empty body or validation errors; 200 `shelfResponse` |
| GET | `/api/v1/shelves/public/{id}` | optional | 404 `shelf not found` when private; `shelf` adds the fields |
| GET | `/api/v1/users/{id}` | optional | **BREAKING**: no `lists`. `shelves[]` adds `description` and excludes private shelves |
| * | `/api/v1/lists`, `/lists/{id}`, `/lists/{id}/books`, `/lists/{id}/books/{bookId}`, `/lists/{id}/books/reorder`, `/lists/public/{id}` | — | **removed** (Echo returns 404) |

Request structs in the handler:
- `createShelfRequest{Name string `validate:"required,min=2,max=100"`; Description *string `validate:"omitempty,max=2000"`; IsPublic *bool}`
- `updateShelfRequest{Name *string; Description *string; IsPublic *bool}`

Rename the handler method `RenameShelf` to `UpdateShelf`, and the service method to `UpdateShelf(ctx, userID, shelfID, UpdateShelfInput)`. Update the swagger annotations, then regenerate `backend/docs` (`docs.go`, `swagger.json`, `swagger.yaml`) with `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`. The Makefile target runs it via `docker compose exec backend`. The `/lists` paths and `list*` definitions must disappear from the output.

## Database

New migration pair `backend/migrations/000034_merge_lists_into_shelves.{up,down}.sql`.

**Up** (runs in one transaction):
```sql
ALTER TABLE shelves ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE shelves ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE shelves ADD CONSTRAINT shelves_default_public_chk CHECK (is_public OR type = 'custom');

INSERT INTO shelves (id, user_id, name, type, description, is_public, created_at, updated_at)
SELECT l.id, l.user_id,
       CASE WHEN char_length(n.name) >= 2 THEN n.name ELSE 'Список' END,
       'custom', COALESCE(l.description, ''), l.is_public, l.created_at, l.updated_at
FROM lists l
CROSS JOIN LATERAL (SELECT regexp_replace(LEFT(btrim(l.title), 100), '\s+$', '') AS name) n;

INSERT INTO shelf_items (shelf_id, book_id, added_at)
SELECT list_id, book_id, added_at FROM list_items
ON CONFLICT DO NOTHING;

DROP TABLE list_items;
DROP TABLE lists;
```
- The `'Список'` fallback covers only a pathological title, such as one character followed by 99 spaces.
- Descriptions were already capped at 2000 (byte-counted, therefore ≤ 2000 characters).
- Indexes `idx_list_items_list_id`, `idx_list_items_book_id` and `idx_lists_user_id` go away with the tables.
- `shelves.name` is `VARCHAR(255)`, so 100 characters fit.

**Down** recreates the structure only:
- `lists` (columns as in `000001`), `list_items` with `position INTEGER NOT NULL DEFAULT 0`, and indexes `idx_list_items_list_id`, `idx_list_items_book_id`, `idx_lists_user_id`;
- `DROP CONSTRAINT shelves_default_public_chk`; drop the columns `is_public` and `description`.

Data loss on down (document it in a header comment of the down file): former lists remain as plain custom shelves, without description or privacy, and the recreated `lists` tables are empty. Private shelves become public again. Never edit earlier migrations.

Domain `Shelf` gains `Description string `gorm:"type:text;not null;default:''"`` and `IsPublic bool `gorm:"not null;default:true"``.

GORM gotcha: with `default:true`, creating a struct whose `IsPublic` is `false` would silently store `true`, because GORM skips zero values for columns with defaults. On create, either:
- set the field explicitly via `Select("*")`, or
- use `*bool` in the domain.

Cover it with a sqlite repository test that creates a private shelf. `shelfListRow` and both list SQL queries must scan the two new columns, since they use `s.*`.

## Frontend

`packages/api-client/src/index.ts`:
- `Shelf` adds `description: string` and `isPublic: boolean`. `RawShelf`, `normalizeShelf`, `RawPublicShelf` and `normalizePublicShelf` map `description` / `is_public`.
- `PublicProfile`: remove `lists`; shelves items add `description`.
- Remove `List`, `ListDetail`, `PublicList`, `BookList` (unused), `RawList`, `RawPublicList`, `normalizeList`, `normalizePublicList` and the whole `api.lists` namespace.
- `api.shelves.create({ name, description?, is_public? })`. Replace `rename` with `update(id, { name?, description?, is_public? })` and update `entities/shelf/api` (`shelvesApi.update`) and its test.

`apps/web/src` (FSD):
- **Delete:**
  - `app/lists/page.tsx`, `app/lists/[id]/page.tsx`, `app/lists/public/[id]/page.tsx`;
  - `views/lists`, `views/list-detail`, `views/public-list-detail`;
  - `entities/list`, `features/add-to-list` (with its tests).
- `views/book-detail/ui/book-detail-view.tsx`: drop `AddToListPopover` and its test expectations; keep the action order «status → На полку → rating».
- `widgets/header/ui/header.tsx` and its test, `widgets/footer/ui/footer.tsx`: remove the `/lists` «Мои списки» links.
- `views/public-profile`: remove the «Публичные списки» section; show `shelf.description` under custom shelf names.
- `views/public-shelf-detail`: show the description; a 404 shows the existing «Полка не найдена».
- `views/shelves`:
  - the create form gets an optional `textarea` «Описание (необязательно)» (zod `max(2000, 'Максимум 2000 символов')`);
  - custom shelf rows show the badge «Приватная» when `!isPublic` and a button «Сделать приватной» / «Сделать публичной» (`shelvesApi.update(id, { is_public: !isPublic })`, error «Ошибка изменения видимости полки»).
- `views/shelf-detail`:
  - the edit form edits name + description (zod schema as in the removed `list-detail` view); the button «Переименовать» becomes «Редактировать»;
  - the description is shown under the title;
  - the badge and privacy toggle as on `/shelves`;
  - all of it only for `type === 'custom'`.
- `middleware.ts` does not reference `/lists` (lists were client-guarded), so no change is needed. Confirm with a grep.
- Grep `apps/web/src` for `/lists`, `listsApi`, `add-to-list`, `entities/list` and «Мои списки» until there are no hits. `apps/admin` has no list references.

## Existing Patterns

- Privacy toggle, description textarea, zod messages: the removed `views/list-detail/ui/list-detail-view.tsx` (copy the behavior, not the file).
- Public 404 for private entities: the removed `lists` service `GetPublicList` (it checks `IsPublic` and returns not found).
- Visible-books-aware counting: shelves repository `ListByUserVisible`.
- Shelf HTTP tests: `backend/internal/modules/shelves/transport/http/handler_test.go`; users profile tests: `backend/internal/modules/users/transport/http/handler_test.go`.

## Risks

- **Data loss is irreversible on down.** Before running `migrate up` locally, take a `pg_dump` of the dev DB. Never run `migrate down` as a test of the down file. Agents must not run any destructive DB command.
- **Name collisions.** A user may end up with two custom shelves with the same name, for example a list «Отпуск» and a shelf «Отпуск». Shelf names are not unique today, so this is accepted.
- **Empty former lists.** They become empty custom shelves and, under the shelf rule, are no longer shown on the public profile. Before, empty public lists were shown. This is accepted.
- **Breaking API.** No external clients exist; the web app and api-client change in the same PR.
- **GORM default-true gotcha** (see Database) could make every new shelf public whatever the request says. This must be covered by tests.
- **No inline shelf creation.** The book page loses the on-the-spot «Название нового списка» creation. «На полку» stays hidden for readers without custom shelves (`collections/shelf-books`), which is unchanged and out of scope.
- **Issue #208** (community lists browse & voting) contradicts the new model. The orchestrator closes or reframes it.
