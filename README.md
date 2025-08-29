# MariaDB Extractor

A comprehensive CLI tool for extracting database information from MariaDB servers. Provides multiple extraction methods including metadata, DDL statements, and full database dumps for development, documentation, and backup purposes.

## Features

- **Extract Command**: Extract database names and metadata from MariaDB servers
- **DDL Command**: Retrieve complete CREATE TABLE statements for all tables
- **Dump Command**: Create full database dumps using mysqldump for local development
- Generate formatted markdown reports for documentation
- Create structured JSON output for automation and integration
- Support for custom connection parameters via CLI flags or environment variables
- Flexible output options including compression support

## Installation

### Option 1: Native Installation

```bash
go build -o mariadb-extractor
```

### Option 2: Docker (Recommended)

```bash
# Build the Docker image
make build

# Or build manually
docker build -t mariadb-extractor .
```

## Quick Start with Docker

```bash
# 1. Set up environment
make env-example
# Edit .env with your database credentials

# 2. Start development environment
make setup-dev

# 3. Access your database
# - Adminer (web UI): http://localhost:8080
# - MySQL client: make dev-db-connect

# 4. Extract from production
make extract
make ddl
make dump
```

## Makefile Commands

The project includes a comprehensive Makefile for common operations:

### Development Environment
- `make setup-dev` - Set up complete development environment
- `make up` - Start local MariaDB and Adminer
- `make down` - Stop all services
- `make dev-db-connect` - Connect to local database

### Data Extraction
- `make extract` - Extract metadata from configured server
- `make ddl` - Extract DDL statements from configured server
- `make dump` - Create full dump from configured server
- `make extract-local` - Extract from local development database

### Workflow Commands
- `make extract-to-dev` - Extract from production and update local dev
- `make backup-local` - Create backup of local development database

### Utility Commands
- `make build` - Build Docker image
- `make clean` - Clean up generated files and containers
- `make status` - Show status of all services
- `make help` - Show all available commands

### Option 2: Docker (Recommended)

The easiest way to run MariaDB Extractor is using Docker, which includes all dependencies:

```bash
# Clone the repository
git clone <repository-url>
cd mariadb-extractor

# Build and run with Docker
./docker-run.sh extract --help
```

Or use Docker Compose for more complex setups:

```bash
# Start with local MariaDB for testing
docker-compose --profile local-db up -d

# Run the extractor
docker-compose run --rm mariadb-extractor extract --all-databases
```

### Option 2: Docker (Recommended)

The easiest way to use MariaDB Extractor is with Docker, which includes all dependencies:

```bash
# Clone the repository
git clone <repository-url>
cd mariadb-extractor

# Make the Docker runner script executable
chmod +x docker-run.sh

# Build the Docker image
./docker-run.sh build
```

**Note:** Docker installation eliminates the need to install MariaDB client tools on your local machine.

## Configuration

The application supports three ways to provide configuration (in order of precedence):
1. Command-line flags (highest priority)
2. Environment variables 
3. `.env` file (lowest priority)

### Using .env File

1. Copy the example configuration:
```bash
cp .env.example .env
```

2. Edit `.env` with your database credentials:
```env
MARIADB_HOST=localhost
MARIADB_PORT=3306
MARIADB_USER=your_username
MARIADB_PASSWORD=your_password
MARIADB_OUTPUT_PREFIX=mariadb-extract
```

3. Run the extractor:
```bash
./mariadb-extractor extract
```

### Docker Configuration

When using Docker, you can configure the tool the same way:

1. **Using .env file** (recommended):
```bash
# Your .env file will be automatically loaded
./docker-run.sh extract
```

2. **Using environment variables**:
```bash
export MARIADB_HOST=your-host
export MARIADB_USER=your-user
export MARIADB_PASSWORD=your-password
./docker-run.sh extract
```

3. **Using command-line flags**:
```bash
./docker-run.sh extract -H your-host -u your-user -p your-password
```

## Commands

The tool provides three main commands:

### Extract Command
Extract database and table metadata:
```bash
./mariadb-extractor extract --host localhost --port 3306 --user your_username --password your_password --output my-extraction
```

