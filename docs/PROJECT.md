# BookHaven

BookHaven is a social book catalog for the Russian/CIS market, inspired by Goodreads.

## Core Features

- Book catalog
- Authors
- Genres
- Authentication
- User shelves
- Ratings
- Reviews
- Custom shelves with a description and privacy
- User profiles
- Administration

## Stack

### Backend

- Go 1.27
- Echo
- GORM
- PostgreSQL 16
- Redis 7

### Frontend

- Next.js 14
- React 18
- TypeScript
- Tailwind CSS
- shadcn/ui

### Monorepo

- Turborepo
- pnpm workspaces

### Infrastructure

- Docker
- Docker Compose

## Repository

apps/web:
Public web application.

apps/admin:
Administration application.

backend:
Go API.

packages/api-client:
API client.

packages/types:
Shared TypeScript types.

packages/config:
Shared TypeScript configuration.

packages/ui:
Shared UI components.

docs:
Project documentation.