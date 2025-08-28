# MariaDB Extractor

A CLI tool for extracting database and table information from MariaDB servers. Generates both human-readable markdown files and machine-readable JSON files with comprehensive database schema information.

## Features

- Extract database names and metadata from MariaDB servers
- Retrieve detailed table information including types, engines, and storage metrics
- Generate formatted markdown reports for documentation
- Create structured JSON output for automation and integration
- Support for custom connection parameters via CLI flags or environment variables

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

## Usage

```bash
./mariadb-extractor extract --host localhost --port 3306 --user your_username --password your_password --output my-extraction
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

#### Using .env File

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

#### Using Environment Variables

Set credentials via environment:
```bash
export MARIADB_HOST=db.example.com
export MARIADB_PORT=3306
export MARIADB_USER=myuser
export MARIADB_PASSWORD=mypassword
export MARIADB_OUTPUT_PREFIX=production

./mariadb-extractor extract
```

#### Mix Environment Variables and CLI Flags

Use environment for credentials, override host via CLI:
```bash
export MARIADB_USER=myuser
export MARIADB_PASSWORD=mypassword

./mariadb-extractor extract -H dev-db.example.com -o dev-extraction
```

#### Traditional CLI Usage

Extract from local MariaDB with all flags:
```bash
./mariadb-extractor extract -u root -p mypassword
```

Extract from remote server:
```bash
./mariadb-extractor extract -H db.example.com -P 3307 -u admin -p secret -o production-db
```

#### CI/CD Pipeline Example

```yaml
# Example GitHub Actions or CI configuration
env:
  MARIADB_HOST: ${{ secrets.DB_HOST }}
  MARIADB_USER: ${{ secrets.DB_USER }}
  MARIADB_PASSWORD: ${{ secrets.DB_PASSWORD }}

script:
  - ./mariadb-extractor extract -o nightly-backup
```

## Output Files

The tool generates two output files:

### Markdown Report (.md)
- Human-readable documentation
- Formatted tables with database and table information
- Summary statistics and metadata

### JSON Data (.json)
- Machine-readable structured data
- Comprehensive metadata including extraction timestamp
- Suitable for automation and further processing

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

## License

This project is open source and available under the MIT License.