### DDL Command
Extract CREATE TABLE statements for all tables:
```bash
./mariadb-extractor ddl --host localhost --port 3306 --user your_username --password your_password --output my-ddl
```

### Dump Command
Create full database dumps for local development:
```bash
./mariadb-extractor dump --all-databases --host localhost --port 3306 --user your_username --password your_password
```

### Docker Usage

Using Docker is the recommended approach as it includes all dependencies:

```bash
# Extract database metadata
./docker-run.sh extract --all-databases

# Extract DDL statements
./docker-run.sh ddl --all-databases

# Create database dump
./docker-run.sh dump --all-databases --compress

# Get help
./docker-run.sh help

# Start interactive shell
./docker-run.sh shell
```

### Docker Benefits

- **No local dependencies**: MariaDB client tools are included in the container
- **OS agnostic**: Works on Windows, macOS, and Linux
- **Isolated environment**: No conflicts with local system tools
- **Easy distribution**: Share the same environment across your team
- **Version consistency**: Everyone uses the same tool versions

### Docker Compose (Advanced)

For more complex setups, you can use Docker Compose:

```bash
# Start with local MariaDB for testing
docker-compose --profile local-db up -d

# Run extraction against local database
docker-compose run --rm mariadb-extractor extract

# Stop local database
docker-compose --profile local-db down
```

### Environment Variables

The tool supports the following environment variables for configuration:

- `MARIADB_HOST`: MariaDB server host (default: localhost)
- `MARIADB_PORT`: MariaDB server port (default: 3306)
- `MARIADB_USER`: MariaDB username
- `MARIADB_PASSWORD`: MariaDB password
- `MARIADB_OUTPUT_PREFIX`: Output file prefix (default: mariadb-extract)

Environment variables are overridden by command-line flags if both are provided.

### Command Line Options

- `-H, --host`: MariaDB host (env: MARIADB_HOST, default: localhost)
- `-P, --port`: MariaDB port (env: MARIADB_PORT, default: 3306)
- `-u, --user`: MariaDB username (env: MARIADB_USER, required if not in env)
- `-p, --password`: MariaDB password (env: MARIADB_PASSWORD, required if not in env)
- `-o, --output`: Output file prefix (env: MARIADB_OUTPUT_PREFIX, default: mariadb-extract)

### Examples

#### Extract Command Examples

**Using .env File:**
```bash
# Create your .env file
cat > .env << EOF
MARIADB_HOST=db.example.com
MARIADB_USER=myuser
MARIADB_PASSWORD=mypassword
MARIADB_OUTPUT_PREFIX=production
EOF

# Run the extractor (will automatically load .env)
./mariadb-extractor extract
```

**Using Environment Variables:**
```bash
export MARIADB_HOST=db.example.com
export MARIADB_PORT=3306
export MARIADB_USER=myuser
export MARIADB_PASSWORD=mypassword
export MARIADB_OUTPUT_PREFIX=production

./mariadb-extractor extract
```

**Mix Environment Variables and CLI Flags:**
```bash
export MARIADB_USER=myuser
export MARIADB_PASSWORD=mypassword

./mariadb-extractor extract -H dev-db.example.com -o dev-extraction
```

**Traditional CLI Usage:**
```bash
./mariadb-extractor extract -u root -p mypassword
./mariadb-extractor extract -H db.example.com -P 3307 -u admin -p secret -o production-db
```

#### DDL Command Examples

**Extract DDL for all databases:**
```bash
./mariadb-extractor ddl --all-databases
```

**Extract DDL for specific databases:**
```bash
./mariadb-extractor ddl --databases mydb,otherdb
```

**Extract DDL with custom output:**
```bash
./mariadb-extractor ddl -H db.example.com -u admin -p secret -o schema-docs
```

#### Dump Command Examples

**Dump all databases (schema + data):**
```bash
./mariadb-extractor dump --all-databases
```

**Dump specific databases:**
```bash
./mariadb-extractor dump --databases mydb,otherdb
```

**Schema-only dump:**
```bash
./mariadb-extractor dump --all-databases --schema-only
```

