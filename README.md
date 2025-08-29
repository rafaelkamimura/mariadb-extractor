# MariaDB Extractor

A comprehensive CLI tool for extracting database schemas and data from MariaDB servers. Designed for efficient database migration, development environment setup, and backup operations with advanced features including foreign key preservation, selective data extraction, and progressive processing.

## Features

### Core Commands

- **Extract**: Database metadata extraction with detailed table information
- **DDL**: Complete schema extraction with CREATE TABLE statements
- **Dump**: Traditional full database backup using mysqldump
- **Data**: Advanced selective data extraction with foreign key preservation
- **Query**: Safe read-only query execution with MCP-compatible output
- **MCP**: Model Context Protocol server for Claude Desktop and Claude Code integration

### Key Capabilities

- Foreign key dependency resolution with topological sorting
- Configurable data sampling (percentage or fixed row counts)
- Progressive extraction with resume capability for large datasets
- Pattern-based table filtering and exclusion
- Batch processing for optimal performance
- Docker-based execution with zero local dependencies

## Installation

### Docker Installation (Recommended)

```bash
# Clone the repository
git clone <repository-url>
cd mariadb-extractor

# Build the Docker image
make build
```

### Native Installation

```bash
# Requires Go 1.25 or later
go build -o mariadb-extractor
```

## Quick Start

### 1. Configure Database Connection

```bash
# Create configuration from template
make env-example

# Edit .env with your database credentials
vim .env
```

Example `.env` configuration:
```env
MARIADB_HOST=your-database-host
MARIADB_PORT=3306
MARIADB_USER=your-username
MARIADB_PASSWORD=your-password
MARIADB_OUTPUT_PREFIX=extraction
```

### 2. Run Complete Pipeline

```bash
# Recommended: Complete pipeline with sample data (fastest)
make pipeline

# Alternative: Full data extraction
make pipeline-full

# Custom: Specific databases and tables
make pipeline-custom ARGS="--databases db1,db2 --sample-tables users:1000"
```

## MCP Server Integration (New)

The MariaDB Extractor can run as an MCP (Model Context Protocol) server, enabling direct integration with Claude Desktop and Claude Code for AI-assisted database operations.

### Quick Setup for Claude

```bash
# 1. Build the tool
go build -o mariadb-extractor

# 2. Add to Claude Desktop config (~/Library/Application Support/Claude/claude_desktop_config.json)
{
  "mcpServers": {
    "mariadb": {
      "command": "/path/to/mariadb-extractor/mcp-start.sh",
      "args": [],
      "env": {
        "MARIADB_HOST": "localhost",
        "MARIADB_PORT": "3306",
        "MARIADB_USER": "username",
        "MARIADB_PASSWORD": "password"
      }
    }
  }
}

# 3. Restart Claude Desktop
```

### Available MCP Tools

- **query_database**: Execute safe SQL queries
- **get_table_schema**: Get table structure information
- **get_foreign_keys**: Discover foreign key relationships
- **list_databases**: List all available databases
- **list_tables**: List tables with statistics

See [MCP_SETUP.md](MCP_SETUP.md) for detailed configuration instructions.

## Command Reference

### Query Command

The `query` command provides safe, read-only query execution with comprehensive security features:

```bash
# Execute a simple query
./mariadb-extractor query -q "SELECT * FROM users LIMIT 10" -d mydb

# Query with JSON output for MCP compatibility
./mariadb-extractor query -q "SHOW TABLES" -d mydb -f json

# Query from file
./mariadb-extractor query -F queries.sql -d mydb

# Query foreign key relationships
./mariadb-extractor query -q "SELECT * FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_NAME IS NOT NULL" -d mydb
```

#### Query Command Options

| Flag | Description | Default |
|------|-------------|---------|
| `-q, --query` | SQL query to execute | - |
| `-F, --file` | File containing SQL query | - |
| `-d, --database` | Database to use | - |
| `-f, --format` | Output format (json, markdown, csv) | markdown |
| `-l, --limit` | Maximum rows to return | 1000 |
| `-t, --timeout` | Query timeout in seconds | 30 |
| `--no-redact` | Disable automatic PII redaction | false |
| `--audit-log` | Path to audit log file | - |
| `--rate-limit` | Max queries per second | 5 |
| `--max-concurrent` | Max concurrent queries | 2 |

#### Security Features

