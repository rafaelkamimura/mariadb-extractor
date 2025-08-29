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

```bash
go build -o mariadb-extractor
```

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

- Go 1.19 or later
- Access to MariaDB/MySQL server
- MySQL client libraries (handled by go-sql-driver/mysql)

### Additional Requirements for Dump Command

The `dump` command requires the `mysqldump` utility to be installed on your system:

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

**Note:** The `extract` and `ddl` commands only require Go and database access. The `dump` command additionally requires the MariaDB/MySQL client tools.

## License

This project is open source and available under the MIT License.