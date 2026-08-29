# BookHaven — спецификация и роадмап

Документ описывает текущее состояние проекта, уже реализованные части и план дальнейшей разработки. Написан так, чтобы по нему можно было быстро понять, что сделано и что делать дальше, даже если смотришь на проект впервые.

---

## 1. Описание проекта

**BookHaven** — социальный каталог книг для рынка РФ и СНГ (аналог Goodreads).

Ключевые возможности MVP:
- Каталог книг с авторами и жанрами.
- Регистрация и аутентификация пользователей.
- Книжные полки (хочу прочитать, читаю, прочитал, брошено, избранное, кастомные).
- Рецензии и оценки книг.
- Пользовательские подборки (списки книг).
- Профили пользователей.
- Админ-панель для управления контентом.

---

## 2. Стек технологий

| Слой | Технологии |
|------|-----------|
| Backend | Go 1.23, Echo, GORM, PostgreSQL 16, Redis 7 |
| Frontend | Next.js 14 (App Router), React 18, TypeScript, Tailwind CSS, shadcn/ui |
| Monorepo | Turborepo + pnpm workspaces |
| Инфраструктура | Docker, Docker Compose |
| Миграции | SQL-файлы + `golang-migrate`-совместимая схема |

### Структура монорепозитория

```
bookhaven/
├── apps/
│   ├── web/               # Публичный frontend (Next.js)
│   └── admin/             # Панель администратора (Next.js)
├── backend/               # Go-backend
│   ├── cmd/               # Точки входа: api, migrate, seed
│   ├── internal/modules/  # Модули предметной области
│   └── migrations/        # SQL-миграции
├── packages/
│   ├── api-client/        # HTTP-клиент к backend
│   ├── config/            # Общие tsconfig
│   ├── types/             # Общие TypeScript-типы
│   └── ui/                # Общие UI-компоненты (shadcn/ui)
└── infrastructure/docker/ # Dockerfiles
```

---

## 3. Что уже сделано

### 3.1 Инфраструктура и DevEx

- [x] Docker Compose для всех сервисов: PostgreSQL, Redis, backend, web, admin.
- [x] Makefile с базовыми командами (`up`, `down`, `migrate`, `logs`, `shell-*`, `lint-*`).
- [x] Dev-окружение: hot-reload backend через Air, Next.js dev-сервер для frontend.
- [x] Миграции БД и сидеры тестовых данных.
- [x] Настроены `pnpm type-check`, `pnpm lint`, `go vet ./...`.

> **Известная проблема WSL2:** Air в backend-контейнере не подхватывает изменения `.go`-файлов с Windows FS. После правки Go нужно выполнять `docker compose restart backend`.

### 3.2 База данных

Миграции уже создают полную схему:
- `users`, `refresh_tokens`
- `books`, `authors`, `genres`
- связи `book_authors`, `book_genres`
- `ratings`, `reviews`, `review_likes`
- `shelves`, `shelf_items`
- `lists`, `list_items`
- GIN-индексы для полнотекстового поиска по книгам и авторам.

### 3.3 Backend-модули

#### ✅ Auth (реализован полностью)
- Регистрация (`POST /api/v1/auth/register`)
- Вход (`POST /api/v1/auth/login`)
- Обновление access-токена (`POST /api/v1/auth/refresh`)
- Выход с отзывом refresh-токена (`POST /api/v1/auth/logout`)
- Получение текущего пользователя (`GET /api/v1/auth/me`)
- JWT access + refresh tokens, bcrypt, auth middleware.

#### ✅ Catalog (реализован полностью)
- `GET /api/v1/books` — список книг с пагинацией, поиском по title/ISBN, фильтрами по жанру/автору.
- `GET /api/v1/books/:slug` — детальная страница книги.
- `GET /api/v1/authors/:slug` — автор с его книгами.
- `GET /api/v1/genres` — список жанров.
- Репозиторий, сервис и HTTP-handler внутри модуля.

