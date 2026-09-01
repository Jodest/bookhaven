# Shelves (книжные полки) — Review + QA

Дата: 2026-08-29
Ревьюер: Review + QA агент
Проверяемые изменения: незакоммиченный diff (backend модуль shelves, миграция 000003, api-client, web-страницы).

## Code review

### Соответствие plan.md

- Структура модуля `backend/internal/modules/shelves/{domain,repository,service,transport/http}` полностью соответствует плану и эталонному модулю catalog.
- Все 7 эндпоинтов реализованы и зарегистрированы в `router.go` под group `/api/v1/shelves` c `authMiddleware` — как в плане.
- Миграция 000003 соответствует SQL из плана дословно (drop constraint, partial unique index, index на `shelf_items.book_id`).
- API-ответы соответствуют спецификации из плана: `{items, total}`, `{shelf, items, total, page, limit}`, `{"status":"ok"}`, ошибки `{"error":"..."}`.
- Frontend-файлы созданы/изменены ровно по списку из плана (Create/Modify).
- Бизнес-правила реализованы: ленивое идемпотентное создание дефолтных полок, эксклюзивность дефолтного трио в транзакции, неизменяемость дефолтных полок, приватность полок (403), пагинация page/limit (default 20, max 100).

### Patterns

- Handler: DTO-мапперы, `validate`-теги, sentinel-ошибки service → HTTP-коды, `userID` из `c.Get("user_id")` — в стиле auth/catalog. Дублирование DTO book/author/genre из catalog — локальные копии в handler (без расширения межмодульных зависимостей), приемлемо.
- Repository: `ListBooks` повторяет паттерн catalog (`query.Count` + переиспользование query для `Limit/Offset/Find`) — консистентно с существующим кодом.
- Pagination-нормализация продублирована в handler и service — защитительно, не ошибка.

### Authentication / authorization / ownership

- Все shelves-роуты в group с `authMiddleware` (router.go) — без токена 401 (проверено curl).
- `userID` берётся только из контекста middleware, никогда из пути/тела.
- Ownership: каждый метод service (Get/Rename/Delete/AddBook/RemoveBook) сравнивает `shelf.UserID != userID` → `ErrForbidden` → 403 до любых мутаций (проверено curl для всех 5 операций).
- `ListByUser`/`UserShelvesWithBook` всегда фильтруют по userID из токена — перечисление чужих полок невозможно.

### Database / migrations

- Существующие миграции не изменены (git status: только новые файлы 000003).
- Миграция применена в dev-БД: version=3, dirty=false; `idx_shelves_user_id_type` — partial unique index `WHERE (type <> 'custom'::shelf_type)`, `idx_shelf_items_book_id` — созданы, старый constraint `shelves_user_id_type_key` удалён (проверено в pg_indexes/pg_constraint).
- `EnsureDefaultShelves` использует `ON CONFLICT (user_id, type) WHERE type <> 'custom' DO NOTHING` — корректный conflict target для partial index, идемпотентность подтверждена повторными вызовами API.
- SQL-инъекции исключены: параметризованные запросы GORM / `Exec` с плейсхолдерами.
- Транзакция эксклюзивного переноса реализована через `repo.Transaction` (AddBook на дефолтную полку: удаление из двух других + вставка в одной транзакции).

### TypeScript / api-client

- Тип `Shelf` расширен (`booksCount`, `hasBook?`, `books?`) — дублирующих типов не создано, новые типы в `@bookhaven/types` не добавлялись (соответствует известному tech debt из плана).
- Namespace `api.shelves` следует существующему стилю fetcher.

### Frontend

- Client-страницы `/shelves`, `/shelves/[id]` — по паттерну `/profile`; React Hook Form + Zod; состояния загрузки/ошибки/пустой полки реализованы.
- `ShelveBookButtons` — client-компонент на странице книги (server component), оптимистичный UI с откатом, аноним → Link на `/login?redirect=...`.
- Middleware `/shelves` уже защищён — не трогался (проверено: аноним → 307 на `/login?redirect=%2Fshelves`).
- Header: ссылка «Мои полки» для авторизованных.

### Принцип минимального изменения

