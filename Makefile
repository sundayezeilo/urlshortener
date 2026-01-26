SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

ENV_FILE ?= .env
-include $(ENV_FILE)
export

.DEFAULT_GOAL := help

# Tools
GO             ?= go
MIGRATE        ?= migrate
SQLC           ?= sqlc
GOLANGCI_LINT  ?= golangci-lint
DOCKER_COMPOSE ?= docker compose

# Paths
MIGRATIONS    ?= db/migrations
COVERAGE_OUT  ?= coverage.out
COVERAGE_HTML ?= coverage.html
COVER_EXCLUDE ?= /internal/db/sqlc
CMD_DIR       ?= ./cmd/server
BUILD_DIR     ?= bin
APP_NAME      ?= urlshortener

# Database DSN
DB_DSN ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

# Helper to check required variables
define require
	@test -n "$($1)" || (echo "❌ Missing required variable: $1" && exit 1)
endef

.PHONY: help install-tools

help:
	@echo ""
	@echo "📦 URL Shortener - Makefile Commands"
	@echo ""
	@echo "🚀 Development:"
	@echo "  make dev              - Start database and run server"
	@echo "  make run              - Build and run the server"
	@echo "  make build            - Build the server binary"
	@echo ""
	@echo "🐳 Docker:"
	@echo "  make db-start         - Start PostgreSQL and wait for ready"
	@echo "  make db-up            - Start PostgreSQL (don't wait)"
	@echo "  make db-stop          - Stop PostgreSQL"
	@echo "  make db-down          - Stop and remove PostgreSQL"
	@echo "  make db-logs          - View PostgreSQL logs"
	@echo "  make db-psql          - Connect to PostgreSQL with psql"
	@echo ""
	@echo "🗄️  Database Migrations:"
	@echo "  make migrate-create name=NAME  - Create a new migration"
	@echo "  make migrate-up       - Run all pending migrations"
	@echo "  make migrate-down     - Rollback last migration"
	@echo "  make migrate-version  - Show current migration version"
	@echo "  make migrate-force VERSION=N   - Force version N (use carefully!)"
	@echo ""
	@echo "🧪 Testing:"
	@echo "  make test             - Run all unit tests"
	@echo "  make test-unit        - Run unit tests only"
	@echo "  make test-e2e         - Run end-to-end tests (with testcontainers)"
	@echo "  make coverage         - Generate coverage report"
	@echo "  make coverage-view    - Open coverage in browser"
	@echo ""
	@echo "🔧 Code Quality:"
	@echo "  make fmt              - Format code"
	@echo "  make lint             - Run linter"
	@echo "  make tidy             - Run go mod tidy"
	@echo "  make sqlc             - Generate sqlc code"
	@echo ""
	@echo "🛠️  Tools:"
	@echo "  make install-tools    - Install all required tools"
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  make clean            - Remove coverage and build files"
	@echo "  make clean-sqlc       - Remove generated sqlc code"
	@echo ""

# =============================================================================
# Development
# =============================================================================

dev: db-start migrate-up
	@echo "🚀 Starting server in development mode..."
	@$(GO) run $(CMD_DIR)

build:
	@echo "🔨 Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GO) build -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "✅ Build complete: $(BUILD_DIR)/$(APP_NAME)"

run: build
	@echo "🚀 Running $(APP_NAME)..."
	@$(BUILD_DIR)/$(APP_NAME)

# =============================================================================
# Docker
# =============================================================================

db-up:
	@echo "🐳 Starting PostgreSQL..."
	@$(DOCKER_COMPOSE) up -d postgres
	@echo "✅ PostgreSQL started"

db-stop:
	@echo "⏸️  Stopping PostgreSQL..."
	@$(DOCKER_COMPOSE) stop postgres

db-down:
	@echo "🗑️  Removing PostgreSQL..."
	@$(DOCKER_COMPOSE) down

db-wait:
	@echo "⏳ Waiting for PostgreSQL to be ready..."
	@until $(DOCKER_COMPOSE) ps postgres | grep -q "healthy"; do \
		echo "   Still waiting..."; \
		sleep 2; \
	done
	@echo "✅ PostgreSQL is ready"

db-start: db-up db-wait

db-logs:
	@$(DOCKER_COMPOSE) logs -f postgres

db-psql:
	$(call require,DB_USER)
	$(call require,DB_NAME)
	@echo "🔌 Connecting to PostgreSQL..."
	@$(DOCKER_COMPOSE) exec postgres psql -U "$(DB_USER)" -d "$(DB_NAME)"

# =============================================================================
# Database Migrations
# =============================================================================

require-db:
	$(call require,DB_HOST)
	$(call require,DB_PORT)
	$(call require,DB_USER)
	$(call require,DB_PASSWORD)
	$(call require,DB_NAME)
	$(call require,DB_SSLMODE)

migrate-create:
	@test -n "$(name)" || (echo "❌ name is required. Usage: make migrate-create name=create_users_table" && exit 1)
	@echo "📝 Creating migration: $(name)"
	@$(MIGRATE) create -ext sql -dir $(MIGRATIONS) $(name)
	@echo "✅ Migration files created in $(MIGRATIONS)/"

migrate-up: require-db
	@echo "⬆️  Running migrations..."
	@$(MIGRATE) -path $(MIGRATIONS) -database "$(DB_DSN)" up
	@echo "✅ Migrations complete"

