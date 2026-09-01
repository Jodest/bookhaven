# Shelves (книжные полки) — план реализации

## Цель

Дать пользователю персональные книжные полки (Фаза 2 ROADMAP): дефолтные «Хочу прочитать» / «Читаю» / «Прочитано» + произвольное количество пользовательских полок. Пользователь добавляет книги на полки со страницы книги и управляет ими на странице «Мои полки». Фича — первая персонализированная после auth, строится поверх существующих модулей catalog и auth.

## User stories

1. Как пользователь, я хочу добавить книгу на полку «Хочу прочитать» / «Читаю» / «Прочитано» прямо со страницы книги, чтобы вести список чтения.
2. Как пользователь, я хочу видеть текущее положение книги на дефолтных полках при открытии страницы книги, чтобы не добавлять её повторно.
3. Как пользователь, я хочу создать собственную полку с уникальным названием (например, «Классика на зиму»), чтобы группировать книги по своему принципу.
4. Как пользователь, я хочу переименовать или удалить свою пользовательскую полку.
5. Как пользователь, я хочу просмотреть все свои полки и книги на выбранной полке.
6. Как пользователь, я хочу убрать книгу с полки (со страницы полки или со страницы книги).
7. Как анонимный посетитель, я не должен управлять полками; кнопки на странице книги ведут к входу.

## Acceptance criteria

Backend (проверяемо через API):

1. `GET /api/v1/shelves` без access-токена → 401.
2. После первого `GET /api/v1/shelves` у пользователя автоматически существуют ровно 3 дефолтные полки типов `want_to_read`, `reading`, `read` (повторный вызов не создаёт дубликатов).
3. `POST /api/v1/shelves` с `{"name": "Классика"}` → 201, полка имеет `type: "custom"`; без имени / имя < 2 символов / > 100 символов → 400.
4. Пользователь может создать несколько custom-полок с разными именами (частичный unique index).
5. `POST /api/v1/shelves/:id/books` с корректным `book_id` → 201; повторное добавление той же книги на ту же полку → 409; несуществующий `book_id` → 404; невалидный UUID → 400.
6. Добавление книги на дефолтную полку удаляет её с двух других дефолтных полок (эксклюзивность трио); custom-полки не затрагиваются; одна книга может состоять в custom-полке и одной дефолтной одновременно.
7. `DELETE /api/v1/shelves/:id/books/:bookId` → 200; книги нет на полке → 404.
8. `GET /api/v1/shelves/:id` с пагинацией `page`/`limit` возвращает `{shelf, items, total, page, limit}`; полка другого пользователя → 403; несуществующая → 404.
9. `PATCH /api/v1/shelves/:id` переименовывает только свою custom-полку; дефолтная полка → 400; чужая → 403.
10. `DELETE /api/v1/shelves/:id` удаляет только свою custom-полку (shelf_items удаляются каскадом); дефолтная → 400; чужая → 403.
11. `GET /api/v1/shelves?book_id=<uuid>` возвращает полки пользователя с полем `has_book` — true для полок, содержащих книгу.
12. Все эндпоинты (кроме перечисленных в п.1 ответов 401) отдают ownership-ошибки 403 при доступе к чужой полке.

Frontend:

13. На `/books/[slug]` для авторизованного пользователя отображаются кнопки «Хочу прочитать» / «Читаю» / «Прочитано» с активным состоянием текущей полки; клик добавляет/убирает книгу; для анонима — кнопка-ссылка на `/login`.
14. Страница `/shelves` (защищена middleware) показывает полки пользователя с количеством книг и формой создания полки.
15. Страница `/shelves/[id]` показывает книги полки с пагинацией и возможностью убрать книгу; custom-полку можно переименовать/удалить.
16. `pnpm type-check` и `pnpm lint` в apps/web проходят; `go vet ./...` и `go test ./...` в backend проходят.

## Бизнес-правила

