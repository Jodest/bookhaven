# Рейтинги и рецензии (ratings-reviews)

Фаза 3 ROADMAP. Оценки книг (1–5 звёзд), рецензии с лайками, агрегированный рейтинг книги.

## Goal

Дать пользователям возможность оценивать книги и писать рецензии, а посетителям — видеть агрегированный рейтинг и отзывы на странице книги. Это ядро социального каталога (аналог Goodreads).

## User stories

- Как читатель, я хочу поставить книге оценку 1–5 звёзд и изменить её позже.
- Как читатель, я хочу написать рецензию к книге (оценка + текст), отредактировать или удалить её.
- Как посетитель, я хочу видеть средний рейтинг книги, количество оценок и список рецензий с авторами и лайками.
- Как читатель, я хочу лайкнуть чужую рецензию и снять лайк повторным нажатием.

## Acceptance criteria

1. Аноним видит на странице книги средний рейтинг (звёзды + число оценок) и список рецензий с пагинацией; клик по звёздам/кнопке рецензии ведёт на `/login?redirect=/books/<slug>`.
2. `POST /api/v1/books/:id/ratings` ставит/обновляет оценку; `rating` и `ratings_count` книги пересчитываются корректно (повторная оценка — update, счётчик не растёт).
3. `GET /api/v1/books/:id/reviews` возвращает пагинированный список только нескрытых рецензий с автором (`id`, `name`, `avatar_url`), `likes_count` и `liked_by_me` (для авторизованного), плюс `my_review` — свою рецензию независимо от страницы.
4. `POST /api/v1/books/:id/reviews` создаёт рецензию; повторное создание той же книгой → 409; рейтинг рецензии синхронизирует оценку пользователя и участвует в среднем рейтинге книги.
5. `PATCH/DELETE /api/v1/reviews/:id` работают только для своей рецензии: чужая → 403, несуществующая → 404. Удаление рецензии не удаляет оценку пользователя.
6. `POST /api/v1/reviews/:id/likes` — toggle: первый вызов ставит лайк (`liked: true`, счётчик +1), повторный снимает.
7. Все защищённые endpoints без токена → 401.
8. `go test ./...`, `go vet ./...`, `pnpm type-check`, `pnpm lint` проходят.
9. Новые миграции БД не требуются.

## Business rules

- Оценка/рейтинг рецензии — целое 1–5 (DB CHECK).
- Одна оценка на пользователя и книгу (UNIQUE) → upsert; одна рецензия на пользователя и книгу (UNIQUE) → 409 при повторе.
- Рецензия всегда содержит рейтинг (`reviews.rating NOT NULL`). Создание/изменение рейтинга рецензии upsert-ит оценку пользователя в `ratings`. Удаление рецензии оценку сохраняет.
- Владелец рецензии — только автор (ownership, как ErrForbidden в shelves). Override для admin/moderator — вне скоупа (Фаза 7).
- `is_hidden` рецензии и книги исключаются из выдачи и лайков (`books.is_hidden` → 404 по всем nested-эндпоинтам).
- Агрегаты книги пересчитываются транзакционно из таблиц-источников (idempotent, без дрейфа): `rating = ROUND(AVG(ratings.value), 2)`, `ratings_count = COUNT(ratings)`, `reviews_count = COUNT(reviews WHERE NOT is_hidden)`.
- Лайк — toggle; лайк своей рецензии разрешён; двойной лайк невозможен (composite PK `review_likes`).
- Контент рецензии: trim, 1–10 000 символов.

## UX

Страница книги (`/books/[slug]`):

- Блок рейтинга: звёзды (среднее по книге, округление до 0.5), число с одним знаком, «N оценок»; для авторизованного — интерактивные звёзды своей оценки (optimistic update, POST/PATCH-семантика upsert); для анонима — ссылки на логин.
- Секция «Рецензии»: список карточек (автор, дата, звёзды, текст, кнопка лайка с счётчиком), пагинация «Показать ещё». Empty: «Пока нет рецензий — станьте первым». Loading: skeleton/spinner. Error: баннер с текстом ошибки (паттерн shelves-страниц).
- Форма рецензии (только для авторизованных): рейтинг-звёзды + textarea, zod-валидация; если `my_review` есть — режим редактирования (prefill) + кнопка удаления с confirm. После успеха — перезагрузка списка. 409 обрабатывается показом режима редактирования.

## Existing implementation

- Эталон backend-модуля: `backend/internal/modules/shelves` — domain/repository/service/transport/http, sentinel-ошибки в service, маппинг в handler, `getUserID(c)` из `c.Get("user_id")`, unit-тесты service на fake-репозитории.
- Схема БД уже есть (`000001_initial.up.sql`): `ratings` (UNIQUE `book_id,user_id`; `value SMALLINT CHECK 1–5`), `reviews` (UNIQUE `book_id,user_id`; `rating NOT NULL CHECK 1–5`, `content`, `likes_count`, `is_hidden`), `review_likes` (PK `review_id,user_id`); `books.rating DECIMAL(3,2)`, `ratings_count`, `reviews_count`.
- Auth: `middleware.Auth` / `middleware.OptionalAuth` готовы (`backend/internal/transport/http/middleware/auth.go`), кладут `user_id` в контекст.
- Wiring: `backend/cmd/api/main.go` (repos → services → `httptransport.RegisterRoutes`), маршруты — `backend/internal/transport/http/router.go`.
- Frontend: страница книги — server component + клиентские компоненты (`shelve-book-buttons.tsx` — паттерн `useAuthStore` и redirect на login); формы — RHF + zod; данные через `@bookhaven/api-client`.
- `backend/internal/modules/{ratings,reviews}/domain` пусты — модули создаются с нуля.