**Data-only dump:**
```bash
./mariadb-extractor dump --databases mydb --data-only
```

**Compressed dump:**
```bash
./mariadb-extractor dump --all-databases --compress
```

#### CI/CD Pipeline Example

```yaml
# Example GitHub Actions or CI configuration
env:
  MARIADB_HOST: ${{ secrets.DB_HOST }}
  MARIADB_USER: ${{ secrets.DB_USER }}
  MARIADB_PASSWORD: ${{ secrets.DB_PASSWORD }}

script:
  # Extract metadata for documentation
  - ./mariadb-extractor extract -o nightly-docs
  # Extract DDL for schema documentation
  - ./mariadb-extractor ddl -o nightly-schema
  # Create backup dump
  - ./mariadb-extractor dump --all-databases --compress
```

## Local Development Setup

The tool includes Docker support for easy local development. Extract schema from your production database and run it locally with a single command.

### Quick Start with Docker

1. **Set up environment variables:**
```bash
# Copy and edit the local development config
cp .env.local.example .env.local

# Edit .env.local with your production database credentials
# MARIADB_HOST=your-production-db.example.com
# MARIADB_USER=extraction-user
# MARIADB_PASSWORD=extraction-password
```

2. **Extract and setup local database:**
```bash
# Load your environment variables
export $(cat .env.local | xargs)

# Run the setup script (extracts from external DB and starts local container)
./setup-local-db.sh
```

3. **Connect to your local database:**
```bash
mysql -h localhost -P 3306 -u local-dev-user -plocal-dev-password
```

### Local Development Workflow

```
Production DB → Extract DDL → Init Scripts → Local MariaDB Container
```

**Step 1: Extract from Production**
```bash
# Extract all databases
./setup-local-db.sh

# Extract specific databases
./setup-local-db.sh --databases myapp,userdb

# Extract schema only (no sample data)
./setup-local-db.sh --schema-only
```

**Step 2: Local Development**
```bash
# Start local database
docker-compose --profile local-db up -d mariadb

# Connect and develop
mysql -h localhost -P 3306 -u local-dev-user -plocal-dev-password

# View logs
docker-compose logs mariadb

# Stop when done
docker-compose down
```

### Docker Commands

**Using Docker directly:**
```bash
# Build the image
docker build -t mariadb-extractor .

# Run extraction
docker run --rm -it \
  --env-file .env \
  -v $(pwd):/app/output \
  mariadb-extractor ddl --all-databases
```

**Using the helper script:**
```bash
# All commands work with Docker
./docker-run.sh extract --all-databases
./docker-run.sh ddl --databases mydb
./docker-run.sh dump --all-databases --compress
```

## Docker Usage

Docker provides the easiest way to run MariaDB Extractor with all dependencies included.

### Quick Start with Docker

```bash
# Build and run (auto-builds if needed)
./docker-run.sh extract --all-databases

# Or run specific commands
./docker-run.sh ddl --databases mydb
./docker-run.sh dump --all-databases --compress
```

### Docker Environment Variables

Set your database credentials:

```bash
# Using environment variables
export MARIADB_HOST=localhost
export MARIADB_PORT=3306
export MARIADB_USER=myuser
export MARIADB_PASSWORD=mypass

./docker-run.sh extract
```

Or create a `.env` file:
```env
MARIADB_HOST=localhost
MARIADB_PORT=3306
MARIADB_USER=myuser
MARIADB_PASSWORD=mypass
MARIADB_OUTPUT_PREFIX=my-extraction
```

### Local Development with Docker Compose

For local testing, use the included MariaDB container:

```bash
# Start MariaDB with sample data
docker-compose --profile local-db up -d

# Wait for MariaDB to be ready
sleep 10

# Run extractor against local database
docker-compose run --rm mariadb-extractor extract --all-databases

# Stop the database
docker-compose --profile local-db down
```

The local MariaDB includes sample databases:
- `ecommerce` - E-commerce site with users, products, orders
- `blog` - Blog system with posts and categories
- `empty_db` - Empty database for testing
- `test_system_db` - Additional test database

### Docker Commands