- **Query Validation**: Only SELECT, SHOW, DESCRIBE, and EXPLAIN queries allowed
- **SQL Injection Prevention**: Comprehensive pattern blocking and validation
- **Rate Limiting**: Configurable per-second and concurrent query limits
- **Audit Logging**: Optional JSON-formatted audit trail
- **PII Redaction**: Automatic redaction of emails, phone numbers, SSNs
- **Timeout Protection**: Configurable query execution timeout

#### Makefile Query Targets

```bash
# Basic query execution
make query Q="SELECT * FROM users" DB=mydb

# JSON output for MCP integration
make query-json Q="SHOW TABLES" DB=mydb

# CSV output for data export
make query-csv Q="SELECT id, name FROM users" DB=mydb

# Query foreign key relationships
make query-relationships DB=mydb

# Query table information
make query-table-info DB=mydb

# Query with audit logging
make query-audit Q="SELECT * FROM sensitive_table" DB=mydb
```

### Data Extraction Pipeline

The `data` command provides advanced selective extraction with foreign key preservation:

```bash
# Extract with 10% sampling
./mariadb-extractor data --all-user-databases --sample-percent 10

# Extract specific databases
./mariadb-extractor data --databases db1,db2 --exclude-tables "*_log,*_audit"

# Custom sampling per table
./mariadb-extractor data \
  --databases myapp \
  --sample-tables "users:1000,orders:5000" \
  --exclude-tables "*_history"

# Resume interrupted extraction
./mariadb-extractor data --resume extraction-id
```

#### Data Command Options

| Flag | Description | Default |
|------|-------------|---------|
| `--all-user-databases` | Extract all non-system databases | - |
| `--databases` | Comma-separated list of databases | - |
| `--exclude-tables` | Pattern-based table exclusion | - |
| `--sample-percent` | Global sampling percentage (0-100) | 0 |
| `--sample-tables` | Per-table row limits (table:count) | - |
| `--chunk-size` | Rows per chunk for large tables | 10000 |
| `--batch-size` | INSERT statement batch size | 100 |
| `--timeout` | Query timeout in seconds | 300 |
| `--resume` | Resume from previous extraction | - |

### DDL Extraction

Extract complete database schemas:

```bash
# Extract all user databases
./mariadb-extractor ddl --all-user-databases

# Extract specific databases
./mariadb-extractor ddl --databases db1,db2
```

Output:
- `output/mariadb-ddl.md` - Formatted documentation
- `output/init-scripts/01-extracted-schema.sql` - Executable SQL script

### Traditional Dump

Full database backup using mysqldump:

```bash
# Dump all user databases
./mariadb-extractor dump --all-user-databases

# Schema only
./mariadb-extractor dump --all-databases --schema-only

# Compressed output
./mariadb-extractor dump --all-databases --compress
```

### Metadata Extract

Extract database and table metadata:

```bash
# Extract all databases
./mariadb-extractor extract --all-databases

# Generate JSON output
./mariadb-extractor extract --output metadata
```

## Makefile Targets

### Pipeline Commands

| Target | Description |
|--------|-------------|
| `make pipeline` | Complete pipeline with 10% sample data |
| `make pipeline-full` | Complete pipeline with full data |
| `make pipeline-custom` | Pipeline with custom extraction parameters |

### Individual Steps

| Target | Description |
|--------|-------------|
| `make ddl` | Extract database schemas |
| `make setup-from-ddl` | Initialize local database with schema |
| `make extract-data-sample` | Extract 10% sample data |
| `make extract-data-full` | Extract complete data |
| `make seed-dev-data` | Import extracted data to local database |

### Development Environment

| Target | Description |
|--------|-------------|
| `make setup-dev` | Start local MariaDB and Adminer |
| `make dev-db-connect` | Connect to local database via CLI |
| `make status` | Show service and file status |
| `make clean` | Clean generated files and containers |

## Workflow Examples

### Development Environment Setup

```bash
# 1. Configure connection
cp .env.example .env
vim .env

# 2. Run pipeline (DDL -> Setup -> Extract -> Seed)
make pipeline

# 3. Access database
# Web UI: http://localhost:8080
# CLI: make dev-db-connect
```

### Production Mirror

```bash
# Extract complete production data
make pipeline-full

# Or manually with custom settings
make ddl
make setup-from-ddl
make extract-data-custom ARGS="--databases prod_db --max-rows 50000"
make seed-dev-data
```

### Selective Data Extraction

```bash
# Extract specific tables with sampling
docker run --rm \
  --env-file .env \
  -v $(pwd):/app/output \
  mariadb-extractor data \
    --databases users_db,orders_db \
    --sample-tables "users:10000,orders:50000,products:all" \
    --exclude-tables "*_temp,*_backup"
```