## Backend

Два модуля по образцу shelves (без взаимных зависимостей сервисов):

**ratings**
- `domain/rating.go`: `Rating{ID, BookID, UserID, Value int, CreatedAt, UpdatedAt}`.
- `repository/repository.go`: `UpsertAndRecalc(ctx, bookID, userID, value) (created bool, err)` — транзакция: upsert в `ratings` (`ON CONFLICT (book_id,user_id) DO UPDATE`) + UPDATE агрегатов книги; `GetByUserAndBook`, `BookExists` (не hidden).
- `service/service.go`: ошибки `ErrInvalidValue`, `ErrBookNotFound`, `ErrRatingNotFound`; `SetRating(ctx, userID, bookID, value)` → `{value, rating, ratings_count, created}`; `GetMyRating(ctx, userID, bookID)` → value / `ErrRatingNotFound`.
- `transport/http/handler.go`: `SetRating`, `GetMyRating`.

**reviews**
- `domain/review.go`: `Review{ID, BookID, UserID, Rating int, Content, LikesCount, IsHidden, timestamps, Author *usersDomain.User gorm:"-", LikedByMe bool gorm:"-"}`; `ReviewLike{ReviewID, UserID, CreatedAt}`.
- `repository/repository.go`: `ListByBook` (JOIN `users`, `liked_by_me` через EXISTS при наличии userID, `NOT is_hidden`, ORDER BY `created_at DESC`, limit/offset, COUNT total); `GetByID`; `GetByUserAndBook`; `Create` (транзакция: insert review + upsert rating + recalc агрегатов); `Update` (update полей + upsert rating при изменении + recalc); `Delete` (delete + recalc, лайки каскадом); `ToggleLike` (insert/delete + `likes_count = COUNT(review_likes)`); `BookExists`.
- `service/service.go`: ошибки `ErrBookNotFound`, `ErrReviewNotFound`, `ErrForbidden`, `ErrAlreadyReviewed`, `ErrInvalidRating`, `ErrInvalidContent`, `ErrNothingToUpdate`; методы `ListReviews` (пагинация как в shelves: page≥1, limit 20/≤100; возвращает items, total, my_review), `CreateReview`, `UpdateReview` (ownership + хотя бы одно поле), `DeleteReview` (ownership), `ToggleLike`. Дубль рецензии — pre-check через `GetByUserAndBook` (UNIQUE в БД — backstop).
- `transport/http/handler.go`: `ListReviews`, `CreateReview`, `UpdateReview`, `DeleteReview`, `ToggleLike`; ошибки → `{"error": "..."}` как в shelves.

**Wiring**: в `router.go` расширить `RegisterRoutes` (добавить `ratingsService`, `reviewsService`, `optionalAuth`); маршруты: GET reviews + optionalAuth, остальные POST/PATCH/DELETE + authMiddleware. В `cmd/api/main.go` создать repos/services и передать их. Echo v4.12 допускает сосуществование `/books/:slug` и `/books/:id/reviews` (имена параметров хранятся per-route) — проверено по исходнику роутера.

## API

| Method | Endpoint | Auth | Request | Response |
|---|---|---|---|---|
| POST | `/api/v1/books/:id/ratings` | да | `{value: 1..5}` | 201 (создана) / 200 (обновлена): `{book_id, value, rating, ratings_count}` |
| GET | `/api/v1/books/:id/ratings/me` | да | — | 200 `{value}` / 404 |
| GET | `/api/v1/books/:id/reviews?page&limit` | optional | — | 200 `{items, total, page, limit, my_review?}` |
| POST | `/api/v1/books/:id/reviews` | да | `{rating: 1..5, content: 1..10000}` | 201 review / 400 / 404 / 409 |
| PATCH | `/api/v1/reviews/:id` | да | `{rating?, content?}` (≥1 поле) | 200 review / 400 / 403 / 404 |
| DELETE | `/api/v1/reviews/:id` | да | — | 200 `{status: "ok"}` / 403 / 404 |
| POST | `/api/v1/reviews/:id/likes` | да | — | 200 `{likes_count, liked}` / 404 |

Review item: `{id, book_id, user_id, user: {id, name, avatar_url}, rating, content, likes_count, liked_by_me (только для авторизованного), created_at, updated_at}`. Общие ошибки: 400 невалидный UUID/тело, 401, 404 (книга/рецензия не найдена или скрыта), 403, 409. Тело ошибок: `{"error": "..."}`.