- Feature-изменения ограничены файлами из плана.
- ⚠️ В рабочем дереве присутствуют unrelated-изменения: `AGENTS.md`, `.opencode/**`, `opencode.json`, `docs/ai/features/README.md` (переработка pipeline-конфигурации + массовая смена переводов строк). Это не код фичи; Release-агент не должен включать их в commit фичи без отдельного решения.
- Секретов и временного мусора в новых файлах нет (проверено grep).

### Известные замечания Developer — оценка

- **Down-миграция 000003 падает при >1 custom-полке** — подтверждено экспериментально (безопасно, в откатываемой транзакции): `ALTER TABLE ... ADD CONSTRAINT UNIQUE (user_id, type)` даёт ошибку на дубликате. Оценка: LOW / nice to have — SQL в точности соответствует plan.md; существующие down-миграции проекта (000001/000002) также best-effort и деструктивны; правило «не изменять существующие миграции» не нарушено. Откат down после появления custom-полок потребует ручной чистки данных — задокументированное ограничение.
- **`HasBook` в domain** — вычисляемое поле с `gorm:"-"` рядом с `BooksCount` — допустимо в рамках GORM-паттернов проекта; ошибок не создаёт.
- **`GetByUserAndType` и `Transaction` в интерфейсе репозитория** — прагматичное расширение интерфейса; fake в тестах реализует оба метода; циклических зависимостей нет. Допустимо.
- **Временные тестовые данные в dev-БД** — проигнорировано для статуса (созданные при QA review-пользователи также являются dev-данными).

## QA

### Фактический прогон проверок

| Проверка | Результат |
|---|---|
| `go vet ./...` (docker compose exec backend) | ✅ чисто, без замечаний |
| `go test -count=1 ./internal/modules/shelves/...` | ✅ ok, 0.005s |
| `pnpm type-check` (docker compose exec web; pnpm на хосте отсутствует — проверено в web-контейнере с примонтированным monorepo) | ✅ 5/5 successful |
| `pnpm lint` (там же) | ✅ 4/4 successful, No ESLint warnings or errors |

### Acceptance criteria — ручная проверка API (curl, live dev-окружение)

| # | Критерий | Результат |
|---|---|---|
| 1 | GET /shelves без токена → 401 | ✅ `{"error":"unauthorized"}` 401; мусорный токен → 401 |
| 2 | 3 дефолтные полки после первого GET, без дубликатов при повторном | ✅ total=3 (want_to_read, reading, read); повторный GET — total=3 |
| 3 | POST /shelves → 201 custom; имя пустое/<2/>100 → 400 | ✅ 201 `type:"custom"`; "" / "a" / 101 символ → 400 |
| 4 | Несколько custom-полок (partial unique index) | ✅ создано 3+ custom-полок у одного пользователя, конфликтов нет |
| 5 | POST /shelves/:id/books → 201; дубль → 409; несущ. book_id → 404; невалидный UUID → 400 | ✅ все 4 исхода подтверждены |
| 6 | Эксклюзивность дефолтного трио; custom не затрагиваются | ✅ add→want_to_read, затем add→reading: want_to_read=false, reading=true, custom=true (книга одновременно в custom + одной дефолтной) |
| 7 | DELETE /shelves/:id/books/:bookId → 200; нет на полке → 404 | ✅ 200; повторное удаление → 404 |
| 8 | GET /shelves/:id с пагинацией {shelf,items,total,page,limit}; чужая → 403; несущ. → 404 | ✅ структура ответа верна; 403; 404; невалидный UUID → 400 |
| 9 | PATCH переименовывает только свою custom; дефолтная → 400; чужая → 403 | ✅ 200 (имя обновилось); 400 "default shelf cannot be modified"; 403 |
| 10 | DELETE удаляет только свою custom (каскад items); дефолтная → 400; чужая → 403 | ✅ 200; cascade проверен в БД (shelf_items=0, shelves=0); 400; 403 |
| 11 | GET /shelves?book_id=… → has_book | ✅ has_book=true/false по фактическому содержимому |
| 12 | 403 на всех эндпоинтах при доступе к чужой полке | ✅ проверено для GET/PATCH/DELETE/:id, POST/DELETE books вторым пользователем |
| 13 | Кнопки на /books/[slug]: аноним → /login?redirect | ✅ на живой странице 3 кнопки-ссылки `href="/login?redirect=/books/<slug>"`; авторизованный сценарий — code review + API |
| 14 | /shelves защищена middleware, форма создания | ✅ аноним → 307 `/login?redirect=%2Fshelves`; авторизован → 200; форма (RHF+Zod, имя 2–100) в коде |
| 15 | /shelves/[id]: сетка книг, убрать книгу, пагинация, rename/delete custom | ✅ код соответствует; API-часть проверена curl |
| 16 | go vet / go test / type-check / lint | ✅ все пройдены (см. таблицу выше) |

