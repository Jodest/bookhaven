# Tasks

## 1. Backend

- [x] 1.1 Domain: add `WorkFields` and a pure normaliser/validator covering trim, blank→nil, original title equal to title→nil, the QID uppercased, and the kind, year, IMDb, Kinopoisk, QID and length rules with the exact messages from the spec. Also add `IsValidRelation`, the new errors and `ValidationError`. Verify with table tests in `domain` covering every rule and message, plus IMDb and Kinopoisk links being rejected.
- [x] 1.2 Repository `works.go`:
  - `ListWorks` (search, unlinked, paging, counts);
  - `GetWork` (with books and authors);
  - `UpdateWork` (full replace, `manually_edited_at = NOW()`, QID add-only, 23505→`ErrQIDTaken`);
  - `DeleteWorkIfUnlinked`, an atomic single statement, then 404 vs 409;
  - `AddManualBookLink` and `AddManualAuthorLink`: one transaction, resolving by id, by QID (upsert + select) or by dup-check and insert with `manually_edited_at`, then the link insert `ON CONFLICT DO NOTHING` → `ErrAlreadyLinked`;
  - `DeleteManualBookLink` and `DeleteManualAuthorLink` (source `manual` only → `ErrLinkNotManual`);
  - `UpdateAuthorLink` (COALESCE hidden and relation).

  Books and authors tables are read only. Verify with `go build ./...` and `go vet ./...` from `backend/`.
- [x] 1.3 Service `manual.go`: list, get, update, delete work; add and delete book and author links; update the author link. Validation runs before any repository call. Verify with unit tests on the fake repository:
  - validation fails without touching the repository;
  - a QID match returns `work_created=false`;
  - a duplicate IMDb id → `ErrDuplicateExternalID` carrying the work id;
  - an already linked work → `ErrAlreadyLinked`;
  - deleting a Wikidata link → `ErrLinkNotManual`;
  - a work with links → `ErrWorkHasLinks`;
  - a QID change → `ErrQIDImmutable`;
  - an empty author-link patch → error.
- [x] 1.4 Transport `works.go` and `manual.go`:
  - the handlers and status mapping from design.md (201 with `work_created`, 204 on deletes, 409 body with `screen_work_id`);
  - the author PATCH accepts `relation` and `hidden` as pointers, and a body with neither returns «hidden or relation is required»;
  - Russian Swagger annotations.

  Verify with handler tests for each status code, and update the existing «hidden is required» test.
- [x] 1.5 Router: register the four link routes and the `/admin/screen-works` group (admin role, bearer on mutations). Verify with a router test: moderator → 403 on each new route, a mutation without bearer → rejected, `POST /admin/authors/sync-screen-works` still reaches the sync handler.
- [x] 1.6 A sync interaction test in the service or repository, whichever has a DB or fake harness. It covers:
  - a work with `manually_edited_at` set keeps its title and empty year after `SyncBook`;
  - a manual link to a QID work keeps `source=manual` and counts as not added after a sync that lists it.
- [x] 1.7 Regenerate Swagger (`backend/docs`) the usual way. Run `go build ./...`, `go vet ./...` and `go test ./internal/modules/screenworks/... ./internal/transport/...` from `backend/`. Run gofmt on changed files only, never on the whole tree. No migration and no DB down/drop commands.

## 2. API client

- [x] 2.1 `packages/api-client`:
  - add the types and methods from design.md (`admin.screenWorks.*`, `addAdaptation`, `deleteAdaptation`, `addScreenWork`, `deleteScreenWork`, `updateScreenWork`);
  - keep `setScreenWorkHidden` working;
  - delete calls handle 204 without a JSON body.

  Verify with `tsc --noEmit` for the package and existing client tests, if any.

## 3. Admin frontend

- [x] 3.1 Entity `entities/screen-work`:
  - `adminScreenWorksApi`;
  - a shared `KIND_LABELS`, replacing the copies in both widgets;
  - `validateWorkForm`, `extractImdbId`/`extractKinopoiskId` and the error-text mapping;
  - `ScreenWorkFields` UI.

  Verify with vitest for the libs: every message, link extraction for `imdb.com/title/tt…`, `kinopoisk.ru/film/<n>/` and `/series/<n>/`, and the server error mapping.
- [x] 3.2 Feature `features/link-screen-work`: the dialog («Выбрать из каталога» / «Создать новый», «Тип связи» for authors, «Уже привязан»), the mutation with invalidations, and the success and error banners from the spec. Verify with vitest:
  - picking a result posts `screen_work_id`;
  - create mode posts `work` and blocks invalid input client-side;
  - a 409 duplicate keeps the dialog open with the message;
  - `work_created=false` adds the QID note.
- [x] 3.3 Feature `features/edit-screen-work`: the update and delete mutations and `ScreenWorkEditForm` (read-only QID when set, link extraction on save). Verify with vitest for the form submit payload and error messages.
- [x] 3.4 Widgets `book-adaptations-admin` and `author-films-admin`: «Добавить экранизацию» / «Добавить фильм об авторе» (always shown), «Вручную», «Изменить» with the back link, «Удалить» on manual rows only with the confirmation, the relation switch for authors, and the error texts. Verify with widget tests: there is no «Удалить» on a Wikidata row, the add button is present without a QID, and the relation switch calls the PATCH with `{relation}`.
- [x] 3.5 Views and routes:
  - `views/screen-works` at `/catalog/screen-works`: search, «Только без связей», paging in the URL, counts, «Изменить», and the empty and error texts;
  - `views/screen-work-edit` at `/catalog/screen-works/[id]/edit`: the form, linked books and authors with marks, «Изменён вручную <date>», «Удалить фильм» only with zero links plus the hint otherwise, and back allow-list handling.

  Verify with view tests, including that a malicious `back` falls back to `/catalog/screen-works`.
- [x] 3.6 Navigation: add the tab `catalog.screen-works` «Фильмы и сериалы» (admin only) after «Жанры». Verify by updating `navigation.test.ts` (path and edit-path resolution) and `section-tabs.test.tsx`.
- [x] 3.7 Run `tsc --noEmit` and `next lint` for `apps/admin` and `pnpm --filter admin test`. Do not run a native `next build`; CI is the typedRoutes gate. `apps/web` must have no diff.

## 4. Docs

- [x] 4.1 `docs/ROADMAP.md`: mark F3 «ручное редактирование экранизаций и фильмов об авторе» done, with a link to this change. Verify that the line references `openspec/changes/add-screen-works-admin-editing`.
- [x] 4.2 Check that the Swagger output contains the new routes, with `@Tags admin` and Russian descriptions.