```bash
# Build the Docker image
./docker-run.sh build

# Start interactive shell in container
./docker-run.sh shell

# Get help
./docker-run.sh help
```

### Docker Compose Examples

```bash
# Extract metadata from local database
docker-compose run --rm mariadb-extractor extract -u testuser -p testpass

# Extract DDL from specific databases
docker-compose run --rm mariadb-extractor ddl --databases ecommerce,blog

# Create compressed dump
docker-compose run --rm mariadb-extractor dump --all-databases --compress
```

## Development Workflow

### 1. Extract from Production
Use the mariadb-extractor to extract data from your production/external database:

```bash
# Extract metadata for documentation
make extract

# Extract DDL for schema management
make ddl

# Create full backup for local development
make dump
```

### 2. Set up Local Development
The extracted data is automatically used to populate your local MariaDB instance:

```bash
# Start local development environment
make setup-dev

# The init-scripts/ directory is mounted as Docker init scripts
# Your extracted data will be automatically loaded into the local database
```

### 3. Update Local with Production Data
When you need to refresh your local development database with fresh production data:

```bash
# Extract from production and update local dev
make extract-to-dev
```

## Command Comparison

Choose the right command for your use case:

| Command | Purpose | Output | Use Case |
|---------|---------|--------|----------|
| `extract` | Database metadata | Markdown + JSON | Documentation, inventory, monitoring |
| `ddl` | Table schemas | Markdown with SQL | Schema documentation, version control |
| `dump` | Full database | SQL dump | Local development, backup, migration |

## Output Files

The tool generates different output files depending on the command used:

### Extract Command Output

**Markdown Report (.md):**
- Human-readable documentation
- Formatted tables with database and table information
- Summary statistics and metadata

**JSON Data (.json):**
- Machine-readable structured data
- Comprehensive metadata including extraction timestamp
- Suitable for automation and further processing

### DDL Command Output

**DDL Markdown (.md):**
- Complete CREATE TABLE statements for all tables
- Organized by database with syntax-highlighted SQL
- Perfect for schema documentation and version control

### Dump Command Output

**SQL Dump (.sql):**
- Complete database dump compatible with mysql client
- Can be imported locally with: `mysql -u root -p < dump.sql`
- Supports schema-only, data-only, or full dumps

**Compressed SQL Dump (.sql.gz):**
- Gzipped version of the SQL dump for smaller file sizes
- Can be imported with: `gunzip < dump.sql.gz | mysql -u root -p`

## Output Structure

### JSON Format
```json
{
  "metadata": {
    "server": "localhost:3306",
    "user": "root",
    "extracted_at": "2025-01-27T10:30:00Z",
    "total_databases": 5
  },
  "databases": [
    {
      "name": "mydatabase",
      "table_count": 12,
      "tables": [
        {
          "name": "users",
          "type": "BASE TABLE",
          "engine": "InnoDB",
          "row_count": 1000,
          "data_length": 16384,
          "index_length": 8192,
          "collation": "utf8mb4_unicode_ci",
          "comment": "User accounts table"
        }
      ],
      "extracted_at": "2025-01-27T10:30:00Z"
    }
  ]
}
```

## Requirements

### For Docker Installation (Recommended)
- Docker Engine 20.10 or later
- Access to MariaDB/MySQL server
- **No additional dependencies required!**

All dependencies (Go, MariaDB client tools, mysqldump, etc.) are included in the Docker container.

### For Source Installation
- Go 1.19 or later
- Access to MariaDB/MySQL server
- MySQL client libraries (handled by go-sql-driver/mysql)

#### Additional Requirements for Dump Command (Source Installation Only)

If building from source and using the `dump` command, you'll need the `mysqldump` utility:

**Ubuntu/Debian:**
```bash
sudo apt-get install mariadb-client
```

**CentOS/RHEL:**
```bash
sudo yum install mariadb
```

**macOS:**
```bash
brew install mariadb
```

**Windows:**
Download and install from: https://mariadb.com/downloads/

**Note:** The `extract` and `ddl` commands only require Go/Docker and database access. The `dump` command requires `mysqldump` when building from source.

## License

This project is open source and available under the MIT License.