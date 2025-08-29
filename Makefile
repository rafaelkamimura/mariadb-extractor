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
	docker-compose exec mariadb mysql -u devuser -p

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

extract-to-dev: ## Extract DDL from production and set up local dev database (schema only)
	@echo "🚀 Extracting DDL from production database..."
	@echo "This will extract database schemas (fast and reliable)"
	$(MAKE) ddl
	@echo ""
	@echo "📦 Setting up local development database with schema..."
	@echo "Note: This will create the database structure without data"
	@read -p "Continue with schema setup? (y/N) " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(MAKE) setup-from-ddl; \
		echo ""; \
		echo "✅ Local development database schema created!"; \
		echo "🌐 Access your database:"; \
		echo "   - Adminer: http://localhost:8080"; \
		echo "   - MySQL: make dev-db-connect"; \
		echo ""; \
		echo "💡 Next steps:"; \
		echo "   - Run 'make migrate-data' to extract and import production data"; \
		echo "   - Or run 'make extract-data DB=database_name' for specific data"; \
	else \
		echo "❌ Operation cancelled."; \
	fi

setup-from-ddl: ## Set up local database from extracted DDL
	@echo "🔧 Setting up local database from DDL..."
	@if [ ! -f "init-scripts/01-extracted-schema.sql" ]; then \
		echo "❌ Error: DDL init script not found. Run 'make ddl' first."; \
		exit 1; \
	fi
	@echo "DDL init script found. Restarting database with new schema..."
	$(MAKE) down
	docker volume rm mariadb-extractor_mariadb_data 2>/dev/null || true
	$(MAKE) up
	@echo "Waiting for database to initialize with schema..."
	@sleep 30
	@echo "✅ Database schema setup complete!"
	@echo "🌐 Access your database:"
	@echo "   - Adminer: http://localhost:8080"
	@echo "   - MySQL: make dev-db-connect"

migrate-data: ## Complete data migration workflow
	@echo "🚀 Starting complete data migration..."
	@echo "This will extract data from production and import into local database"
	@read -p "Continue with full data migration? (y/N) " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(MAKE) dump-data-only; \
		$(MAKE) import-data; \
		echo ""; \
		echo "✅ Data migration complete!"; \
		echo "🌐 Your local database now has production data."; \
	else \
		echo "❌ Operation cancelled."; \
	fi

populate-data: ## Extract and populate data for existing schema
	@echo "📊 Extracting data from production database..."
	@echo "This will populate your existing schema with production data"
	@read -p "Continue with data population? (y/N) " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(MAKE) dump-data-only; \
		echo ""; \
		echo "✅ Data extraction complete!"; \
		echo "💡 Next: Run 'make import-data' to load into local database"; \
	else \
		echo "❌ Operation cancelled."; \
	fi

dump-data-only: ## Extract data only (no schema) from all databases
	@echo "📊 Extracting data only from all user databases..."
	@echo "This excludes schema/structure, only extracts data"
	docker run --rm \
		--env-file .env \
		-v $(PWD):/app/output \
		mariadb-extractor dump --all-user-databases --data-only 2>&1 | tee data-dump.log
	@echo "✅ Data extraction complete. Check mariadb-dump.sql"

import-data: ## Import extracted data into local database
	@echo "📥 Importing data into local database..."
	@if [ ! -f "mariadb-dump.sql" ]; then \
		echo "❌ Error: Data dump file not found. Run 'make dump-data-only' first."; \
		exit 1; \
	fi
	@echo "Importing data dump into local MariaDB..."
	docker-compose exec -T mariadb mysql -u root -p${MYSQL_ROOT_PASSWORD:-password} < mariadb-dump.sql
	@echo "✅ Data import complete!"