- Дефолтные полки: `want_to_read` («Хочу прочитать»), `reading` («Читаю»), `read` («Прочитано»). Создаются лениво при первом `GET /api/v1/shelves` (в транзакции, `ON CONFLICT DO NOTHING` — идемпотентно).
- Дефолтное трио взаимоисключающее: добавление книги на одну дефолтную полку удаляет её с двух других. Custom-полки независимы.
- Типы `abandoned` и `favorite` из enum `shelf_type` в MVP не используются как отдельные полки — зарезервированы на будущее (UI создаёт только `custom`).
- Переименовать/удалить можно только custom-полку. Дефолтные — нет (400).
- Название custom-полки: 2–100 символов, непустое; уникальность имени в рамках пользователя не требуется (MVP).
- Книга добавляется только если существует и не скрыта (`is_hidden = false`).
- Полки в MVP приватные: доступ владельцу; чужая полка → 403 (публичный просмотр — Фаза 6).
- Пагинация: `page` (default 1), `limit` (default 20, max 100) — как в catalog.
- Удаление полки каскадно удаляет её items (уже задано в FK `ON DELETE CASCADE`).

## UX

1. **Страница книги** (`/books/[slug]`): блок из трёх toggle-кнопок «Хочу прочитать» / «Читаю» / «Прочитано» (активная — залита) + выпадающий список custom-полок с чекбоксами («Добавить на полку…»). Оптимистичное обновление UI, при ошибке — откат с toast/инлайн-сообщением. Для анонима — те же кнопки, но ведущие на `/login?redirect=...`.
2. **Мои полки** (`/shelves`): список карточек-полок (имя, счётчик книг), дефолтные закреплены сверху в фиксированном порядке; форма создания полки (inline input + кнопка); у custom-полок кнопки «Переименовать» / «Удалить» (удаление — с подтверждением).
3. **Полка** (`/shelves/[id]`): заголовок, сетка книг (обложка, название, авторы — как в каталоге), кнопка «Убрать с полки» на карточке, пагинация «Назад/Вперёд».
4. Состояния: загрузка («Загрузка...»), пустая полка («На этой полке пока нет книг»), ошибка — инлайн-сообщение.
5. Header: ссылка «Мои полки» для авторизованных.

## Existing implementation

- **Migration 000001** уже создаёт `shelves` (id, user_id, name, type `shelf_type`, timestamps, `UNIQUE (user_id, type)`) и `shelf_items` (PK `(shelf_id, book_id)`, `added_at`, FK CASCADE), индекс `idx_shelves_user_id`. Проблема: `UNIQUE (user_id, type)` запрещает больше одной custom-полки — нужна новая миграция (partial unique index).
- **Module shelves** (`backend/internal/modules/shelves/`) — пустая директория, кода нет.
- **Catalog** — эталонный модуль: `domain/repository/service/transport/http`, DTO-мапперы, pagination `{items, total, page, limit}`, ошибки `service.Err*NotFound` → 404, `gorm.ErrRecordNotFound` маппится в service.
- **Auth middleware** (`transport/http/middleware/auth.go`): `Auth(...)` ставит `c.Set("user_id")` / `c.Set("user")`; есть `RequireRole`. Токен — Bearer или cookie `access_token`.
- **DI и роуты**: `cmd/api/main.go` создаёт repos/services, `transport/http/router.go::RegisterRoutes` (catalog); auth-роуты регистрируются в main.go. Handler получает конкретный `*service.Service`.
- **api-client**: namespace-объект `api.*`, `fetcher` с `credentials: 'include'`, тип `Shelf` уже объявлен (типы живут в api-client, `@bookhaven/types` не интегрирован — известный tech debt, не трогаем).
- **Web**: middleware уже защищает `/shelves`; `/profile` — паттерн client-страницы на zustand (`useAuthStore`); формы — React Hook Form + Zod; `/books/[slug]` — server component; книги рендерятся сеткой карточек.
- **Тестов в backend нет**, testify нет — стандартный `testing` + fake-реализации интерфейса Repository.
- В auth-регистрации дефолтные полки не создаются (grep подтверждает) — ленивое создание в shelves service.

## Backend

Новый модуль `backend/internal/modules/shelves/` по образцу catalog:

