# Tasks

## 1. Backend: migration

- [x] 1.1 Add `backend/migrations/000034_merge_lists_into_shelves.up.sql` as in design.md → Database. It adds the `description` and `is_public` columns and the `shelves_default_public_chk` check, copies `lists` into custom `shelves` (same ids, trimmed title cut to 100 characters, description, is_public, timestamps), copies `list_items` into `shelf_items` keeping `added_at`, then drops `list_items` and `lists`. Verify by reading the SQL; do not edit earlier migrations.
- [x] 1.2 Add `000034_merge_lists_into_shelves.down.sql`. It recreates `lists` / `list_items` (with `position` and the three indexes) and drops the check and both columns. A header comment states the data loss on down. Verify by reading it; do NOT run `migrate down`.
- [x] 1.3 The developer does not apply the migration. The orchestrator or user takes a `pg_dump` of the local DB first and then runs a normal `migrate up`. Never drop the DB or run `migrate down`.

## 2. Backend: shelves

- [x] 2.1 Add `Description` (text, not null, default `''`) and `IsPublic` (default true) to the shelves domain. Repository: scan both new columns in `shelfListRow` / `ListByUser` / `ListByUserVisible`, and make sure create persists `is_public = false` (GORM zero-value-with-default gotcha). Add a sqlite repository test that creates a private shelf and reads it back as private.
- [x] 2.2 Service:
  - `CreateShelf(name, description, isPublic *bool)` and `UpdateShelf(…, input{Name, Description, IsPublic *string/*bool})` replace `RenameShelf`.
  - Validation: trimmed name 2–100 runes, trimmed description ≤ 2000 runes (`utf8.RuneCountInString`). An empty input returns a new `ErrNothingToUpdate`. Any update of a default shelf returns `ErrDefaultShelfImmutable`.
  - `GetPublicShelf` returns `ErrShelfNotFound` for a private shelf. `ListPublicByUser` skips private shelves.
  - Unit tests cover: default visibility, 2001-rune description → error, 100 Cyrillic letters → ok, `is_public:false` on «Читаю» → immutable error, private shelf → public not found even for its owner, private shelf excluded from the profile list.
- [x] 2.3 Handler:
  - `createShelfRequest` gets `description`/`is_public`; `updateShelfRequest` has pointer fields. `UpdateShelf` maps errors to 400/403/404.
  - `shelfResponse` and `shelfListItemResponse` add `description` and `is_public`.
  - Update the swagger annotations (Russian summaries; the public endpoint description says that private shelves return 404).
  - Handler tests cover create with description, PATCH of `is_public`, PATCH of a default shelf → 400, empty PATCH → 400, public GET of a private shelf → 404.
- [x] 2.4 Router: replace `RenameShelf` with `UpdateShelf` on `PATCH /shelves/:id`. Verify `go build ./...`.

## 3. Backend: remove lists

- [x] 3.1 Delete `backend/internal/modules/lists` entirely. Remove its imports, the `listsService` parameter and the `/lists` route block from `backend/internal/transport/http/router.go`, and remove `listsRepo` / `listsService` and their argument from `backend/cmd/api/main.go`.
- [x] 3.2 Users handler: drop the `ListService` dependency, `publicListActivityResponse` and the `lists` field. Add `description` to `publicShelfActivityResponse`. Update `NewHandler` callers and `handler_test.go`, which asserts that there is no `lists` key and that `description` is present.
- [x] 3.3 `rg -n "modules/lists|list_items|listsService|/lists" backend --glob '!migrations/**' --glob '!docs/**'` returns nothing. `backend/cmd/seed`, `home`, `admin` and the worker have no list references (confirmed during planning); re-check.
- [x] 3.4 Regenerate swagger (`backend/docs/docs.go`, `swagger.json`, `swagger.yaml`) with `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`. Check that no `/lists` paths or `list` definitions remain and that the shelf schemas carry `description` / `is_public`.
- [x] 3.5 From `backend/`: `go build ./...`, `go vet ./...`, `go test ./...` pass. gofmt only the changed files, never the whole tree.

## 4. Frontend: api-client

- [x] 4.1 `packages/api-client/src/index.ts`:
  - `Shelf` gets `description` and `isPublic`, and the shelf normalizers map them. `PublicProfile` loses `lists`; its shelf items gain `description`.
  - Remove `List`, `ListDetail`, `PublicList`, `BookList`, the raw list types and normalizers, and `api.lists`.
  - `api.shelves.create({name, description?, is_public?})`; `api.shelves.update(id, {name?, description?, is_public?})` replaces `rename`.
  - Verify with `tsc --noEmit` for the package.

## 5. Frontend: web

- [x] 5.1 Delete these, with their tests: `app/lists/**`, `views/lists`, `views/list-detail`, `views/public-list-detail`, `entities/list`, `features/add-to-list`.
- [x] 5.2 Book page: remove `AddToListPopover` from `views/book-detail/ui/book-detail-view.tsx` and update `book-detail-view.test.tsx` (there is no «В список» / «Добавить в список»; the order is status → «На полку» → rating).
- [x] 5.3 Header/footer: remove the «Мои списки» / `/lists` links and update `header.test.tsx`.
- [x] 5.4 `entities/shelf/api`: expose `update` instead of `rename` and update `index.test.ts`.
- [x] 5.5 `views/shelves`:
  - the create form gets the optional «Описание (необязательно)» (max 2000, «Максимум 2000 символов»);
  - custom rows show the badge «Приватная» and «Сделать приватной» / «Сделать публичной» (error «Ошибка изменения видимости полки»);
  - default rows show neither.
  - Add a component test for the toggle and the badge.
- [x] 5.6 `views/shelf-detail`, for custom shelves only:
  - «Редактировать» edits the name and description together (zod: 2–100 for the name, ≤ 2000 for the description, with the Russian messages);
  - the description is shown under the title;
  - the badge «Приватная» and the privacy toggle.
  - Add a component test for the edit and the toggle, including that the «Прочитано» shelf shows no controls.
- [x] 5.7 `views/public-shelf-detail` shows the description; `views/public-profile` drops «Публичные списки» and shows custom shelf descriptions. Update or add tests.
- [x] 5.8 `middleware.ts` needs no change (it has no `/lists` entry). Confirm by grep. `rg -n "/lists|listsApi|add-to-list|entities/list|Мои списки|PublicList" apps/web/src packages` returns nothing.
- [x] 5.9 For `apps/web`: `tsc --noEmit`, `next lint` and the unit tests pass. Do not run a native `next build`; the CI frontend job is the typedRoutes gate.

## 6. Docs

- [x] 6.1 `docs/ROADMAP.md`:
  - add a done entry «Lists merged into custom shelves (description + privacy), `/lists` removed» linking this change;
  - reword the old "Lists — viewing verified" line as superseded;
  - leave the #208 line for the orchestrator to close or reframe.
- [x] 6.2 `docs/features/lists.md`: add at the top `Status: superseded by openspec/changes/merge-lists-into-shelves`. The body stays frozen history.
- [x] 6.3 `docs/PIPELINE.md`: the `collections` domain row drops "lists, list books" (it becomes "shelves, shelf books, public collections"). `docs/PROJECT.md`: replace "User lists" with custom shelves that have a description and privacy.
