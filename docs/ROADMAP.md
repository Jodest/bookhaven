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

#### 🔲 Shelves / Ratings / Reviews / Lists / Search / Admin
- Созданы только директории `domain/` с заготовками моделей.
- API, репозитории, сервисы и транспорт **не реализованы**.

### 3.4 Frontend

#### ✅ Web (`apps/web`)
- Главная страница с сеткой популярных книг.
- `/books` — каталог книг с пагинацией.
- `/books/[slug]` — страница книги.
- `/authors/[slug]` — страница автора.
- SSR-загрузка данных через `@bookhaven/api-client`.
- Настроены `next.config.js`, `images.remotePatterns` для Open Library.

#### 🔲 Web — что ещё не сделано
- Страницы входа/регистрации.
- Клиентская аутентификация (хранение токенов, защищённые роуты).
- Полки, рецензии, рейтинги, профиль.
- Поиск и фильтры в каталоге.

#### 🔲 Admin (`apps/admin`)
- Пока заглушка с базовой страницей.
- Нужен полноценный функционал управления книгами, авторами, жанрами, пользователями, модерацией.

### 3.5 Shared packages

#### ✅ `@bookhaven/api-client`
- HTTP-клиент на `fetch`.
- Методы для catalog и auth (`api.books.*`, `api.authors.*`, `api.genres.*`, `api.auth.me`).
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

### Фаза 1. Frontend-аутентификация
**Приоритет: высокий.** Без неё нельзя делать полки, рецензии и профиль.

- [ ] Страница `/login` с формой входа.
- [ ] Страница `/register` с формой регистрации.
- [ ] Валидация форм (Zod + React Hook Form).
- [ ] Добавить в `api-client`: `auth.login`, `auth.register`, `auth.logout`, `auth.refresh`.
- [ ] Хранение токенов: http-only cookie (предпочтительно) или memory + refresh.
- [ ] Middleware Next.js для защищённых роутов.
- [ ] Отображение текущего пользователя в шапке / выпадающее меню.
- [ ] Страница `/profile` (минимальная: имя, email, аватар).

### Фаза 2. Книжные полки (Shelves)
**Приоритет: высокий.** Является ключевой фичей Goodreads.

Backend:
- [ ] `POST /api/v1/shelves` — создать/обновить полку пользователя.
- [ ] `POST /api/v1/shelves/:id/books` — добавить книгу в полку.
- [ ] `DELETE /api/v1/shelves/:id/books/:book_id` — убрать книгу из полки.
- [ ] `GET /api/v1/users/:id/shelves` — полки пользователя.
- [ ] Репозиторий, сервис, handler для shelves.

Frontend:
- [ ] Кнопки «Хочу прочитать / Читаю / Прочитал» на карточке книги и странице книги.
- [ ] Страница `/shelves` или `/profile/shelves` с полками пользователя.

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

**Фаза 1 — Frontend-аутентификация.**

Это разблокирует все персонализированные фичи: полки, рецензии, профиль. Начать стоит с:

1. Добавить методы `login`, `register`, `logout`, `refresh` в `@bookhaven/api-client`.
2. Создать страницы `/login` и `/register` в `apps/web`.
3. Настроить хранение токенов и Next.js middleware для защищённых роутов.
4. Вывести текущего пользователя в шапку.

Если хочешь, я могу сразу приступить к реализации этой фазы.
