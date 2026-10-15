## Context

F3, the last part of the screen works series. F1 (`openspec/changes/archive/2026-10-15-add-book-screen-adaptations/`) and F2 (`…/2026-10-15-add-author-screen-works/`) prepared everything F3 needs in the schema:
- `screen_works.manually_edited_at` exists, and `upsertWikidataWork` already skips works where it is set (`WHERE … AND manually_edited_at IS NULL`).
- `external_source`/`external_id` may both be NULL (CHECK), so a work without a QID is allowed.
- Both link tables have `source IN ('wikidata','manual')`.
- Link inserts by the sync are `ON CONFLICT DO NOTHING`, so a manual link keeps its source, hidden flag and relation across syncs.

**No migration is needed** (000031 is not used). All new code lives in the `screenworks` module, the router, `packages/api-client` and `apps/admin`. `apps/web` is unchanged: manual links are not hidden and show up through the existing public queries.

## Decisions

1. **One new capability, `screen-works/admin-editing`.** The two existing capabilities change only where their admin UI or the author PATCH changes (MODIFIED with every scenario name kept). `catalog-import/*` is not modified: "do not touch manual works" and "links are insert-only" are already there. The cross-cutting scenarios ("manual link then sync", "re-sync after an edit") live in the new capability.
2. **Link creation takes either an existing work or a new work.** `POST /admin/books/{id}/adaptations` and `POST /admin/authors/{id}/screen-works` accept either `screen_work_id` or `work`, but not both. One endpoint per owner keeps the "add" dialog a single request and a single transaction: create the work, then link it. There is no standalone `POST /admin/screen-works`, because a work without a link has no use. Works are born from a book or an author.
3. **A QID resolves to an existing work and never overwrites it.** A `work.qid` that is stored links the existing work unchanged, and the submitted fields are ignored. Changing its data is an explicit edit on the work page. `work_created: false` tells the UI to say so. A manual work with a QID is stored with `external_source='wikidata'` and `external_id=<QID>`, so the sync finds it through the existing unique index.
4. **A soft duplicate guard on IMDb and Kinopoisk ids, on create only.** These columns have no unique index. Wikidata data may already hold two works with one IMDb id, and adding a unique index would need a data cleanup. On create without a QID match, a lookup returns 409 with the existing `screen_work_id`, so the dialog can point to it. PATCH does not check this (accepted risk, see Risks).
5. **Every manual create and every PATCH sets `manually_edited_at = NOW()`.** This covers even a created work that has a QID. Data typed by an admin is authoritative. The cost is that the sync no longer fills the empty fields of such a work, but the admin can type them. Reusing an existing work through `screen_work_id` or a QID match does not mark it, because its data was not touched.
6. **The QID can be added, never changed or cleared.** Clearing or changing the QID of a Wikidata work would make the next sync create a duplicate work and lose the hides, which belong to the old work's links. Setting a QID on a manual work is the fix for a hand-made duplicate. If another work holds the QID, the response is 409 (the unique index violation is mapped to 409, after an explicit pre-check).
7. **Only manual links are deletable, and Wikidata links are hidden.** Deleting a Wikidata link would be undone by the next sync, unhidden. Deleting a manual link to a QID work may be re-added by the sync as a `wikidata` link if Wikidata lists it. The spec says so explicitly, and hiding is the way to suppress a work for good. Deleting a link never deletes the work.
8. **A work can be deleted only with zero links, hidden ones included.** The alternative, deleting with confirmation and cascading the links, was rejected. The cascade would drop hidden Wikidata links, and the next sync would recreate the work and its links unhidden, silently undoing an admin's hide. With zero links nothing can be lost. Since Wikidata links cannot be deleted, a work with one stays forever, and that is intended. The UI explains why the button is missing. The delete is a single statement, `DELETE FROM screen_works w WHERE w.id = ? AND NOT EXISTS (book link) AND NOT EXISTS (author link)`. With 0 rows, a follow-up existence check decides between 404 and 409.
9. **The relation is editable on links of any source.** The PATCH on author links becomes partial: `hidden *bool` and `relation *string`, at least one of them. The sync never updates existing links, so the admin's relation is stable.
10. **The work list is a plain admin list, not `CatalogList`, if that widget requires status props.** Works have no status. If `widgets/catalog-list` cannot run without `statusMutation`/`statusLabels`, build a small view with `shared/ui/pagination` and URL state, the same way the catalog lists keep `search`/`page`. The developer checks the widget first and makes the status props optional only if that is a trivial, backward-compatible change.
11. **Navigation.** The new tab `catalog.screen-works` «Фильмы и сериалы» (`/catalog/screen-works`, roles `['admin']`) goes after «Жанры». The edit page is under the same prefix, so it resolves to the tab.

