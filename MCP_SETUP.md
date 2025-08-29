# MariaDB MCP Server Setup Guide

This guide explains how to set up the MariaDB Extractor as an MCP (Model Context Protocol) server for use with Claude Desktop and Claude Code.

## What is MCP?

MCP (Model Context Protocol) allows AI assistants like Claude to interact with external tools and databases safely. This implementation provides read-only access to your MariaDB databases with comprehensive security features.

## Features

- **Safe Query Execution**: Only SELECT, SHOW, DESCRIBE, and EXPLAIN queries allowed
- **SQL Injection Prevention**: Comprehensive validation and pattern blocking
- **Rate Limiting**: Prevents abuse with configurable limits
- **Audit Logging**: Tracks all queries for security monitoring
- **PII Redaction**: Automatic redaction of sensitive data
- **Multiple Database Support**: Configure multiple database connections

## Available MCP Tools

1. **query_database**: Execute safe SQL queries
2. **get_table_schema**: Get table structure and column information
3. **get_foreign_keys**: Discover foreign key relationships
4. **list_databases**: List all available databases
5. **list_tables**: List tables with statistics

## Setup for Claude Desktop

### Step 1: Build the Binary

```bash
# Build the mariadb-extractor binary
go build -o mariadb-extractor

# Or use Docker
make build
```

### Step 2: Configure Claude Desktop

Add to your Claude Desktop configuration file:
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "mariadb": {
      "command": "/path/to/mariadb-extractor/mcp-start.sh",
      "args": [],
      "env": {
        "MARIADB_HOST": "localhost",
        "MARIADB_PORT": "3306",
        "MARIADB_USER": "your-username",
        "MARIADB_PASSWORD": "your-password"
      }
    }
  }
}
```

### Step 3: Restart Claude Desktop

After updating the configuration, restart Claude Desktop for the changes to take effect.

## Setup for Claude Code (claude.ai/code)

### Option 1: Local Installation

For use in your local Claude Code environment:

```bash
# 1. Clone and build
git clone <repository-url>
cd mariadb-extractor
go build -o mariadb-extractor

# 2. Create configuration
cp .env.example .env
# Edit .env with your database credentials

# 3. Test MCP server
./mcp-start.sh
# Should output: {"name":"mariadb-query","version":"1.0.0","tools":...}
```

### Option 2: Global Installation

Install globally for use across all projects:

```bash
# Build and install
go build -o mariadb-extractor
sudo cp mariadb-extractor /usr/local/bin/
sudo cp mcp-start.sh /usr/local/bin/mariadb-mcp

