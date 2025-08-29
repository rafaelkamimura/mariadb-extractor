#!/bin/bash

# MariaDB Extractor - Local Development Setup
# This script extracts DDL from an external database and sets up a local MariaDB container

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to show usage
show_usage() {
    echo "MariaDB Extractor - Local Development Setup"
    echo ""
    echo "This script extracts DDL from an external database and sets up a local MariaDB container."
    echo ""
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --databases DB1,DB2    Extract specific databases (default: all databases)"
    echo "  --schema-only          Extract schema only (no data)"
    echo "  --help                 Show this help message"
    echo ""
    echo "Environment Variables (set these before running):"
    echo "  MARIADB_HOST           External database host"
    echo "  MARIADB_PORT           External database port (default: 3306)"
    echo "  MARIADB_USER           External database username"
    echo "  MARIADB_PASSWORD       External database password"
    echo ""
    echo "Examples:"
    echo "  # Extract all databases from external DB"
    echo "  export MARIADB_HOST=prod-db.example.com"
    echo "  export MARIADB_USER=myuser"
    echo "  export MARIADB_PASSWORD=mypass"
    echo "  $0"
    echo ""
    echo "  # Extract specific databases"
    echo "  $0 --databases myapp,analytics"
    echo ""
    echo "  # Extract schema only"
    echo "  $0 --schema-only"
}

# Function to check if Docker is running
check_docker() {
    if ! docker info &> /dev/null; then
        print_error "Docker is not running or not accessible"
        print_info "Please start Docker and try again"
        exit 1
    fi
}