## Architecture

### Project Structure

```
mariadb-extractor/
├── cmd/
│   ├── root.go      # CLI root command
│   ├── extract.go   # Metadata extraction
│   ├── ddl.go       # Schema extraction
│   ├── dump.go      # Full backup
│   ├── data.go      # Selective data extraction
│   └── query.go     # Safe query execution
├── internal/
│   └── config/
│       └── env.go   # Environment configuration
├── output/          # Generated files
│   └── init-scripts/
│       └── *.sql    # Database initialization scripts
└── docker-compose.yml
```

### Data Extraction Pipeline

The `data` command implements a sophisticated extraction pipeline:

1. **Schema Analysis**: Discovers foreign key relationships
2. **Dependency Resolution**: Topological sort for correct table ordering
3. **Extraction Planning**: Optimizes based on table sizes and sampling
4. **Progressive Extraction**: Chunks large tables with progress tracking
5. **Data Generation**: Creates optimized INSERT statements

### Foreign Key Handling

- Automatic dependency detection via `information_schema`
- Topological sorting ensures correct extraction order
- `SET FOREIGN_KEY_CHECKS=0/1` wrapper for safe imports
- Preserves referential integrity across sampled data

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MARIADB_HOST` | Database host | localhost |
| `MARIADB_PORT` | Database port | 3306 |
| `MARIADB_USER` | Database username | - |
| `MARIADB_PASSWORD` | Database password | - |
| `MARIADB_OUTPUT_PREFIX` | Output file prefix | mariadb-extract |
| `MARIADB_TIMEOUT` | Query timeout (seconds) | 300 |
| `MARIADB_CHUNK_SIZE` | Rows per chunk | 10000 |
| `MARIADB_BATCH_SIZE` | Batch insert size | 100 |

### Docker Compose Services

- **MariaDB**: Local database instance (port 3307)
- **Adminer**: Web-based database management (port 8080)
- **Extractor**: Main application container

## Performance Considerations

### Large Database Optimization

- **Chunked Processing**: Configurable chunk size for memory efficiency
- **Batch Inserts**: Reduces I/O with configurable batch sizes
- **Progress Tracking**: Resume capability for interrupted extractions
- **Connection Pooling**: Optimized database connections

### Sampling Strategies

- **Percentage Sampling**: Consistent sampling across all tables
- **Fixed Row Counts**: Specific limits per table
- **Pattern Exclusion**: Skip log, audit, and temporary tables
- **Foreign Key Preservation**: Maintains relationships in sampled data

## Output Files

### DDL Extraction

- `output/mariadb-ddl.md`: Human-readable schema documentation
- `output/init-scripts/01-extracted-schema.sql`: Complete DDL statements

### Data Extraction

- `output/data-extract.sql`: INSERT statements with data
- `data-extract.progress`: Resume tracking file

### Metadata Extraction

- `output/mariadb-extract.md`: Formatted database information
- `output/mariadb-extract.json`: Structured metadata

## Troubleshooting

### Common Issues

**Connection Timeout**
```bash
# Increase timeout
export MARIADB_TIMEOUT=600
```

**Foreign Key Errors**
- Handled automatically with `SET FOREIGN_KEY_CHECKS=0`
- Tables extracted in dependency order

**Large Dataset Memory Issues**
```bash
# Reduce chunk size
export MARIADB_CHUNK_SIZE=5000
```

**Resume Failed Extraction**
```bash
# Check progress file
ls *.progress

# Resume with ID
make extract-data-resume
```

## Development

### Building from Source

```bash
# Install dependencies
go mod download

# Build binary
go build -o mariadb-extractor

# Run tests
go test ./...
```

### Docker Development

```bash
# Build image
docker build -t mariadb-extractor .

# Run with local code
docker run --rm -v $(pwd):/app/output mariadb-extractor --help
```

## Security

- No hardcoded credentials in source code
- Environment-based configuration
- Secure password handling via temporary config files
- Optional table exclusion for sensitive data
- Pattern-based filtering for PII protection

## Requirements

### Docker (Recommended)
- Docker Engine 20.10+
- Docker Compose 1.29+

### Native Installation
- Go 1.25+
- MariaDB client tools (for dump command)
- Network access to MariaDB server

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome. Please ensure:
- Code follows Go best practices
- Tests pass (`go test ./...`)
- Docker build succeeds
- Documentation is updated

## Support

For issues, feature requests, or questions, please open an issue on GitHub.