#### ⚠️ Users (частично)
- Есть доменная модель `User` и репозиторий.
- Используется в auth.
- Отдельный CRUD профиля не реализован.

#### ✅ Shelves (реализован, MVP)
- `GET /api/v1/shelves` — полки текущего пользователя; параметр `?book_id=` добавляет флаг `has_book`.
- `POST /api/v1/shelves` — создание custom-полки; `PATCH /api/v1/shelves/:id` — переименование; `DELETE /api/v1/shelves/:id` — удаление (только custom-полки).
- `GET /api/v1/shelves/:id` — книги полки с пагинацией (`page`/`limit`).
- `POST /api/v1/shelves/:id/books`, `DELETE /api/v1/shelves/:id/books/:bookId` — добавление/удаление книги.
- Дефолтные полки «Хочу прочитать» / «Читаю» / «Прочитано» создаются лениво и взаимоисключающи (книга — ровно на одной из трёх); custom-полки независимы.
- Все эндпоинты под auth middleware; полки приватные — доступ только владельцу (чужая полка → 403).
- Миграция `000003_shelves_partial_unique`: partial unique index вместо `UNIQUE (user_id, type)` — разрешает несколько custom-полок.
- Unit-тесты service (`go test ./internal/modules/shelves/...`).

#### 🔲 Ratings / Reviews / Lists / Search / Admin
- Созданы только директории `domain/` с заготовками моделей.
- API, репозитории, сервисы и транспорт **не реализованы**.

### 3.4 Frontend

#### ✅ Web (`apps/web`)
- Главная страница с сеткой популярных книг.
- `/books` — каталог книг с пагинацией.
- `/books/[slug]` — страница книги.
- `/authors/[slug]` — страница автора.
- `/login` и `/register` — формы входа и регистрации (React Hook Form + Zod).
- `/profile` — страница профиля (защищена middleware).
- Cookie-based аутентификация через backend (`access_token` / `refresh_token` http-only cookies).
- Zustand-store для клиентского состояния пользователя.
- Next.js middleware для защиты роутов (`/profile`, `/shelves`) и редиректа авторизованных от `/login`/`/register`.
- Header с отображением состояния авторизации и кнопкой выхода.
- Кнопки «Хочу прочитать» / «Читаю» / «Прочитано» на странице книги (для анонима — редирект на `/login`).
- `/shelves` — «Мои полки»: список полок со счётчиками книг и форма создания custom-полки (защищена middleware).
- `/shelves/[id]` — книги полки с пагинацией; убрать книгу с полки; переименовать/удалить custom-полку.
- Ссылка «Мои полки» в header для авторизованных.
- SSR-загрузка данных через `@bookhaven/api-client`.
- Настроены `next.config.js`, `images.remotePatterns` для Open Library.

#### 🔲 Web — что ещё не сделано
- Восстановление пароля и подтверждение email.
- Редактирование профиля и загрузка аватара.
- Кнопки полок на карточках книг в каталоге (сейчас — только на странице книги).
- Рецензии, рейтинги.
- Поиск и фильтры в каталоге.

#### 🔲 Admin (`apps/admin`)
- Пока заглушка с базовой страницей.
- Нужен полноценный функционал управления книгами, авторами, жанрами, пользователями, модерацией.

### 3.5 Shared packages

#### ✅ `@bookhaven/api-client`
- HTTP-клиент на `fetch`.
- Методы для catalog, auth и shelves (`api.books.*`, `api.authors.*`, `api.genres.*`, `api.auth.*`, `api.shelves.*`).
- Типы для Book, Author, Genre, User, Review, Shelf, BookList.
- Серверный и клиентский URL API (`API_URL` / `NEXT_PUBLIC_API_URL`).

> **Технический долг:** сейчас типы дублируются в `api-client`, а `@bookhaven/types` почти не используется. Нужно вернуть общие типы через TypeScript project references.