- **domain/shelf.go**: `ShelfType` (string, константы 6 типов), `Shelf` (ID, UserID, Name, Type, CreatedAt, UpdatedAt, `Books []catalogDomain.Book` через `gorm:"many2many:shelf_items"`). Импорт `catalog/domain` для Book — аналогично тому, как auth импортирует users/domain.
- **repository/repository.go**: интерфейс `Repository` + `GormRepository`:
  - `EnsureDefaultShelves(ctx, userID)` — INSERT ... ON CONFLICT DO NOTHING для 3 типов;
  - `ListByUser(ctx, userID)` — полки + `books_count` (LEFT JOIN COUNT), порядок: дефолтные по фиксированному порядку типов, затем custom по `created_at`;
  - `GetByID(ctx, shelfID)`;
  - `Create(ctx, shelf)`, `Update(ctx, shelf)`, `Delete(ctx, shelfID)`;
  - `AddBook(ctx, shelfID, bookID)`, `RemoveBook(ctx, shelfID, bookID)`, `BookOnShelf(ctx, shelfID, bookID) (bool)`;
  - `ListBooks(ctx, shelfID, limit, offset)` — JOIN `shelf_items`→`books` (+ Preload Authors/Genres), `is_hidden = false`, ORDER BY `added_at DESC`, с total;
  - `BookExists(ctx, bookID)` — по таблице books через `catalog/domain.Book` (модульная граница не расширяется, new-зависимость только на domain-слой catalog);
  - `UserShelvesWithBook(ctx, userID, bookID)` — множество shelf_id, содержащих книгу (для флага `has_book`).
- **service/service.go**: sentinel-ошибки `ErrShelfNotFound`, `ErrForbidden`, `ErrBookNotFound`, `ErrBookAlreadyOnShelf`, `ErrDefaultShelfImmutable`, `ErrInvalidName`. Методы принимают `userID uuid.UUID` (из middleware, не из запроса):
  - `ListShelves(ctx, userID, bookID *uuid.UUID)` — EnsureDefaultShelves + список (+has_book);
  - `CreateShelf(ctx, userID, name)` — валидация имени, type=custom;
  - `GetShelf(ctx, userID, shelfID, page, limit)` — ownership check → ErrForbidden; книги с пагинацией;
  - `RenameShelf(ctx, userID, shelfID, name)`, `DeleteShelf(ctx, userID, shelfID)` — только owner, только custom;
  - `AddBook(ctx, userID, shelfID, bookID)` — ownership; проверка книги; duplicate → ErrBookAlreadyOnShelf; если полка дефолтная — удалить книгу с двух других дефолтных полок (одна транзакция);
  - `RemoveBook(ctx, userID, shelfID, bookID)`.
- **transport/http/handler.go**: DTO `shelfResponse`, `shelfListItemResponse` (+`books_count`, `has_book`), `shelfDetailResponse` (`shelf` + `items/total/page/limit`); request-структуры с `validate`-тегами; `userID` из `c.Get("user_id").(string)` → `uuid.Parse`; маппинг ошибок: 400/401/403/404/409/500 — стиль как в auth/catalog handler.
- **Роуты**: расширить `transport/http/router.go::RegisterRoutes(e, catalogService, shelvesService, authMiddleware)` (вызов из main.go не дублирует group) — все shelves-эндпоинты под `authMiddleware`. Wiring repos/services в `cmd/api/main.go`.

## API

Все под `/api/v1`, требуют authentication (Bearer или cookie):

| Метод | Путь | Тело/Query | Ответ |
|---|---|---|---|
| GET | `/shelves` | `?book_id=<uuid>` (optional) | 200 `{items: [{id, name, type, books_count, has_book?, created_at}], total}` |
| POST | `/shelves` | `{name}` | 201 `shelfResponse` / 400 |
| GET | `/shelves/:id` | `?page&limit` | 200 `{shelf, items, total, page, limit}` / 403 / 404 |
| PATCH | `/shelves/:id` | `{name}` | 200 `shelfResponse` / 400 / 403 / 404 |
| DELETE | `/shelves/:id` | — | 200 `{status:"ok"}` / 400 / 403 / 404 |
| POST | `/shelves/:id/books` | `{book_id}` | 201 `{status:"ok"}` / 400 / 404 / 409 |
| DELETE | `/shelves/:id/books/:bookId` | — | 200 `{status:"ok"}` / 404 |

