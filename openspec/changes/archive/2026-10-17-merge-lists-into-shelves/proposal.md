# Proposal

## Why

Readers have two overlapping ways to group books: custom shelves and user lists. Lists add a title, a description and a privacy flag, and otherwise duplicate shelves. We merge lists into custom shelves to get one model, one UI and one public page. System collections «Подборки» (`/top`, curated) are a different thing and stay as they are.

## What Changes

- Custom shelves get an optional description (at most 2,000 characters) and a privacy flag `is_public` (default `true`). `POST /api/v1/shelves` and `PATCH /api/v1/shelves/{id}` accept `{name, description?, is_public?}`. Shelf payloads carry `description` and `is_public`.
- Default shelves («Хочу прочитать», «Читаю», «Прочитано») stay public. Trying to make one private, or to give it a description, returns 400.
- A private shelf is hidden from the public profile. `GET /api/v1/shelves/public/{id}` returns 404 for it, as for an unknown shelf, even when the owner asks. The owner still sees and manages it through `/api/v1/shelves`.
- Migration: every user list becomes a custom shelf of the same user, keeping its title (trimmed and cut to 100 characters), description, privacy and timestamps. Its books move over with their `added_at`. The `lists` and `list_items` tables are dropped. List order is **not** carried over: shelves sort by date added.
- **BREAKING** All `/api/v1/lists…` endpoints are removed. Old list ids open nothing.
- **BREAKING** `GET /api/v1/users/{id}` no longer returns `lists`. Former lists appear under `shelves`.
- Web: these go away — `/lists`, `/lists/{id}` and `/lists/public/{id}`, «Мои списки» in the header and footer, the «Публичные списки» block of the public profile, and «В список» / «Добавить в список» on the book page. No redirects from the old URLs. `/shelves` and `/shelves/{id}` gain description editing, «Сделать приватной» / «Сделать публичной» and the badge «Приватная».
- Backlog: issue [#208](https://github.com/Jodest/bookhaven/issues/208) (browse and vote on community lists) assumes user lists. It will be closed or reframed around public shelves; the orchestrator handles the issue.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `collections/shelves`: custom shelves get a description and a privacy flag; default shelves cannot be private; the shelves pages edit both.
- `collections/public-collections`: the public shelf endpoint and the profile hide private shelves; public list rules and pages are removed.
- `collections/lists`: all requirements removed (merged into shelves).
- `collections/list-books`: all requirements removed (merged into shelf books).
- `users/public-profile`: the profile payload and page have no lists.
- `auth/authorization`: the optional-auth examples no longer name lists.
- `catalog-moderation/entity-status`: book visibility no longer mentions lists.
- `catalog/book-page`: the lists control and its sign-in link are removed from the book page.

## Impact

- backend: new migration `000034`; delete module `lists` and its wiring (router, `cmd/api`); shelves domain/repository/service/handler; users public profile handler; swagger regenerated.
- frontend: `apps/web`: remove the list routes, views, `entities/list` and `features/add-to-list`, plus their links; update the shelves views, public shelf and public profile views and the book page. `packages/api-client`: drop the lists API and types and extend `Shelf`.
- devops: none. The migration runs with the normal `migrate up`. There is no production yet; it only touches the local dev DB.
- docs: `docs/ROADMAP.md`, `docs/features/lists.md` (marked superseded), `docs/PIPELINE.md` (collections domain table), `docs/PROJECT.md`.