#### ✅ `@bookhaven/ui`
- shadcn/ui компоненты (Button и др.).

#### ✅ `@bookhaven/config`
- Общие `tsconfig.json` для Next.js и React-библиотек.

#### 🔲 `@bookhaven/types`
- Пакет есть, но не интегрирован в `api-client` из-за проблем с разрешением workspace-зависимостей в Next.js build.

---

## 4. Как запустить проект

```bash
# 1. Перейти в корень проекта
cd /mnt/e/projects-ai/bookhaven

# 2. Запустить сервисы
docker compose up -d
# или
make up

# 3. Дождаться healthcheck PostgreSQL/Redis и первого pnpm install в web/admin (~30–60 сек)

# 4. Накатить миграции
make migrate

# 5. Заполнить тестовыми данными (один раз; при повторном запуске выдаст duplicate key — это нормально)
docker compose exec backend go run cmd/seed/main.go
```

### URL после запуска

- Web: http://localhost:3000
- Admin: http://localhost:3001
- API: http://localhost:8080
- Swagger: http://localhost:8080/swagger/index.html

### Полезные команды

```bash
make down                 # остановить все сервисы
make logs                 # логи всех сервисов
make shell-backend        # shell в backend-контейнере
make shell-web            # shell в web-контейнере
docker compose exec web pnpm type-check
docker compose exec web pnpm lint
docker compose exec backend go vet ./...
docker compose exec backend go test ./...
```

---

## 5. Роадмап

### ✅ Фаза 1. Frontend-аутентификация
**Статус: выполнена.** Базовый вход/регистрация работают, cookies ставит backend.

- [x] Страница `/login` с формой входа.
- [x] Страница `/register` с формой регистрации.
- [x] Валидация форм (Zod + React Hook Form).
- [x] Добавить в `api-client`: `auth.login`, `auth.register`, `auth.logout`, `auth.refresh`, `auth.me`.
- [x] Хранение токенов: http-only cookie от backend.
- [x] Middleware Next.js для защищённых роутов.
- [x] Отображение текущего пользователя в шапке / кнопка выхода.
- [x] Страница `/profile` (минимальная: имя, email, роль).

#### Оставшиеся подзадачи (можно вернуться позже)
- [ ] Восстановление пароля.
- [ ] Подтверждение email.
- [ ] Валидация access-токена в middleware через backend (сейчас проверяется только наличие cookie).

### Фаза 2. Книжные полки (Shelves)
**Статус: MVP реализована.** Приватные полки текущего пользователя: дефолтные «Хочу прочитать» / «Читаю» / «Прочитано» + custom-полки. Публичный просмотр полок других пользователей — позже (Фаза 6).

Backend:
- [x] `POST /api/v1/shelves` — создать custom-полку; `PATCH /api/v1/shelves/:id` — переименовать; `DELETE /api/v1/shelves/:id` — удалить (только custom).
- [x] `POST /api/v1/shelves/:id/books` — добавить книгу в полку.
- [x] `DELETE /api/v1/shelves/:id/books/:book_id` — убрать книгу из полки.
- [x] `GET /api/v1/shelves` — полки текущего пользователя (`?book_id=` → `has_book`); `GET /api/v1/shelves/:id` — книги полки с пагинацией.
- [x] Репозиторий, сервис, handler для shelves + unit-тесты service.
- [x] Дефолтные полки с эксклюзивным переносом книги; миграция `000003` (partial unique index — несколько custom-полок).

Frontend:
- [x] Кнопки «Хочу прочитать / Читаю / Прочитано» на странице книги (аноним → `/login?redirect=...`).
- [x] Страница `/shelves` с полками пользователя и формой создания.
- [x] Страница `/shelves/[id]` с книгами полки и управлением (убрать книгу, переименовать/удалить custom-полку).