## Database

Изменений нет. Таблицы `ratings`, `reviews`, `review_likes` и колонки `books.rating/ratings_count/reviews_count` уже существуют (миграция `000001_initial`). Новая миграция не нужна.

## Frontend

**api-client** (`packages/api-client/src/index.ts`):
- `api.ratings`: `set(bookId, value)` → POST; `my(bookId)` → GET (404 → null через catch).
- `api.reviews`: заменить заглушку `list` на пагинированную версию `(bookId, {page, limit})` → `{items, total, page, limit, my_review?}`; добавить `create`, `update`, `remove`, `toggleLike`.
- Тип `Review` привести к реальным полям backend JSON (snake_case: `likes_count`, `liked_by_me`, `created_at`, `user: {id, name, avatar_url}`); новые типы `ReviewsListResponse`, `MyRating`. Не рефакторить существующие типы Book/Shelf (вне скоупа).

**apps/web**:
- `apps/web/src/app/books/[slug]/page.tsx`: заменить текстовый «Рейтинг: …» на `<BookRating bookId slug rating ratingsCount />`; добавить `<ReviewsSection bookId slug />` под блоком книги.
- `components/book-rating.tsx` (client): звёзды среднего + свои звёзды (optimistic), `useAuthStore`, аноним → `/login?redirect=...`.
- `components/reviews-section.tsx` (client): загрузка списка (page/limit, «Показать ещё»), карточки с лайк-кнопкой (toggle), empty/error/loading; для владельца — кнопки «Редактировать»/«Удалить».
- `components/review-form.tsx` (client): RHF + zod (rating 1–5, content 1–10000), create/edit режимы по `my_review`.

State: локальный `useState` в компонентах (как shelve-book-buttons); новый глобальный store не нужен.

## Files

### Existing files (изучить/изменить)

- `backend/cmd/api/main.go` — wiring ratings/reviews.
- `backend/internal/transport/http/router.go` — новые маршруты и параметры RegisterRoutes.
- `backend/internal/transport/http/middleware/auth.go` — только использовать (Auth/OptionalAuth).
- `backend/internal/modules/shelves/**` — эталонный паттерн (не менять).
- `backend/migrations/000001_initial.up.sql` — только читать (схема).
- `packages/api-client/src/index.ts` — типы и методы ratings/reviews.
- `apps/web/src/app/books/[slug]/page.tsx` — интеграция компонентов.

### New files

- `backend/internal/modules/ratings/domain/rating.go`
- `backend/internal/modules/ratings/repository/repository.go`
- `backend/internal/modules/ratings/service/service.go`, `service/service_test.go`
- `backend/internal/modules/ratings/transport/http/handler.go`
- `backend/internal/modules/reviews/domain/review.go`
- `backend/internal/modules/reviews/repository/repository.go`
- `backend/internal/modules/reviews/service/service.go`, `service/service_test.go`
- `backend/internal/modules/reviews/transport/http/handler.go`
- `apps/web/src/components/book-rating.tsx`
- `apps/web/src/components/reviews-section.tsx`
- `apps/web/src/components/review-form.tsx`

## Testing

Automated:
- Unit-тесты обоих сервисов на fake-репозитории (паттерн `shelves/service/service_test.go`): ratings — create/update/value вне 1–5/книга не найдена; reviews — create/duplicate 409/границы content (пусто, 10 001)/невалидный rating/update свой и чужой (403)/PATCH без полей/delete чужой (403)/toggle like (liked + счётчик туда-обратно).
- `go test ./...`, `go vet ./...`; frontend: `pnpm type-check`, `pnpm lint`.

QA-сценарии: анонимный просмотр; постановка и изменение оценки (среднее и счётчик меняются верно); create → edit → delete своей рецензии (оценка после удаления остаётся); вторая рецензия → 409 → режим редактирования; лайк/анлайк; редактирование чужой рецензии → 403; скрытая книга → 404; empty state списка.

## Risks

- **Пересечение модулей по таблицам**: reviews-репозиторий пишет в `ratings` и пересчитывает агрегаты книги. Это осознанное решение ради транзакционной целостности (альтернатива — вызов ratings-service без общей транзакции). Пересчёт из таблиц-источников делает операции идемпотентными и самовосстанавливаемыми.
- **Несоответствие имён полей в api-client**: существующие TS-типы (Book/Shelf) — camelCase, а backend отдаёт snake_case (латентный баг проекта). Для новых типов reviews/ratings использовать фактические имена backend; существующие типы не трогать (отдельная задача техдолга).
- **Route params**: `/books/:slug` (catalog) и `/books/:id/...` (новые) — разные имена параметров на одной позиции; для Echo v4.12 это безопасно, но при апгрейде Echo проверить тестом.
- **Rounding**: `rating DECIMAL(3,2)` вмещает среднее 1–5; округлять до 2 знаков в SQL, до 1 знака — на frontend.

После реализации (этап Documentation): отметить пункты Фазы 3 в `docs/ROADMAP.md` как выполненные, включая пункт 3.4 Web.