## API

All routes are in the admin group, with `adminRole`. Mutations add `requireBearer`. Errors are `{"error": "<msg>"}`. Swagger uses Russian annotations: `@Tags admin`.

### Work payload (create inside `work`, and PATCH)
```json
{"title": "Солярис", "original_title": null, "kind": "film", "year": 1972,
 "director": "Андрей Тарковский", "imdb_id": "tt0069293", "kinopoisk_id": "43911", "qid": "Q181069"}
```
Normalisation and validation is a pure domain function with table tests (`domain.NormalizeWorkFields` or similar), and it returns a typed validation error carrying the message:
- trim every string, and turn `""` into nil;
- the `original_title` equal to the title becomes nil;
- `qid`: trim and uppercase the leading `q`, then check `^Q[0-9]+$`;
- the rules and messages are those of the spec «Screen work data rules», and the lengths match the columns (500/500/500/20/20). The messages are exactly as in the spec.

### `POST /api/v1/admin/books/:id/adaptations` and `POST /api/v1/admin/authors/:id/screen-works`
- Body: `{"screen_work_id": "<uuid>"}` or `{"work": {…}}`. The author route also needs `"relation": "biopic"|"documentary"`.
- 201: the existing admin item (`adminAdaptationResponse` / `adminAuthorFilmResponse`) plus `"work_created": bool`.
- 400: `invalid request` (bad JSON), `invalid book id` / `invalid author id`, `invalid screen work id`, `either screen_work_id or work is required`, `invalid relation`.
- 404: `book not found` / `author not found`, `screen work not found`.
- 409: `screen work is already linked to this book` / `… to this author`; `{"error":"screen work with this imdb id already exists","screen_work_id":"…"}` and the same with `kinopoisk id`.
- 422: validation.
- Transaction:
  1. check the owner exists, with `BookStatus` / `AuthorStatus`;
  2. resolve the work: by id (404 when missing), by QID (`SELECT id … WHERE external_source='wikidata' AND external_id=?`), or by dup-check and then `INSERT` with `manually_edited_at = NOW()`, using `INSERT … ON CONFLICT (external_source, external_id) WHERE external_id IS NOT NULL DO NOTHING` followed by a select, so that two racing creates with one QID converge;
  3. `INSERT` the link with `source='manual', hidden=false` `ON CONFLICT DO NOTHING`. `RowsAffected=0` means 409 already linked, and the transaction rolls back;
  4. re-read the item via the existing admin list query filtered to this work.

### `DELETE /api/v1/admin/books/:id/adaptations/:screen_work_id` and `DELETE /api/v1/admin/authors/:id/screen-works/:screen_work_id`
- `DELETE … WHERE owner = ? AND screen_work_id = ? AND source = 'manual'`. With 0 rows, check whether the link exists: missing means 404 (`adaptation link not found` / `author film link not found`), a Wikidata link means 409 `only manual links can be deleted; hide the link instead`.
- 204 without a body.

### `PATCH /api/v1/admin/authors/:id/screen-works/:screen_work_id` (changed)
- `{"hidden": *bool, "relation": *string}`. Both nil means 400 `hidden or relation is required`. A bad relation means 400 `invalid relation`.
- A single `UPDATE author_screen_works SET hidden = COALESCE(?, hidden), relation = COALESCE(?, relation), updated_at = NOW() WHERE …`. 0 rows means 404.
- Replace `SetAuthorLinkHidden` with `UpdateAuthorLink(ctx, authorID, workID, hidden *bool, relation *string)`, or keep both, at the developer's choice. The old behaviour with only `hidden` must not change. The existing tests for `hidden is required` are updated to the new message.

