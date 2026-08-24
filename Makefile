# ==============================================================================
# Variables
# ==============================================================================
APP_NAME       := invise-backend
MAIN_PATH      := ./cmd/api
MIGRATE_PATH   := ./cmd/migrate
BIN_DIR        := ./build/bin
ENV_FILE       := .env
MIGRATION_DIR  := ./db/migrations
LOG_DIR        := ./logs
CONTAINER_FILE := Containerfile
DOCKER_IMAGE   := $(APP_NAME):latest

# Go commands
GOCMD   := go
GORUN   := $(GOCMD) run
GOBUILD := $(GOCMD) build
GOTEST  := $(GOCMD) test
GOMOD   := $(GOCMD) mod

.DEFAULT_GOAL := help

.PHONY: all help setup run dev build local-build test test-cover fmt lint tidy clean \
        db-create db-migrate db-rollback db-status db-reset db-version \
        docker-build up down restart logs logs-all status \
        db-backup db-restore db-shell migrate-up migrate-down migrate-status \
        health clean-all

# ==============================================================================
# Help
# ==============================================================================
help: ## Show this help message
	@echo "Invise Backend Makefile"
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

all: fmt lint test build ## Format, lint, test, and build the application

# ==============================================================================
# Setup & Development
# ==============================================================================
setup: ## Setup development environment (.env, directories, dependencies)
	@echo "Setting up development environment..."
	@if [ ! -f $(ENV_FILE) ]; then \
		if [ -f .env.example ]; then \
			cp .env.example $(ENV_FILE); \
			echo "Created $(ENV_FILE) from .env.example"; \
		else \
			touch $(ENV_FILE); \
			echo "Created empty $(ENV_FILE)"; \
		fi \
	else \
		echo "$(ENV_FILE) already exists"; \
	fi
	@mkdir -p $(LOG_DIR) $(BIN_DIR)
	@$(GOMOD) download
	@echo "Setup complete! Edit $(ENV_FILE) if needed."

run: ## Run application locally
	@echo "Starting application..."
	@mkdir -p $(LOG_DIR)
	@$(GORUN) $(MAIN_PATH)

dev: ## Run in development mode with hot-reload (air)
	@echo "Starting development mode..."
	@mkdir -p $(LOG_DIR)
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not installed (run: go install github.com/air-verse/air@latest). Running directly..."; \
		$(GORUN) $(MAIN_PATH); \
	fi

tidy: ## Tidy and verify go modules
	@echo "Tidying go modules..."
	@$(GOMOD) tidy

# ==============================================================================
# Code Quality & Testing
# ==============================================================================
fmt: ## Format Go source code
	@echo "Formatting code..."
	@gofmt -w -s .
	@echo "Code formatted!"

lint: ## Run golangci-lint
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed (run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)"; \
	fi

test: ## Run unit tests with race detection
	@echo "Running tests..."
	@$(GOTEST) -v -race ./...

test-cover: ## Run unit tests with coverage report
	@echo "Running tests with coverage..."
	@$(GOTEST) -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -func=coverage.out

# ==============================================================================
# Build & Clean
# ==============================================================================
build: local-build ## Alias for local-build

local-build: ## Build application and migration binaries into build/bin
	@echo "Building binaries..."
	@mkdir -p $(BIN_DIR)
	@$(GOBUILD) -o $(BIN_DIR)/$(APP_NAME) $(MAIN_PATH)
	@$(GOBUILD) -o $(BIN_DIR)/migrate $(MIGRATE_PATH)
	@echo "Build successful! Binaries in $(BIN_DIR)/"

clean: ## Remove build artifacts and temporary files
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR) $(LOG_DIR) coverage.out
	@echo "Cleaned!"

# ==============================================================================
# Database & Migrations
# ==============================================================================
db-create: ## Create new migration file (usage: make db-create name=create_users_table)
	@if [ -z "$(name)" ]; then \
		echo "Error: Migration name not specified. Usage: make db-create name=create_users_table"; \
		exit 1; \
	fi
	@$(GORUN) $(MIGRATE_PATH) create $(name)

db-migrate: ## Run database migrations up
	@echo "Applying database migrations..."
	@$(GORUN) $(MIGRATE_PATH) up

db-rollback: ## Rollback last database migration
	@echo "Rolling back database migration..."
	@$(GORUN) $(MIGRATE_PATH) down

db-status: ## Check database migration status
	@$(GORUN) $(MIGRATE_PATH) status

db-reset: ## Reset all migrations (WARNING: drops all tables!)
	@echo "WARNING: This will reset ALL migrations and drop all tables!"
	@read -p "Type 'yes' to continue: " confirm && [ "$$confirm" = "yes" ] || exit 1
	@$(GORUN) $(MIGRATE_PATH) reset

db-version: ## Print current database migration version
	@$(GORUN) $(MIGRATE_PATH) version

migrate-up: db-migrate ## Alias for db-migrate
migrate-down: db-rollback ## Alias for db-rollback
migrate-status: db-status ## Alias for db-status

# ==============================================================================
# Docker & Compose
# ==============================================================================
docker-build: ## Build container image using Containerfile
	@echo "Building Docker image..."
	docker build -f $(CONTAINER_FILE) -t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

up: ## Start all services with Docker Compose
	@echo "Starting all services..."
	docker compose up -d
	@echo "Services started:"
	@echo "  Application: http://localhost:8080"
	@echo "  Postgres:    http://localhost:5432"
	@echo "  MinIO:       http://localhost:9000 (Console: http://localhost:9001)"
	@echo "  Valkey:      http://localhost:6379"

down: ## Stop all services
	@echo "Stopping all services..."
	docker compose down

restart: ## Restart all services
	@echo "Restarting services..."
	docker compose restart

logs: ## Tail application container logs
	docker compose logs -f app

logs-all: ## Tail all container logs
	docker compose logs -f

status: ## Show status of running containers
	docker compose ps

db-shell: ## Open interactive PostgreSQL shell in container
	docker compose exec postgres psql -U postgres -d backend

db-backup: ## Backup database from container into backups/ directory
	@mkdir -p backups
	@BACKUP_FILE=backups/backup_$$(date +%Y%m%d_%H%M%S).sql; \
	docker compose exec postgres pg_dumpall -U postgres > $$BACKUP_FILE; \
	echo "Database backup created at $$BACKUP_FILE"

db-restore: ## Restore database from backup file (usage: make db-restore FILE=backups/backup.sql)
	@if [ -z "$(FILE)" ]; then \
		echo "Error: Please specify backup file: make db-restore FILE=backups/backup.sql"; \
		exit 1; \
	fi
	@echo "Restoring database from $(FILE)..."
	docker compose exec -T postgres psql -U postgres < $(FILE)
	@echo "Database restored!"

health: ## Check application health endpoint
	@echo "Checking application health..."
	@if curl -f -s http://localhost:8080/health > /dev/null; then \
		echo "Application is healthy!"; \
	else \
		echo "Health check failed (is the application running on port 8080?)"; \
		exit 1; \
	fi

clean-all: ## Stop containers, remove volumes and unused images
	@echo "Cleaning up all docker resources..."
	docker compose down -v
	docker system prune -f
	@echo "Cleanup complete!"