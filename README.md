# BookHaven

Социальный каталог книг для рынка РФ и СНГ.

## Стек

- **Backend:** Go, Echo, PostgreSQL, Redis
- **Frontend:** Next.js 15 App Router, React 19, TypeScript, Tailwind CSS, shadcn/ui
- **Monorepo:** Turborepo + pnpm workspaces
- **Инфраструктура:** Docker, Docker Compose

## Быстрый старт

```bash
# 1. Скопировать переменные окружения
cp .env.example .env

# 2. Запустить все сервисы
make up

# 3. Выполнить миграции
make migrate

# 4. Заполнить тестовыми данными
make seed
```

Тестовый аккаунт (создаётся сидом, только для разработки): `test@bookhaven.local` / `test1234`

## Сервисы

- Web: http://localhost:3000
- Admin: http://localhost:3001
- API: http://localhost:8080
- API Docs: http://localhost:8080/swagger/index.html
- Mailpit (все письма, отправленные локально): http://localhost:8025

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