Ошибки: `{"error": "..."}` — существующий стиль.

## Database

Новая миграция `000003_shelves_partial_unique.up.sql` / `.down.sql` (golang-migrate, идемпотентная):

```sql
-- up
ALTER TABLE shelves DROP CONSTRAINT shelves_user_id_type_key;
CREATE UNIQUE INDEX idx_shelves_user_id_type ON shelves(user_id, type) WHERE type <> 'custom';
CREATE INDEX idx_shelf_items_book_id ON shelf_items(book_id);
-- down
DROP INDEX idx_shelf_items_book_id;
DROP INDEX idx_shelves_user_id_type;
ALTER TABLE shelves ADD CONSTRAINT shelves_user_id_type_key UNIQUE (user_id, type);
```

Существующие миграции не изменяются. Новых таблиц нет — схема 000001 достаточна.

## Frontend

Изменения только в `apps/web` (admin не затрагивается):

- **`src/components/shelve-book-buttons.tsx`** (new, client): props `bookId`; грузит `api.shelves.list({book_id})`; 3 toggle-кнопки + dropdown custom-полок; optimistic UI; для анонима (`!isAuthenticated` из `useAuthStore`) — Link на `/login?redirect=/books/<slug>` (slug передать пропом).
- **`src/app/books/[slug]/page.tsx`** (modify): добавить блок с `<ShelveBookButtons bookId={book.id} slug={book.slug} />` под заголовком/метаданными.
- **`src/app/shelves/page.tsx`** (new): client-страница; список полок (карточки: имя, счётчик; дефолтные сверху), inline-форма создания (Zod: имя 2–100), кнопки rename/delete для custom (delete — confirm).
- **`src/app/shelves/[id]/page.tsx`** (new): client-страница; книги полки сеткой (переиспользовать разметку карточки книги как в `/books`), «Убрать с полки», пагинация page/limit.
- **`src/components/header.tsx`** (modify): ссылка «Мои полки» для авторизованных.
- Middleware `/shelves` уже защищён — не трогаем.

**api-client** (`packages/api-client/src/index.ts`): namespace `api.shelves`: `list(params?)`, `get(id, params?)`, `create({name})`, `rename(id, {name})`, `remove(id)`, `addBook(id, bookId)`, `removeBook(id, bookId)`. Обновить `Shelf`-тип: `booksCount: number`, `hasBook?: boolean`, `books?` (детали). Новых зависимостей нет.

## Security

- Все 7 endpoints за `middleware.Auth` (проверка JWT + существования пользователя).
- `userID` берётся только из контекста middleware (`c.Get("user_id")`), никогда — из пути/тела запроса.
- Ownership: service сравнивает `shelf.UserID != userID` → `ErrForbidden` → 403 (GET/PATCH/DELETE/POST books/DELETE books). Существование полки и владение проверяются до любых мутаций.
- Массовое перечисление чужих полок исключено: `ListByUser` всегда фильтрует по userID из токена.
- Валидация ввода: `validate`-теги (name 2–100, book_id required UUID); невалидные UUID → 400.
- SQL-инъекции исключены (GORM параметризованные запросы); полки приватны в MVP — 403 не раскрывает больше, чем сам факт существования UUID.

## Implementation plan

1. Миграция `000003` (+ down), проверить `make migrate` up/down.
2. `shelves/domain/shelf.go`.
3. `shelves/repository/repository.go` (+ реализация на GORM, транзакция в AddBook для эксклюзивного переноса).
4. `shelves/service/service.go` (sentinel-ошибки, бизнес-правила, ownership).
5. `shelves/transport/http/handler.go` (DTO, валидация, маппинг ошибок).
6. Wiring: `cmd/api/main.go` (repo, service), `transport/http/router.go` (роуты + authMiddleware).
7. Ручная проверка API (curl/httptest): 401 без токена, happy path, 403/404/409.
8. Тесты service (см. Testing).
9. api-client: `api.shelves.*` + типы.
10. Web: компонент кнопок на странице книги → страница `/shelves` → страница `/shelves/[id]` → ссылка в header.
11. `go vet ./...`, `go test ./...`, `pnpm type-check`, `pnpm lint`.