# Function to check environment variables
check_env_vars() {
    local missing_vars=()

    if [[ -z "${MARIADB_HOST}" ]]; then
        missing_vars+=("MARIADB_HOST")
    fi

    if [[ -z "${MARIADB_USER}" ]]; then
        missing_vars+=("MARIADB_USER")
    fi

    if [[ -z "${MARIADB_PASSWORD}" ]]; then
        missing_vars+=("MARIADB_PASSWORD")
    fi

    if [[ ${#missing_vars[@]} -gt 0 ]]; then
        print_error "Missing required environment variables:"
        printf '  %s\n' "${missing_vars[@]}"
        echo ""
        print_info "Please set these environment variables and try again:"
        echo "  export MARIADB_HOST=your-external-db-host"
        echo "  export MARIADB_USER=your-username"
        echo "  export MARIADB_PASSWORD=your-password"
        exit 1
    fi
}

# Function to extract DDL and convert to init script
extract_and_setup() {
    local databases="$1"
    local schema_only="$2"

    print_info "Starting extraction from external database..."
    print_info "Host: $MARIADB_HOST:$MARIADB_PORT"
    print_info "User: $MARIADB_USER"

    # Create init-scripts directory if it doesn't exist
    mkdir -p init-scripts

    # Backup existing init scripts
    if [[ -f "init-scripts/01-sample-data.sql" ]]; then
        print_warning "Backing up existing init-scripts/01-sample-data.sql"
        mv init-scripts/01-sample-data.sql init-scripts/01-sample-data.sql.backup
    fi

    # Build extraction command
    local extract_cmd="./mariadb-extractor ddl"

    if [[ -n "$databases" ]]; then
        extract_cmd="$extract_cmd --databases $databases"
    fi

    # Set output to init-scripts directory
    extract_cmd="$extract_cmd --output init-scripts/01-extracted-schema"

    print_info "Running: $extract_cmd"

    # Run the extraction
    if ! eval "$extract_cmd"; then
        print_error "Failed to extract DDL from external database"
        exit 1
    fi

    # Convert the markdown DDL output to SQL init script
    convert_ddl_to_sql "$schema_only"

    print_success "External database schema extracted successfully!"
    print_info "Generated: init-scripts/01-extracted-schema.sql"
}

# Function to convert markdown DDL to SQL init script
convert_ddl_to_sql() {
    local schema_only="$1"
    local ddl_file="init-scripts/01-extracted-schema.md"
    local sql_file="init-scripts/01-extracted-schema.sql"

    if [[ ! -f "$ddl_file" ]]; then
        print_error "DDL file not found: $ddl_file"
        exit 1
    fi

    print_info "Converting DDL markdown to SQL init script..."

    # Extract SQL statements from markdown and create init script
    {
        echo "-- Auto-generated from external database extraction"
        echo "-- Generated on: $(date)"
        echo "-- Source: $MARIADB_HOST:$MARIADB_PORT"
        echo ""

        # Extract SQL code blocks from markdown
        awk '
        BEGIN { in_sql = 0 }
        /^```sql$/ { in_sql = 1; next }
        /^```$/ { in_sql = 0; next }
        in_sql { print }
        ' "$ddl_file"

        # If not schema-only, we could add data extraction here in the future
        if [[ "$schema_only" != "true" ]]; then
            echo ""
            echo "-- Note: This init script contains schema only."
            echo "-- For data migration, use the dump command separately."
        fi
    } > "$sql_file"

    print_success "SQL init script created: $sql_file"
}

# Function to start local MariaDB container
start_local_db() {
    print_info "Starting local MariaDB container..."

    # Stop any existing containers
    docker-compose down 2>/dev/null || true

    # Start the local database
    if ! docker-compose --profile local-db up -d mariadb; then
        print_error "Failed to start local MariaDB container"
        exit 1
    fi

    print_success "Local MariaDB container started!"
    print_info "Container: mariadb-local"
    print_info "Port: 3306"
    print_info "Root Password: ${MYSQL_ROOT_PASSWORD:-rootpassword}"
    print_info "Database: ${MYSQL_DATABASE:-testdb}"
    print_info "User: ${MYSQL_USER:-testuser}"
    print_info "Password: ${MYSQL_PASSWORD:-testpass}"

    # Wait for database to be ready
    print_info "Waiting for database to initialize..."
    sleep 10

    # Check if database is ready
    if docker-compose exec -T mariadb mysqladmin ping -h localhost --silent; then
        print_success "Local MariaDB is ready!"
        print_info "You can now connect to: mysql -h localhost -P 3306 -u ${MYSQL_USER:-testuser} -p${MYSQL_PASSWORD:-testpass}"
    else
        print_warning "Database may still be initializing. Check logs with: docker-compose logs mariadb"
    fi
}

# Function to show status
show_status() {
    echo ""
    print_info "Local Development Setup Complete!"
    echo ""
    echo "📁 Generated Files:"
    echo "  - init-scripts/01-extracted-schema.md (DDL documentation)"
    echo "  - init-scripts/01-extracted-schema.sql (Database init script)"
    echo ""
    echo "🐳 Docker Services:"
    echo "  - mariadb-local: Local MariaDB container on port 3306"
    echo ""
    echo "🔗 Connection Details:"
    echo "  Host: localhost"
    echo "  Port: 3306"
    echo "  Root Password: ${MYSQL_ROOT_PASSWORD:-rootpassword}"
    echo "  Database: ${MYSQL_DATABASE:-testdb}"
    echo "  User: ${MYSQL_USER:-testuser}"
    echo "  Password: ${MYSQL_PASSWORD:-testpass}"
    echo ""
    echo "💻 Usage Examples:"
    echo "  # Connect to local database"
    echo "  mysql -h localhost -P 3306 -u ${MYSQL_USER:-testuser} -p${MYSQL_PASSWORD:-testpass}"
    echo ""
    echo "  # View container logs"
    echo "  docker-compose logs mariadb"
    echo ""
    echo "  # Stop local database"
    echo "  docker-compose down"
}

# Main script logic
main() {
    local databases=""
    local schema_only="false"

    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --databases)
                databases="$2"
                shift 2
                ;;
            --schema-only)
                schema_only="true"
                shift
                ;;
            --help|-h)
                show_usage
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    print_info "MariaDB Extractor - Local Development Setup"
    echo ""

    # Check prerequisites
    check_docker
    check_env_vars

    # Extract DDL from external database
    extract_and_setup "$databases" "$schema_only"

    # Start local MariaDB container
    start_local_db

    # Show status
    show_status
}

# Run main function with all arguments
main "$@"