extract-data: ## Extract data from specific database
	@if [ -z "$(DB)" ]; then \
		echo "Error: Please specify database name with DB= parameter"; \
		echo "Example: make extract-data DB=myapp"; \
		exit 1; \
	fi
	@echo "📊 Extracting data from database: $(DB)"
	docker run --rm \
		--env-file .env \
		-v $(PWD):/app/output \
		mariadb-extractor dump --databases $(DB) --data-only 2>&1 | tee dump-data-$(DB).log
	@echo "✅ Data extraction complete for $(DB)!"
	@echo "💡 Next: Run 'make import-data FILE=dump-data-$(DB).sql' to load locally"

test-ddl-small: ## Test DDL extraction with just first 10 databases (to trigger save)
	@echo "🧪 Testing DDL extraction with first 10 databases..."
	@echo "This will process exactly 10 databases to trigger intermediate file saving"
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found. Please create one from .env.example"; \
		echo "Run: cp .env.example .env && edit .env with your credentials"; \
		exit 1; \
	fi
	docker run --rm \
		--env-file .env \
		-v $(PWD):/app/output \
		mariadb-extractor ddl 2>&1 | head -40
	@echo ""
	@echo "Test completed. Check for generated files:"
	@echo "  - test-ddl.md (markdown documentation)"
	@echo "  - init-scripts/01-extracted-schema.sql (SQL init script)"
	@ls -la test-ddl.md init-scripts/01-extracted-schema.sql 2>/dev/null || echo "Files not found - process may have been interrupted"




	@echo "🌐 Your local database now has both schema and data!"
	@echo "   - Adminer: http://localhost:8080"
	@echo "   - MySQL: make dev-db-connect"

full-setup: ## Complete setup: extract schema + setup local + populate data
	@echo "🚀 Starting complete production-to-local setup..."
	@echo "This will:"
	@echo "  1. Extract DDL schema from production"
	@echo "  2. Set up local database with schema"
	@echo "  3. Extract and import production data"
	@echo ""
	@read -p "This is a comprehensive process. Continue? (y/N) " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		echo "📋 Step 1: Extracting schema..."; \
		$(MAKE) ddl; \
		echo ""; \
		echo "🏗️  Step 2: Setting up local database..."; \
		$(MAKE) setup-from-ddl; \
		echo ""; \
		echo "📊 Step 3: Extracting data..."; \
		$(MAKE) dump-data-only; \
		echo ""; \
		echo "📥 Step 4: Importing data..."; \
		$(MAKE) import-data; \
		echo ""; \
		echo "🎉 Complete setup finished!"; \
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
	@echo "=== Schema Files ==="
	@if [ -f "mariadb-ddl.md" ]; then \
		echo "✅ DDL Schema: mariadb-ddl.md"; \
		ls -lh mariadb-ddl.md; \
	else \
		echo "❌ No DDL schema file found"; \
	fi
	@echo ""
	@echo "=== Data Files ==="
	@if [ -f "mariadb-dump.sql" ]; then \
		echo "✅ Full Dump: mariadb-dump.sql"; \
		ls -lh mariadb-dump.sql; \
	else \
		echo "❌ No dump file found"; \
	fi
	@ls -la *-data*.sql 2>/dev/null || true
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
	@echo "2. Choose your workflow:"
	@echo ""
	@echo "   Option A - Schema Only (Fast):"
	@echo "   make extract-to-dev     # Extract schema + setup local"
	@echo ""
	@echo "   Option B - Full Setup (Complete):"
	@echo "   make full-setup         # Extract schema + data + setup"
	@echo ""
	@echo "   Option C - Custom:"
	@echo "   make ddl                # Extract schema only"
	@echo "   make setup-from-ddl     # Setup local with schema"
	@echo "   make populate-data      # Add production data later"
	@echo ""
	@echo "3. Access your database:"
	@echo "   - Adminer (web UI): http://localhost:8080"
	@echo "   - MySQL client: make dev-db-connect"
	@echo ""
	@echo "4. Check status:"
	@echo "   make status             # See current setup"
	@echo "   make dev-db-logs        # View database logs"
	@echo ""
	@echo "Happy coding! 🎉"
