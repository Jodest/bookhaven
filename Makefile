.PHONY: help up down build logs shell-backend shell-web shell-admin migrate migrate-down seed swagger test-backend test-web lint-backend lint-web

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

up: ## Start all services
	docker compose up -d

down: ## Stop all services
	docker compose down

build: ## Build all services
	docker compose build

logs: ## Show logs for all services
	docker compose logs -f

shell-backend: ## Open shell in backend container
	docker compose exec backend sh

shell-web: ## Open shell in web container
	docker compose exec web sh

shell-admin: ## Open shell in admin container
	docker compose exec admin sh

migrate: ## Run database migrations
	docker compose exec backend go run cmd/migrate/main.go up

migrate-down: ## Rollback database migrations
	docker compose exec backend go run cmd/migrate/main.go down

seed: ## Seed the database with dev/demo test data (refuses to run if APP_ENV=production; pass FORCE=1 to override)
	docker compose exec backend go run cmd/seed/main.go $(if $(FORCE),-force,)

swagger: ## Regenerate Swagger/OpenAPI docs (backend/docs)
	docker compose exec backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal

test-backend: ## Run backend tests
	docker compose exec backend go test ./...

test-web: ## Run web tests
	docker compose exec web pnpm test

lint-backend: ## Lint backend code
	docker compose exec backend go vet ./...

lint-web: ## Lint web code
	docker compose exec web pnpm lint
