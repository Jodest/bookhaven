---
name: bookhaven
description: Технические правила и архитектурные conventions проекта BookHaven
---

# BookHaven

BookHaven — социальный каталог книг для рынка РФ и СНГ, аналог Goodreads.

## Stack

Backend:

- Go 1.23
- Echo
- GORM
- PostgreSQL 16
- Redis 7

Frontend:

- Next.js 14
- App Router
- React 18
- TypeScript
- Tailwind CSS
- shadcn/ui

Monorepo:

- Turborepo
- pnpm workspaces

Infrastructure:

- Docker
- Docker Compose

Migrations:

- SQL
- golang-migrate compatible

## Repository

apps/web

Публичный frontend.

apps/admin

Административная панель.

backend

Go backend.

packages/api-client

HTTP client для backend API.

packages/types

Общие TypeScript types.

packages/ui

Shared UI components.

packages/config

Shared configuration.

## Backend

Backend организован по domain modules.

Типичная структура:

domain/
repository/
service/
handler/

Сохраняй существующие boundaries.

Не перемещай код между модулями без необходимости.

Используй существующие conventions проекта.

## Authentication

Используются:

- JWT access tokens;
- refresh tokens;
- bcrypt;
- auth middleware.

Frontend использует http-only cookies.

Все защищённые endpoints должны проверять authentication.

Для пользовательских ресурсов обязательно проверять ownership.

## Database

Используются SQL migrations.

Не изменяй уже применённые migrations.

Изменение схемы выполняй через новую migration.

Следуй существующим naming conventions.

## API

Backend API находится под:

/api/v1/

Сохраняй существующие conventions:

- HTTP status codes;
- error responses;
- pagination;
- validation;
- authentication.

Не создавай альтернативный API style для одной фичи.

## Frontend

Используй Next.js App Router.

Для API используй:

@bookhaven/api-client

Не создавай отдельный HTTP client без необходимости.

Переиспользуй:

@bookhaven/ui

и существующие components.

## Shared types

Предпочитай существующие shared types.

Не создавай дублирующие TypeScript types без необходимости.

Если существующая архитектура packages/types ещё не полностью интегрирована, не выполняй её масштабную перестройку в рамках unrelated feature.

## Testing

Backend:

go test ./...

go vet ./...

Frontend:

pnpm type-check

pnpm lint

Добавляй тесты для новой бизнес-логики.

## Code changes

Не изменяй unrelated code.

Не добавляй dependencies без необходимости.

Не переписывай работающий код без причины.

Сначала используй существующий pattern, затем создавай новый только если существующий неприменим.

## Git

Рабочие агенты не создают commits.

Commit создаёт только release agent после успешного Review + QA.

## Documentation

Основной roadmap:

docs/ROADMAP.md

Документация должна соответствовать фактическому состоянию кода.

Не отмечай незавершённую функциональность как выполненную.