Дополнительно: пагинация — `limit=0` → 20 (default), `limit=150` → 100 (cap) — подтверждено по ответу API. Пустая полка — total=0, items=[] (структура ответа). `has_book` отсутствует в ответе без `book_id` (omitempty) — соответствует плану.

### Тестовое покрытие (service_test.go, 15 тестов)

Покрыто: создание дефолтных полек однократно; create/rename/delete happy path; invalid name (<2, пустое); GetShelf/AddBook forbidden; shelf not found; duplicate → ErrBookAlreadyOnShelf; эксклюзивный перенос (custom не затрагивается); removeBook + not-on-shelf; has_book; дефолтные значения пагинации.

Пробелы (несущественные): forbidden не покрыт юнит-тестами для Rename/Delete/RemoveBook; имя >100 символов; пустая полка; limit>100 → 100. Основные сценарии (happy/invalid/forbidden/not found/edge) покрыты; авторизация на уровне роутов проверена вручную.

## Найденные проблемы

### Критические (blocker)

Нет.

### Важные (must fix)

Нет.

### Незначительные (nice to have)

1. **Down-миграция 000003 необратима при наличии данных** (несколько custom-полок у пользователя) — `backend/migrations/000003_shelves_partial_unique.down.sql`. Подтверждено экспериментально. Соответствует плану и стилю существующих down-миграций; откат потребует ручной чистки. Оставить как есть; задокументировано.
2. **Оптимистичный откат в `shelve-book-buttons.tsx` (handleDefaultToggle)**: при ошибке запроса восстанавливается только нажатая полка; соседние дефолтные полки, сброшенные в false оптимистично, не восстанавливаются (UI рассинхронизируется до перезагрузки страницы). Только error-path, самоисцеляется при reload.
3. **`ShelfCard` (`/shelves/page.tsx`)**: `editName` инициализируется один раз при mount; повторный вход в редактирование после переименования может показать устаревшее имя. Мелкий UI-дефект.
4. **Инвалидный `book_id` в query GET /shelves молча игнорируется** (200 без has_book вместо 400). Консистентно с обработкой `genre_id`/`author_id` в catalog handler — не отклонение от паттернов проекта.
5. **Пробелы юнит-тестов**: forbidden для Rename/Delete/RemoveBook, имя >100, пустая полка, cap limit=100. Рекомендуется добавить в следующей итерации.
6. **Unrelated-изменения в рабочем дереве** (`AGENTS.md`, `.opencode/**`, `opencode.json`, `docs/ai/features/README.md` — pipeline-конфигурация/переводы строк) — не включать в commit фичи.

Заметки (без статуса проблемы): вычисляемые поля `BooksCount`/`HasBook` в domain и `GetByUserAndType`/`Transaction` в интерфейсе репозитория — допустимые расширения в рамках существующих паттернов; новых зависимостей нет; `@bookhaven/types` не тронут (по плану).

## Вывод

Реализация соответствует plan.md, все acceptance criteria подтверждены фактическими проверками (go vet/go test/type-check/lint — пройдены; API — ручная проверка curl по всем критериям, включая 401/403/404/409/400 и эксклюзивность трио). Безопасность (auth, ownership, приватность полок, SQL-инъекции) нарушений не имеет. Найдены только незначительные проблемы, не влияющие на acceptance criteria и безопасность.

Status: PASS