Оставшиеся подзадачи:
- [ ] `GET /api/v1/users/:id/shelves` — публичный просмотр полок пользователя (актуально для Фазы 6).
- [ ] Кнопки полок на карточках книг в каталоге (сейчас только на странице книги).

### Фаза 3. Рейтинги и рецензии
**Приоритет: высокий.**

Backend:
- [ ] `POST /api/v1/books/:id/ratings` — поставить оценку.
- [ ] `GET/POST /api/v1/books/:id/reviews` — список и создание рецензий.
- [ ] `PATCH /api/v1/reviews/:id`, `DELETE /api/v1/reviews/:id` — редактирование/удаление своей рецензии.
- [ ] `POST /api/v1/reviews/:id/likes` — лайк рецензии.
- [ ] Пересчёт `rating` и `ratings_count` у книги при изменении оценки.
- [ ] Репозиторий, сервис, handler для ratings/reviews.

Frontend:
- [ ] Блок рейтинга на странице книги (звёзды).
- [ ] Список рецензий.
- [ ] Форма добавления/редактирования рецензии.

### Фаза 4. Поиск и фильтры в каталоге
**Приоритет: средний.**

- [ ] Поисковая строка на `/books`.
- [ ] Фильтры по жанру, автору, году издания, языку.
- [ ] Сортировка по рейтингу, году, дате добавления.
- [ ] Использование GIN-индексов (`idx_books_search`, `idx_authors_search`).

### Фаза 5. Пользовательские подборки (Lists)
**Приоритет: средний.**

- [ ] CRUD списков: `POST /api/v1/lists`, `GET /api/v1/lists/:id`, `PATCH`, `DELETE`.
- [ ] Добавление/удаление книг в список.
- [ ] Страница `/lists` и `/lists/[id]`.

### Фаза 6. Профили пользователей
**Приоритет: средний.**

- [ ] Редактирование профиля (имя, аватар, тип профиля).
- [ ] Страница пользователя `/users/[id]` с полками, рецензиями, списками.
- [ ] Загрузка аватаров (S3/minio или локальное хранилище).

### Фаза 7. Админ-панель
**Приоритет: средний.**

- [ ] Управление книгами: создание, редактирование, скрытие.
- [ ] Управление авторами и жанрами.
- [ ] Модерация рецензий и пользователей.
- [ ] Middleware/проверка роли `admin`/`moderator`.

### Фаза 8. Тесты, CI/CD и production
**Приоритет: низкий/фоновый, но критичен перед продом.**

- [ ] Unit-тесты backend (`go test`).
- [ ] Интеграционные тесты API.
- [ ] E2E-тесты frontend (Playwright).
- [ ] GitHub Actions: lint, type-check, go vet, tests, build Docker.
- [ ] Production build Next.js, оптимизация образов.
- [ ] Reverse proxy (nginx/traefik), SSL, мониторинг.

---

## 6. Технический долг

- [ ] **TypeScript project references.** Вернуть `@bookhaven/types` как единый источник типов вместо дублирования в `api-client`.
- [ ] **Hot-reload backend под WSL2.** Найти решение для Air на Windows FS или перейти на volume с inotify.
- [ ] **Makefile seed.** Добавить `make seed`, потому что README уже ссылается на неё.
- [ ] **README.** В README указан Next.js 15 / React 19, а в `package.json` Next.js 14 / React 18. Привести в соответствие.

---

## 7. Следующий рекомендуемый шаг

**Фаза 3 — Рейтинги и рецензии.**

Аутентификация и полки реализованы, персонализация работает. Следующий шаг — рейтинги и рецензии:

1. Backend-модуль ratings/reviews по образцу catalog и shelves (domain/repository/service/transport).
2. Пересчёт `rating` и `ratings_count` у книги при изменении оценки.
3. Блок рейтинга (звёзды) и список рецензий на странице книги.
4. Форма добавления/редактирования рецензии.