### `GET /api/v1/admin/screen-works?search=&unlinked=&page=&limit=`
- `{items, total, page, limit}`. `page` defaults to 1 and `limit` to 20, with a cap of 100. Out-of-range values clamp, as the catalog lists do; follow `admin` ListGenres parsing.
- Search: `w.title ILIKE ? OR w.original_title ILIKE ? OR w.external_id = ? OR w.imdb_id = ? OR w.kinopoisk_id = ?`. The ILIKE pattern escapes `%`, `_` and `\`. The exact comparisons use the trimmed query, with the QID uppercased.
- `unlinked=true` adds `NOT EXISTS` on both link tables.
- The counts are `(SELECT COUNT(*) FROM book_screen_works WHERE screen_work_id = w.id)`, and the same for authors. This is fine at catalog scale (around 10^3 works).
- Order: `w.title, w.year NULLS LAST, w.id`.
- Item: `{id, title, original_title, kind, year, director, imdb_id, kinopoisk_id, qid, manually_edited_at, book_count, author_count}`.

### `GET /api/v1/admin/screen-works/:id`
- The item plus `books: [{id, title, status, hidden, source}]`, ordered by title, and `authors: [{id, name, status, hidden, source, relation}]`, ordered by name. These are read-only joins on `books` and `authors`, the same style as `GetBooksForSync`.
- 400 `invalid screen work id`, 404 `screen work not found`.

### `PATCH /api/v1/admin/screen-works/:id`
- Full replace of the seven fields, and an optional `qid` (Decision 6). 200 returns the detail.
- 409 `screen work with this qid already exists`, 422 `qid cannot be changed` plus the validation messages, and 404.
- `UPDATE screen_works SET …, manually_edited_at = NOW(), updated_at = NOW()`. When the QID is set, it also sets `external_source='wikidata'`. Map the unique violation (SQLSTATE 23505) to 409 as a race backstop.

### `DELETE /api/v1/admin/screen-works/:id`
- 204, 404 `screen work not found`, 409 `screen work has links` (Decision 8).

### Router (`backend/internal/transport/http/router.go`)
```go
books.POST("/:id/adaptations", h.AdminAddAdaptation, requireBearer)
books.DELETE("/:id/adaptations/:screen_work_id", h.AdminDeleteAdaptation, requireBearer)
authors.POST("/:id/screen-works", h.AdminAddAuthorFilm, requireBearer)
authors.DELETE("/:id/screen-works/:screen_work_id", h.AdminDeleteAuthorFilm, requireBearer)