Порядок даёт работающий backend до начала frontend; каждый шаг компилируется независимо.

## Files

### Create

- `backend/migrations/000003_shelves_partial_unique.up.sql`
- `backend/migrations/000003_shelves_partial_unique.down.sql`
- `backend/internal/modules/shelves/domain/shelf.go`
- `backend/internal/modules/shelves/repository/repository.go`
- `backend/internal/modules/shelves/service/service.go`
- `backend/internal/modules/shelves/service/service_test.go`
- `backend/internal/modules/shelves/transport/http/handler.go`
- `apps/web/src/components/shelve-book-buttons.tsx`
- `apps/web/src/app/shelves/page.tsx`
- `apps/web/src/app/shelves/[id]/page.tsx`

### Modify

- `backend/cmd/api/main.go` — DI shelves repo/service, передача в RegisterRoutes.
- `backend/internal/transport/http/router.go` — регистрация shelves-роутов под authMiddleware.
- `packages/api-client/src/index.ts` — `api.shelves.*`, расширение типа `Shelf`.
- `apps/web/src/app/books/[slug]/page.tsx` — блок кнопок полок.
- `apps/web/src/components/header.tsx` — ссылка «Мои полки».

## Testing

`backend/internal/modules/shelves/service/service_test.go` — unit-тесты service с in-memory fake `Repository` (интерфейс небольшой, testify не добавляем — stdlib `testing`). Обязательные сценарии:

- **Успешный сценарий**: ListShelves создаёт 3 дефолтные полки однократно; CreateShelf; AddBook; RemoveBook; GetShelf с пагинацией; RenameShelf/DeleteShelf custom.
- **Invalid input**: имя < 2 / > 100 / пустое; невалидный book_id (несуществующая книга → ErrBookNotFound).
- **Unauthorized**: на уровне handler/роутов — все эндпоинты за `Auth` (проверяется ручным тестом/-review: без токена 401; unit: service не вызывается без userID).
- **Forbidden**: доступ к полке другого пользователя (GetShelf/Rename/Delete/AddBook/RemoveBook) → ErrForbidden.
- **Not found**: несуществующая полка; книги нет на полке при удалении.
- **Edge cases**: повторное добавление книги на полку → ErrBookAlreadyOnShelf; эксклюзивный перенос между дефолтными полками (книга остаётся ровно на одной); custom-полки не затрагиваются при переносе; повторный ListShelves не дублирует дефолтные полки; пустая полка (items=[], total=0); limit=0 → default 20, limit>100 → 100.

Интеграционные/E2E-тесты — вне scope (Фаза 8 ROADMAP). Frontend — type-check + lint + ручная проверка сценариев UX.

## Risks

- ** UNIQUE(user_id, type) в существующей схеме** — без миграции 000003 вторая custom-полка будет падать с 500. Миграция обязательна до релиза кода.
- **Ленивое создание дефолтных полок** — при прямом POST /shelves/:id/books без предварительного GET /shelves дефолтной полки может не существовать (404). Фронтенд всегда грузит список первым; документируем поведение.
- **Эксклюзивный перенос** требует транзакции (удаление из двух дефолтных полок + вставка) — при ошибке посреди операции без транзакции возможна потеря состояния; реализовать в одной GORM-транзакции.
- **Нет тестовой инфраструктуры backend** — service-тесты на fake repo не покрывают SQL/GORM-слой; риск ошибок в запросах ListBooks/counts. Компенсируется ручной проверкой на seed-данных.
- **`@bookhaven/types` не интегрирован** — типы полок продолжаем держать в api-client (tech debt ROADMAP, отдельная задача).
- CORS/cookies уже настроены (`credentials: 'include'`, AllowOrigins 3000/3001) — изменений не требуется.
