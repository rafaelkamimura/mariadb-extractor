# MariaDB Extractor - Development Makefile
.PHONY: help build up down restart logs clean extract ddl dump dev-db dev-db-logs dev-db-connect

# Default target
help: ## Show this help message
	@echo "MariaDB Extractor Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Docker Image Management
build: ## Build the mariadb-extractor Docker image
	docker build -t mariadb-extractor .

build-no-cache: ## Build the mariadb-extractor Docker image without cache
	docker build --no-cache -t mariadb-extractor .

# Local Development Database
up: ## Start the local MariaDB development environment
	docker-compose up -d mariadb adminer

down: ## Stop the local MariaDB development environment
	docker-compose down

restart: ## Restart the local MariaDB development environment
	docker-compose restart mariadb adminer

dev-db: ## Start only the MariaDB development database
	docker-compose up -d mariadb

dev-db-logs: ## Show logs from the development database
	docker-compose logs -f mariadb

dev-db-connect: ## Connect to the development database via mysql client
	docker-compose exec mariadb mysql -u root -p

# Extraction Commands
extract: ## Extract database metadata from configured server
	docker run --rm --network mariadb-extractor_mariadb-network \
		-e MARIADB_HOST=${MARIADB_HOST} \
		-e MARIADB_PORT=${MARIADB_PORT} \
		-e MARIADB_USER=${MARIADB_USER} \
		-e MARIADB_PASSWORD=${MARIADB_PASSWORD} \
		-v $(PWD):/app/output \
		mariadb-extractor extract

extract-local: ## Extract from local development database
	docker run --rm --network mariadb-extractor_mariadb-network \
		-e MARIADB_HOST=mariadb \
		-e MARIADB_PORT=3306 \
		-e MARIADB_USER=root \
		-e MARIADB_PASSWORD=${MYSQL_ROOT_PASSWORD:-password} \
		-v $(PWD):/app/output \
		mariadb-extractor extract -o local-dev

ddl: ## Extract DDL statements from configured server
	docker run --rm --network mariadb-extractor_mariadb-network \
		-e MARIADB_HOST=${MARIADB_HOST} \
		-e MARIADB_PORT=${MARIADB_PORT} \
		-e MARIADB_USER=${MARIADB_USER} \
		-e MARIADB_PASSWORD=${MARIADB_PASSWORD} \
		-v $(PWD):/app/output \
		mariadb-extractor ddl

ddl-local: ## Extract DDL from local development database
	docker run --rm --network mariadb-extractor_mariadb-network \
		-e MARIADB_HOST=mariadb \
		-e MARIADB_PORT=3306 \
		-e MARIADB_USER=root \
		-e MARIADB_PASSWORD=${MYSQL_ROOT_PASSWORD:-password} \
		-v $(PWD):/app/output \
		mariadb-extractor ddl -o local-ddl

dump: ## Create full database dump from configured server
	docker run --rm --network mariadb-extractor_mariadb-network \
		-e MARIADB_HOST=${MARIADB_HOST} \
		-e MARIADB_PORT=${MARIADB_PORT} \
		-e MARIADB_USER=${MARIADB_USER} \
		-e MARIADB_PASSWORD=${MARIADB_PASSWORD} \
		-v $(PWD):/app/output \
		mariadb-extractor dump --all-databases

dump-local: ## Create dump from local development database
	docker run --rm --network mariadb-extractor_mariadb-network \
		-e MARIADB_HOST=mariadb \
		-e MARIADB_PORT=3306 \
		-e MARIADB_USER=root \
		-e MARIADB_PASSWORD=${MYSQL_ROOT_PASSWORD:-password} \
		-v $(PWD):/app/output \
		mariadb-extractor dump --all-databases -o local-dump

# Development Workflow
setup-dev: ## Set up complete development environment
	@echo "Setting up MariaDB Extractor development environment..."
	$(MAKE) build
	$(MAKE) up
	@echo "Waiting for database to be ready..."
	@sleep 30
	@echo "Development environment is ready!"
	@echo "Adminer available at: http://localhost:8080"
	@echo "MariaDB available at: localhost:3307"

extract-to-dev: ## Extract from production and update local dev database
	@echo "Extracting from production database..."
	$(MAKE) dump
	@echo "Updating local development database..."
	cp mariadb-dump.sql init-scripts/01-production-data.sql
	$(MAKE) restart
	@echo "Local development database updated with production data!"

backup-local: ## Create backup of local development database
	@echo "Creating backup of local development database..."
	docker run --rm --network mariadb-extractor_mariadb-network \
		-e MARIADB_HOST=mariadb \
		-e MARIADB_PORT=3306 \
		-e MARIADB_USER=root \
		-e MARIADB_PASSWORD=${MYSQL_ROOT_PASSWORD:-password} \
		-v $(PWD):/app/output \
		mariadb-extractor dump --all-databases -o local-backup-$(shell date +%Y%m%d-%H%M%S)

# Utility Commands
logs: ## Show logs from all services
	docker-compose logs -f

shell: ## Open shell in the mariadb-extractor container
	docker run --rm -it --network mariadb-extractor_mariadb-network \
		-v $(PWD):/app/output \
		mariadb-extractor sh

clean: ## Clean up generated files and stop containers
	docker-compose down -v
	docker system prune -f
	rm -f mariadb-*.sql mariadb-*.sql.gz mariadb-*.md mariadb-*.json *-dump.sql *-ddl.md *-extract.*

clean-db: ## Clean up database data (WARNING: This will delete all data!)
	@echo "WARNING: This will delete all database data!"
	@read -p "Are you sure? (y/N) " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		docker-compose down -v; \
		docker volume rm mariadb-extractor_mariadb_data 2>/dev/null || true; \
		echo "Database data cleaned."; \
	else \
		echo "Operation cancelled."; \
	fi

# Environment Setup
env-example: ## Create .env file from example
	if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env file from .env.example"; \
		echo "Please edit .env with your database credentials."; \
	else \
		echo ".env file already exists."; \
	fi

# Information
status: ## Show status of all services
	@echo "=== Docker Services Status ==="
	@docker-compose ps
	@echo ""
	@echo "=== Docker Images ==="
	@docker images mariadb-extractor 2>/dev/null || echo "mariadb-extractor image not found"
	@echo ""
	@echo "=== Docker Volumes ==="
	@docker volume ls | grep mariadb-extractor || echo "No mariadb-extractor volumes found"
	@echo ""
	@echo "=== Generated Files ==="
	@ls -la mariadb-* *-dump* *-ddl* *-extract* 2>/dev/null || echo "No generated files found"

# Quick Start Commands
quick-start: ## Quick start for new developers
	@echo "🚀 Quick Start Guide for MariaDB Extractor"
	@echo ""
	@echo "1. Set up environment:"
	@echo "   make env-example"
	@echo "   # Edit .env with your database credentials"
	@echo ""
	@echo "2. Start development environment:"
	@echo "   make setup-dev"
	@echo ""
	@echo "3. Access your database:"
	@echo "   - Adminer (web UI): http://localhost:8080"
	@echo "   - MySQL client: make dev-db-connect"
	@echo ""
	@echo "4. Extract data from production:"
	@echo "   make extract"
	@echo "   make ddl"
	@echo "   make dump"
	@echo ""
	@echo "5. Update local dev with production data:"
	@echo "   make extract-to-dev"
	@echo ""
	@echo "Happy coding! 🎉"