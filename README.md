# BookHaven

Социальный каталог книг для рынка РФ и СНГ.

## Стек

- **Backend:** Go, Echo, PostgreSQL, Redis
- **Frontend:** Next.js 15 App Router, React 19, TypeScript, Tailwind CSS, shadcn/ui
- **Monorepo:** Turborepo + pnpm workspaces
- **Инфраструктура:** Docker, Docker Compose

## Локальное тестовое окружение (Docker Compose)

Весь стек (PostgreSQL, Redis, backend API, worker, Mailpit, web, admin) поднимается одной командой. Нужен только Docker Desktop.

```bash
# 1. Скопировать переменные окружения (значения по умолчанию подходят для локальной работы)
cp .env.example .env

# 2. Запустить все сервисы (= make up)
docker compose up -d

# 3. Применить миграции (= make migrate)
docker compose exec backend go run ./cmd/migrate up

# 4. Создать тестовые аккаунты (без демо-каталога)
docker compose exec backend go run ./cmd/seed -accounts-only

#    ...или аккаунты + демо-каталог (~19 книг, жанры, авторы) (= make seed)
docker compose exec backend go run ./cmd/seed

# 5. (Необязательно) 12 демо-читателей с пересекающимися полками для блока
#    «Похожие книги» (этап 2, «читатели также читали») и ручной пересчёт
docker compose exec backend go run ./cmd/seed -demo-readers
docker compose exec backend go run ./cmd/similarity
```

Сид идемпотентен — его можно запускать повторно; `-accounts-only` можно позже «догнать» полным сидом. В `APP_ENV=production` сид отказывается работать без `-force`. Демо-читатели (`reader01..reader12@demo.bookhaven.local`) создаются без пароля, войти под ними нельзя. Пересчёт похожих книг воркер также запускает сам ежедневно в 03:30 UTC (см. [docs/features/similar-books-stage2.md](docs/features/similar-books-stage2.md)).

Первый запуск `web`/`admin` занимает несколько минут: контейнеры выполняют `pnpm install` в собственные (анонимные) тома `node_modules` и не трогают `node_modules` на хосте, поэтому нативные `pnpm`/`tsc`/`next` на хосте продолжают работать.

### Сервисы

| Сервис | URL |
|---|---|
| Web | http://localhost:3000 |
| Admin | http://localhost:3001 |
| API | http://localhost:8080 (health: `/health`, API: `/api/v1/...`) |
| API Docs (Swagger) | http://localhost:8080/swagger/index.html |
| Mailpit (почтовый ящик) | http://localhost:8025 |

### Тестовые аккаунты

Создаются сидом, только для разработки (оба с подтверждённым email):

| Роль | Email | Пароль |
|---|---|---|
| Пользователь | `test@bookhaven.local` | `test1234` |
| Администратор (вход в admin) | `admin@bookhaven.local` | `admin1234` |

### Почта

Все письма (подтверждение email при регистрации, повторная отправка подтверждения, сброс пароля и т.д.) никуда не доставляются, а попадают в Mailpit: http://localhost:8025. Чтобы проверить флоу регистрации, зарегистрируйтесь с любым адресом и откройте письмо в Mailpit.

### Автоперезагрузка при изменении кода

Изменения подхватываются автоматически, перезапускать контейнеры не нужно:

- `backend` (API) и `worker` работают под [air](https://github.com/air-verse/air) (`backend/.air.toml`, `backend/.air.worker.toml`) и пересобираются при изменении `.go`-файлов;
- `web` и `admin` — Next.js dev-серверы с hot reload.

Так как bind mount Docker Desktop на Windows/macOS не пробрасывает события файловой системы, отслеживание изменений работает через polling (`poll = true` в конфигах air, `WATCHPACK_POLLING=true` для Next.js); задержка — около секунды.

Пересборка образов (`docker compose up -d --build`) нужна только после изменения `Dockerfile`. После изменения зависимостей фронтенда (`package.json`/`pnpm-lock.yaml`) достаточно `docker compose restart web admin` — `pnpm install` выполняется при каждом старте контейнера.

## Команды

```bash
make help              # показать все команды
make up                # запустить сервисы
make down              # остановить сервисы
make logs              # логи
make shell-backend     # shell в backend-контейнере
make test-backend      # тесты backend
make test-web          # тесты frontend
```

## Архитектура

- Backend: модульный монолит с Clean Architecture
- Frontend: FSD (Feature-Sliced Design) внутри Next.js App Router
- Shared packages: ui, config, api-client, types