works := admin.Group("/screen-works", adminRole)
works.GET("", h.AdminListWorks)
works.GET("/:id", h.AdminGetWork)
works.PATCH("/:id", h.AdminUpdateWork, requireBearer)
works.DELETE("/:id", h.AdminDeleteWork, requireBearer)
```
There is no conflict: `POST /authors/:id/screen-works` is distinct from `POST /authors/sync-screen-works`, because Echo's static segment wins. A router test proves that a moderator gets 403 and that a cookie-only mutation is rejected by `requireBearer`.

## Backend layout

- `screenworks/domain`:
  - `WorkFields` (title, original title, kind, year, director, imdb, kinopoisk, qid) and its normaliser/validator;
  - `WorkListItem`, `WorkDetail`, `WorkBookLink`, `WorkAuthorLink`;
  - the errors `ErrWorkNotFound` («screen work not found»), `ErrAlreadyLinked`, `ErrLinkNotManual`, `ErrWorkHasLinks`, `ErrQIDTaken`, `ErrQIDImmutable`, and `ErrDuplicateExternalID{Field, WorkID}`;
  - `ValidationError{Msg}`;
  - `IsValidKind`, `IsValidRelation`.
- `screenworks/repository/works.go`: `ListWorks(filter)`, `GetWork(id)`, `UpdateWork(id, fields)`, `DeleteWorkIfUnlinked(id)`, `AddManualBookLink(bookID, ref)`, `AddManualAuthorLink(authorID, ref, relation)`, `DeleteManualBookLink`, `DeleteManualAuthorLink` and `UpdateAuthorLink`. Here `ref` is the existing work id or `WorkFields`. Reuse `linkedSelect`/`linkedOrder`.
- `screenworks/service/manual.go`: validation before the repository, and the 1:1 error mapping. Unit tests use the fake repository from `service_test.go`.
- `screenworks/transport/http/works.go` and `manual.go`: the handlers and Swagger annotations. Then regenerate `backend/docs` the usual way.

## Frontend

- `packages/api-client`:
  - types `ScreenWorkInput`, `AdminScreenWork` (list item), `AdminScreenWorkDetail`, `AdminScreenWorkBookLink`, `AdminScreenWorkAuthorLink`, and `AddScreenWorkLinkBody` (`{screen_work_id} | {work}`, plus `relation` for authors);
  - `admin.screenWorks.{list, get, update, delete}`;
  - `admin.books.addAdaptation(bookId, body)` and `admin.books.deleteAdaptation(bookId, workId)`;
  - `admin.authors.addScreenWork(authorId, body)`, `admin.authors.deleteScreenWork(authorId, workId)` and `admin.authors.updateScreenWork(authorId, workId, {hidden?, relation?})`. Keep `setScreenWorkHidden` as a thin wrapper, so existing callers do not change.
  - The 201 responses carry `work_created`.
- `apps/admin`:
  - New entity `entities/screen-work`:
    - `api` (`adminScreenWorksApi`);
    - `lib/kind-labels.ts`, the single `KIND_LABELS`, which removes the copies in both widgets;
    - `lib/work-form.ts`, a pure `validateWorkForm` with the Russian messages, plus `extractImdbId` / `extractKinopoiskId`, which accept a bare id or an `imdb.com/title/tt…` / `kinopoisk.ru/film|series/<n>` link;
    - `lib/error-text.ts`, which maps server errors to Russian;
    - `ui/screen-work-fields.tsx`, the controlled fields with no mutation, shared by both features.
  - New feature `features/link-screen-work`:
    - the `LinkScreenWorkDialog` with props `target: {type: 'book' | 'author'; id; linkedIds}`, and the two modes «Выбрать из каталога» (debounced search via the entity API, at most 10 results) and «Создать новый» (`ScreenWorkFields`), plus «Тип связи» for authors;
    - the `useLinkScreenWork` mutation, which invalidates the owner's list query (`['admin','books',id,'adaptations']` / `['admin','authors',id,'screen-works']`) and `['admin','screen-works']`.
  - New feature `features/edit-screen-work`: the `useUpdateScreenWork` and `useDeleteScreenWork` mutations and the `ScreenWorkEditForm`, which uses the entity fields.
  - Widgets `book-adaptations-admin` and `author-films-admin`:
    - the «Добавить…» button, which opens the feature dialog;
    - the «Вручную» badge;
    - «Изменить», a link to `/catalog/screen-works/<id>/edit?back=<encoded owner edit path>`;
    - «Удалить» on manual rows, with a `confirm` and the entity API delete;
    - for authors, a relation `<select>` calling `updateScreenWork`;
    - the strings from the specs.
  - Views and routes:
    - `views/screen-works`, the list page, at `app/catalog/screen-works/page.tsx`;
    - `views/screen-work-edit`, the edit page, at `app/catalog/screen-works/[id]/edit/page.tsx`.

    The back URL is validated against an allow-list of prefixes (`/catalog/screen-works`, `/catalog/books/`, `/catalog/authors/`), the same way `views/genre-edit` does (`ALLOWED_BACK`), and it falls back to `/catalog/screen-works`.
  - `shared/config/navigation.ts`: the new tab (Decision 11). Update `navigation.test.ts` and `section-tabs.test.tsx`.
  - FSD: widgets compose features and entities, and views compose widgets, features and entities. There are no cross-feature imports and no view imports a view. `typedRoutes`: cast hrefs as the existing code does (`as any`/`Route`), since CI is the gate.

## Existing Patterns

- Admin links and hide: `screenworks/transport/http/handler.go` (`AdminSetAdaptationHidden`), `author.go`, and `repository/repository.go` (`upsertWikidataWork`, `linkedSelect`, `linkedOrder`).
- Service unit tests with a fake repository: `screenworks/service/service_test.go` and `author_test.go`.
- Full-replace PATCH, 422/409 mapping and the edit page with back handling: the admin genre editing (`views/genre-edit`, its `ALLOWED_BACK`) and book editing.
- List page and URL state: `views/genres` (`widgets/catalog-list`), `shared/ui/pagination.tsx`.
- Widgets to extend: `apps/admin/src/widgets/book-adaptations-admin`, `widgets/author-films-admin`.
- Confirm-then-delete in the admin: `collections` (`curatedHandler.AdminDelete`, its admin view).

## Risks

- **Duplicate works without a QID.** The IMDb/Kinopoisk guard is only on create, and works with neither id can still be duplicated. Mitigation: the dialog searches the catalog first, and adding a QID on the edit page lets the sync reuse the work. There is no merge of works in F3 (out of scope).
- **A deleted manual link comes back as a Wikidata link** when the work has a QID that Wikidata lists for the owner. This is documented in the spec, and hiding is the permanent way.
- **Wikidata works become permanent.** A work with a hidden Wikidata link can never be deleted. This is intended (Decision 8), because hides must survive.
- **A sync fill-empty race with an admin edit.** The sync's `UPDATE … WHERE manually_edited_at IS NULL` and the admin's `UPDATE … SET manually_edited_at = NOW()` are each single statements. At worst the sync fills a field just before the admin's full replace overwrites it, and the admin's values win.
- **The PATCH contract change on author links.** The 400 message changes from `hidden is required` to `hidden or relation is required`. The only client is the admin app, in this repository. Update the tests and the api-client together.
- **`widgets/catalog-list` coupling to status.** See Decision 10. Do not change its behaviour for the books, authors and genres lists.
