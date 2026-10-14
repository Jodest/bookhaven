# Tasks

Ground rules for every group:
- Never run destructive DB commands: no `migrate down`, no `DROP`, no DB reset or volume wipe. `migrate up` on the running local stack is allowed.
- Never run `gofmt -w` on the whole tree: format only the files you changed.
- Never run a native `next build`; CI is the typedRoutes gate.
- Verify with `go build ./... && go vet ./... && go test ./...` from `backend/`, and with `tsc --noEmit` and `next lint` in each touched app.

## 1. Backend: storage and module

- [x] 1.1 Add the forward-only migration `backend/migrations/000025_curated_collections.{up,down}.sql` exactly as in design.md → Database. Never edit an earlier migration. Verify with `migrate up` on the local stack.
- [x] 1.2 Extract `slugify` from `admin/service/service.go` into `backend/internal/pkg/slug` (`slug.Make`) and make the admin service call it; add a unit test (`«Топ 100 фантастики»` → `top-100-fantastiki`, `ё`, punctuation). Existing admin tests stay green.
- [x] 1.3 Create `modules/curated/domain` (`Collection`, `CollectionItem`, `KindManual`, limits `MaxSize = 1000`, `MaxBooks = 1000`, item states `ranked|reserve|hidden`, errors) and a pure function that turns ordered items + book statuses + size into `{position, place, state}`. Unit tests: the three ranking scenarios of the spec (shift up, return, reserve exhausted), size shrink, all hidden.
- [x] 1.4 Create `modules/curated/repository` (GORM): CRUD with slug uniqueness, published index with `shown_count` and up to 4 covers, published page query (design.md → API), admin detail rows with book status/authors, add/remove/move in one transaction locking the collection row. SQLite tests: dense positions after add-at-position, remove and move (both directions, clamping); published-only page with global place numbers; draft excluded from the index; cascade on collection delete.

## 2. Backend: service and API

- [x] 2.1 `modules/curated/service`: validation (title, slug pattern/derivation, size, description, source, source_url, genre exists/not rejected/not merged), `published_at` set on first publish only, add-book rules (pending/published ok, rejected 422, missing 404, duplicate 409, full 422). Service tests for every rule, including `published_at` kept on unpublish/republish.
- [x] 2.2 `modules/curated/transport/http`: public `ListPublic`, `GetPublic`; admin `List`, `Create`, `Get`, `Update`, `Delete`, `AddBook`, `RemoveBook`, `MoveBook`, with Russian Swagger annotations and the response shapes of design.md → API (author/genre visibility rules copied from `lists`). Handler tests: 404 for draft/unknown slug, page 2 place numbering, limit clamp, every 400/404/409/422 mapping, PATCH clearing `genre_id`/`source`.
- [x] 2.3 Wire routes: public `GET /api/v1/collections` and `/collections/:slug` in `RegisterRoutes`; admin group `/admin/collections` with `adminRole` and `requireBearer` on non-GET in `RegisterAdminRoutes`; construct the service in `cmd/api/main.go`. Router tests: a moderator gets 403 on list and create; a cookie-only POST/PUT/DELETE gets 401; the public routes need no token.

## 3. Frontend: shared client

- [x] 3.1 Add the types and `collectionsApi` / `adminCollectionsApi` to `packages/api-client` (design.md → Frontend). Verify with `tsc --noEmit` in both apps.

## 4. Frontend: admin

- [x] 4.1 Navigation: section «Подборки» (`collections`, admin only, `/collections`) in `shared/config/navigation.ts`; update `navigation.test.ts` and the shell tests (moderator does not see it).
- [x] 4.2 `entities/curated-collection` (keys, state labels, `reserveWarning`) with unit tests for the warning text («В подборке видно 98 из 100 книг — пополните резерв», null when full).
- [x] 4.3 `features/manage-collection` (create/update/delete + form) and `features/edit-collection-books` (add/remove/move). Hook tests for the 409 slug, 409 duplicate, 422 rejected and 422 full mappings. FSD: no cross-feature imports.
- [x] 4.4 `widgets/collection-books`: ordered table with places, «Резерв» / «Скрыта: не опубликована» markers, reserve divider, «Выше»/«Ниже», «Переместить на», «Убрать», and the «Найти книгу в каталоге» search with status badges and disabled «Добавить» («Уже в подборке», rejected). Component tests for markers, divider position, disabled states and the move call.
- [x] 4.5 `views/collections` (table with status, «{shown} из {size}», warning marker, «Новая подборка») and `views/collection-edit` (new/edit modes, genre select of published genres, delete confirmation, warning banner); pages `app/collections/page.tsx`, `app/collections/new/page.tsx`, `app/collections/[id]/page.tsx`. View tests: create redirects to the edit page, warning shown when short.

## 5. Frontend: web

- [x] 5.1 `entities/curated-collection` (hooks, `CollectionCard`, source line helper) and `views/collections` + `views/collection-detail`; pages `app/top/page.tsx`, `app/top/[slug]/page.tsx`. Tests: empty index «Подборок пока нет», ranked rows show place numbers from the API, source line linked only with a URL, unpublished author not linked, 404 shows «Подборка не найдена», pagination keeps `page` in the URL.
- [x] 5.2 Header: «Подборки» → `/top` for every visitor; update the header test.

## 6. Docs

- [x] 6.1 Add the epic #413 entry to `docs/ROADMAP.md` (Up Next) with feature 1 marked done when merged; document the new public and admin routes in the existing API doc (if any) and regenerate Swagger docs the way the repo does.
- [x] 6.2 Run `npx -y @fission-ai/openspec@1.13.2 validate add-curated-collections --strict` (`OPENSPEC_TELEMETRY=0`) and fix any errors.