# Create global config
mkdir -p ~/.mariadb-mcp
cat > ~/.mariadb-mcp/config.env << EOF
MARIADB_HOST=localhost
MARIADB_PORT=3306
MARIADB_USER=your-username
MARIADB_PASSWORD=your-password
EOF
```

## Configuration Examples

### Multiple Database Connections

Configure multiple database connections in Claude Desktop:

```json
{
  "mcpServers": {
    "mariadb-dev": {
      "command": "/path/to/mariadb-extractor/mcp-start.sh",
      "args": [],
      "env": {
        "MARIADB_HOST": "localhost",
        "MARIADB_PORT": "3307",
        "MARIADB_USER": "devuser",
        "MARIADB_PASSWORD": "devpass"
      }
    },
    "mariadb-staging": {
      "command": "/path/to/mariadb-extractor/mcp-start.sh",
      "args": [],
      "env": {
        "MARIADB_HOST": "staging.example.com",
        "MARIADB_PORT": "3306",
        "MARIADB_USER": "staging_user",
        "MARIADB_PASSWORD": "staging_pass"
      }
    },
    "mariadb-prod": {
      "command": "/path/to/mariadb-extractor/mcp-start.sh",
      "args": ["--audit-log", "/var/log/mariadb-mcp/prod-audit.log"],
      "env": {
        "MARIADB_HOST": "prod.example.com",
        "MARIADB_PORT": "3306",
        "MARIADB_USER": "readonly_user",
        "MARIADB_PASSWORD": "secure_password"
      }
    }
  }
}
```

### Docker Configuration

For Docker-based setups:

```json
{
  "mcpServers": {
    "mariadb-docker": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "--env-file", "/path/to/.env",
        "mariadb-extractor",
        "mcp"
      ]
    }
  }
}
```

## Usage Examples

Once configured, you can use natural language in Claude to query your databases:

### Basic Queries
- "Show me all tables in the users database"
- "Get the schema for the orders table in myapp database"
- "List all foreign key relationships in the production database"
- "Query the first 10 users from the users table"

### Complex Queries
- "Find all orders with their customer information using a JOIN"
- "Show me the table sizes and row counts for all tables in mydb"
- "Get the foreign key relationships between orders and customers tables"
- "Analyze the structure of the authentication schema"

### Claude Code Integration
When using Claude Code, the MCP server enables:
- Database schema exploration while coding
- Real-time data validation
- Foreign key relationship discovery
- Test data inspection
- Query result analysis

## Security Features

### Query Validation
- Blocks all write operations (INSERT, UPDATE, DELETE, DROP, etc.)
- Prevents SQL injection with pattern matching
- Validates query structure before execution

### Rate Limiting
- Default: 10 queries per second
- Maximum 3 concurrent queries
- Configurable via environment variables

### Audit Logging
All queries are logged with:
- Timestamp
- Query text
- Database name
- Execution time
- Success/failure status
- Error messages (if any)

Default location: `~/.mariadb-mcp/audit.log`

### PII Redaction
Automatically redacts:
- Email addresses
- Phone numbers
- Credit card numbers
- Social Security Numbers

Disable with: `--no-redact` flag (use with caution)

## Troubleshooting

### Connection Issues

```bash
# Test connection
./mariadb-extractor query -q "SELECT 1" -d mysql

# Check environment variables
env | grep MARIADB

# Test MCP server directly
echo '{"id":"1","method":"list_databases","params":{}}' | ./mariadb-extractor mcp
```

### Permission Errors

Ensure the database user has SELECT permissions:
```sql
GRANT SELECT ON *.* TO 'readonly_user'@'%';
FLUSH PRIVILEGES;
```

### Claude Desktop Not Finding Tool

1. Check configuration file location
2. Verify command path is absolute
3. Ensure script is executable: `chmod +x mcp-start.sh`
4. Check Claude Desktop logs for errors

### Rate Limiting

If you hit rate limits, adjust in the configuration:
```json
"args": ["--rate-limit", "20", "--max-concurrent", "5"]
```

## Advanced Configuration

### Custom Audit Log Location

```json
"args": ["--audit-log", "/custom/path/audit.log"]
```

### Increased Timeout

```json
"args": ["--timeout", "60"]
```

### Disable PII Redaction (Development Only)

```json
"args": ["--no-redact"]
```

## Best Practices

1. **Use Read-Only Database Users**: Create dedicated users with only SELECT permissions
2. **Enable Audit Logging**: Always enable for production databases
3. **Set Appropriate Rate Limits**: Adjust based on your database capacity
4. **Regular Security Reviews**: Review audit logs periodically
5. **Separate Environments**: Use different MCP configurations for dev/staging/prod
6. **Monitor Resource Usage**: Watch for high CPU or memory usage
7. **Keep Software Updated**: Regularly update the mariadb-extractor tool

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MARIADB_HOST` | Database host | localhost |
| `MARIADB_PORT` | Database port | 3306 |
| `MARIADB_USER` | Database username | - |
| `MARIADB_PASSWORD` | Database password | - |
| `MARIADB_MCP_AUDIT_LOG` | Audit log path | ~/.mariadb-mcp/audit.log |
| `MARIADB_MCP_RATE_LIMIT` | Queries per second | 10 |
| `MARIADB_MCP_MAX_CONCURRENT` | Max concurrent queries | 3 |

## Support

For issues or feature requests related to MCP functionality, please open an issue on GitHub with the tag `mcp`.