migrate-down: require-db
	@echo "⬇️  Rolling back last migration..."
	@$(MIGRATE) -path $(MIGRATIONS) -database "$(DB_DSN)" down 1
	@echo "✅ Rollback complete"

migrate-version: require-db
	@echo "📊 Current migration version:"
	@$(MIGRATE) -path $(MIGRATIONS) -database "$(DB_DSN)" version

migrate-force: require-db
	@test -n "$(VERSION)" || (echo "❌ VERSION is required. Usage: make migrate-force VERSION=20251231010534" && exit 1)
	@echo "⚠️  Forcing migration version to $(VERSION)..."
	@$(MIGRATE) -path $(MIGRATIONS) -database "$(DB_DSN)" force $(VERSION)
	@echo "✅ Version forced to $(VERSION)"

# =============================================================================
# Code Generation
# =============================================================================

clean-sqlc:
	@echo "🧹 Removing generated sqlc code..."
	@rm -rf internal/db/sqlc

sqlc: clean-sqlc
	@echo "⚙️  Generating sqlc code..."
	@$(SQLC) generate
	@echo "✅ sqlc generation complete"

# =============================================================================
# Testing
# =============================================================================

test:
	@echo "🧪 Running all tests..."
	@$(GO) test ./... -v

test-unit:
	@echo "🧪 Running unit tests..."
	@$(GO) test ./internal/... ./sluggen/... ./idgen/... -v

test-e2e:
	@echo "🧪 Running end-to-end tests with testcontainers..."
	@echo "⚠️  This will start a PostgreSQL container automatically"
	@$(GO) test ./test/e2e/... -v -timeout 5m

test-e2e-short:
	@echo "🧪 Running end-to-end tests (short mode)..."
	@$(GO) test ./test/e2e/... -v -short -timeout 2m

clean:
	@echo "🧹 Cleaning up..."
	@rm -f $(COVERAGE_OUT) $(COVERAGE_HTML)
	@rm -rf $(BUILD_DIR)
	@echo "✅ Cleanup complete"

coverage: clean
	@echo "📊 Generating coverage report..."
	@PKGS=$$($(GO) list ./... | grep -v '$(COVER_EXCLUDE)'); \
	$(GO) test $$PKGS \
		-coverprofile=$(COVERAGE_OUT) \
		-covermode=atomic \
		-coverpkg=$$(echo $$PKGS | tr ' ' ',')
	@$(GO) tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo ""
	@echo "📈 Coverage Summary:"
	@$(GO) tool cover -func=$(COVERAGE_OUT) | tail -n 1
	@echo ""
	@echo "📄 Detailed report: $(COVERAGE_HTML)"

coverage-func: clean
	@echo "📊 Running tests with coverage..."
	@PKGS=$$($(GO) list ./... | grep -v '$(COVER_EXCLUDE)'); \
	$(GO) test $$PKGS \
		-coverprofile=$(COVERAGE_OUT) \
		-covermode=atomic \
		-coverpkg=$$(echo $$PKGS | tr ' ' ',')
	@$(GO) tool cover -func=$(COVERAGE_OUT)

coverage-view: coverage
	@echo "🌐 Opening coverage report in browser..."
	@command -v open >/dev/null 2>&1 && open $(COVERAGE_HTML) || true
	@command -v xdg-open >/dev/null 2>&1 && xdg-open $(COVERAGE_HTML) || true
	@command -v start >/dev/null 2>&1 && start $(COVERAGE_HTML) || true

# =============================================================================
# Code Quality
# =============================================================================

fmt:
	@echo "✨ Formatting code..."
	@gofmt -s -w .
	@echo "✅ Code formatted"

lint:
	@echo "🔍 Running linter..."
	@$(GOLANGCI_LINT) run
	@echo "✅ Linting complete"

tidy:
	@echo "📦 Tidying dependencies..."
	@$(GO) mod tidy
	@echo "✅ Dependencies tidied"

# =============================================================================
# Tools Installation
# =============================================================================

install-tools:
	@echo "🛠️  Installing required tools..."
	@echo ""
	@echo "Installing golang-migrate..."
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo ""
	@echo "Installing sqlc..."
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@echo ""
	@echo "Installing golangci-lint..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo ""
	@echo "Installing godotenv (for .env file support)..."
	@go get github.com/joho/godotenv
	@echo ""
	@echo "Installing testcontainers-go..."
	@go get github.com/testcontainers/testcontainers-go
	@go get github.com/testcontainers/testcontainers-go/modules/postgres
	@echo ""
	@echo "✅ All tools installed!"
	@echo ""
	@echo "Tool versions:"
	@echo "  migrate: $$(migrate -version 2>&1 | head -n 1 || echo 'not found')"
	@echo "  sqlc: $$(sqlc version 2>&1 || echo 'not found')"
	@echo "  golangci-lint: $$(golangci-lint version 2>&1 | head -n 1 || echo 'not found')"
	@echo ""
	@echo "💡 Make sure $$GOPATH/bin is in your PATH"

# =============================================================================
# Utility targets
# =============================================================================

.PHONY: all setup init

setup: install-tools
	@echo "🎉 Setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Copy .env.example to .env:  cp .env.example .env"
	@echo "  2. Update .env with your settings"
	@echo "  3. Start development:  make dev"

init: setup
	@echo "🚀 Initializing project..."
	# @test -f .env || cp .env.example .env
	@echo "✅ .env file created"
	@echo ""
	@echo "Ready to go! Run: make dev"

all: fmt lint test build
	@echo "✅ All checks passed and binary built!"
