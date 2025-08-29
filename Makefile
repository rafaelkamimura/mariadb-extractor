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
	docker run --rm \
		--env-file .env \
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
	docker run --rm \
		--env-file .env \
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

dump: ## Create database dump of user databases (recommended)
	@echo "Starting database dump of user databases (excluding system databases)..."
	@echo "This may take a while for large databases..."
	@echo "If this hangs or fails, try: make dump-specific DB=name"
	docker run --rm \
		--env-file .env \
		-v $(PWD):/app/output \
		mariadb-extractor dump --all-user-databases 2>&1 | tee dump.log
	@echo "Dump completed. Check mariadb-dump.sql and dump.log for details."

dump-all: ## Create full database dump including system databases
	@echo "Starting full database dump (including system databases)..."
	@echo "WARNING: This will include large system databases and may take a very long time!"
	docker run --rm \
		--env-file .env \
		-v $(PWD):/app/output \
		mariadb-extractor dump --all-databases 2>&1 | tee dump-all.log
	@echo "Full dump completed. Check mariadb-dump.sql and dump-all.log for details."

dump-specific: ## Dump a specific database (usage: make dump-specific DB=database_name)
	@if [ -z "$(DB)" ]; then \
		echo "Error: Please specify database name with DB= parameter"; \
		echo "Example: make dump-specific DB=myapp"; \
		exit 1; \
	fi
	@echo "Starting database dump of: $(DB)"
	docker run --rm \
		--env-file .env \
		-v $(PWD):/app/output \
		mariadb-extractor dump --databases $(DB) 2>&1 | tee dump-$(DB).log
	@echo "Dump of $(DB) completed. Check mariadb-dump.sql and dump-$(DB).log for details."

test-connection: ## Test database connection
	@echo "Testing database connection..."
	docker run --rm \
		--env-file .env \
		mariadb-extractor extract --help > /dev/null 2>&1 && echo "✅ Connection successful!" || echo "❌ Connection failed!"
	@echo "If connection fails, check your .env file and database credentials."

test-dump-progress: ## Test dump progress with first 3 databases
	@echo "Testing dump progress with first 3 databases..."
	@echo "This will show you the progress format without dumping everything"
	$(eval FIRST_3_DBS := $(shell docker run --rm --env-file .env mariadb-extractor sh -c "cd /app && go run . dump --help > /dev/null 2>&1 && echo 'adh,agendamento,agendamento_eventos'" 2>/dev/null || echo "adh,agendamento,agendamento_eventos"))
	docker run --rm \
		--env-file .env \
		-v $(PWD):/app/output \
		mariadb-extractor dump --databases $(FIRST_3_DBS) 2>&1 | head -20
	@echo ""
	@echo "✅ Progress test completed! Use 'make dump' for full dump or 'make extract-to-dev' for production sync."

dump-safe: ## Create database dump excluding system databases
	@echo "Starting safe database dump (excluding system databases)..."
	docker run --rm \
		--env-file .env \
		-v $(PWD):/app/output \
		mariadb-extractor dump --databases $(shell docker run --rm --env-file .env -v $(PWD):/app/output mariadb-extractor extract 2>/dev/null | grep -o '"name":"[^"]*"' | cut -d'"' -f4 | grep -v -E '^(information_schema|mysql|performance_schema|sys)$' | tr '\n' ',' | sed 's/,$$//') 2>&1 | tee dump-safe.log
	@echo "Safe dump completed. Check mariadb-dump.sql and dump-safe.log for details."

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
	@if [ -f "init-scripts/01-production-data.sql" ]; then \
		echo "Found production data - using extracted database schema"; \
	else \
		echo "No production data found - using sample data for testing"; \
	fi
	$(MAKE) up
	@echo "Waiting for database to be ready..."
	@sleep 30
	@echo "Development environment is ready!"
	@echo "Adminer available at: http://localhost:8080"
	@echo "MariaDB available at: localhost:3307"
	@if [ -f "init-scripts/01-production-data.sql" ]; then \
		echo "Database loaded with: Production data"; \
	else \
		echo "Database loaded with: Sample data"; \
	fi

extract-to-dev: ## Extract from production and update local dev database
	@echo "🚀 Extracting from production database..."
	@echo "This will dump all user databases (excluding system databases)"
	@echo "Progress will be shown for each database..."
	$(MAKE) dump
	@echo ""
	@echo "📦 Updating local development database..."
	@echo "Note: This will replace the current local database with production data"
	@read -p "Continue with database replacement? (y/N) " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(MAKE) import-production-data; \
		echo ""; \
		echo "✅ Local development database updated with production data!"; \
		echo "🌐 Access your database:"; \
		echo "   - Adminer: http://localhost:8080"; \
		echo "   - MySQL: make dev-db-connect"; \
	else \
		echo "❌ Operation cancelled."; \
	fi

extract-to-dev-full: ## Extract from production (including system DBs) and update local dev
	@echo "Extracting from production database (including system databases)..."
	@echo "WARNING: This will include large system databases!"
	$(MAKE) dump-all
	@echo "Updating local development database..."
	@echo "Note: This will replace the current local database with production data"
	@read -p "Continue? (y/N) " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(MAKE) import-production-data; \
		echo "Local development database updated with production data!"; \
	else \
		echo "Operation cancelled."; \
	fi

import-production-data: ## Import production data into local development database
	@echo "Stopping current database..."
	$(MAKE) down
	@echo "Removing old database volume..."
	docker volume rm mariadb-extractor_mariadb_data 2>/dev/null || true
	@echo "Copying production data to init scripts..."
	cp mariadb-dump.sql init-scripts/01-production-data.sql
	@echo "Starting fresh database with production data..."
	$(MAKE) up
	@echo "Waiting for database to initialize with new data..."
	@sleep 30
	@echo "Production data imported successfully!"

use-sample-data: ## Switch to sample data for local development
	@echo "Switching to sample data..."
	$(MAKE) down
	docker volume rm mariadb-extractor_mariadb_data 2>/dev/null || true
	@if [ -f "init-scripts/01-production-data.sql" ]; then \
		mv init-scripts/01-production-data.sql init-scripts/01-production-data.sql.backup; \
	fi
	$(MAKE) up
	@echo "Waiting for database to initialize..."
	@sleep 30
	@echo "Database loaded with sample data!"

use-production-data: ## Switch back to production data
	@echo "Switching to production data..."
	@if [ ! -f "init-scripts/01-production-data.sql" ] && [ -f "init-scripts/01-production-data.sql.backup" ]; then \
		mv init-scripts/01-production-data.sql.backup init-scripts/01-production-data.sql; \
		$(MAKE) down; \
		docker volume rm mariadb-extractor_mariadb_data 2>/dev/null || true; \
		$(MAKE) up; \
		echo "Waiting for database to initialize..."; \
		sleep 30; \
		echo "Database loaded with production data!"; \
	else \
		echo "No production data backup found. Run 'make extract-to-dev' first."; \
	fi

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
	@echo "=== Database Data Source ==="
	@if [ -f "init-scripts/01-production-data.sql" ]; then \
		echo "Current: Production data"; \
		ls -la init-scripts/01-production-data.sql; \
	elif [ -f "init-scripts/01-sample-data.sql" ]; then \
		echo "Current: Sample data"; \
		ls -la init-scripts/01-sample-data.sql; \
	else \
		echo "Current: No data source found"; \
	fi
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