#!/bin/bash

# MariaDB MCP Server Startup Script
# For use with Claude Desktop and Claude Code

# Load environment variables from .env if it exists
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Check if running in Docker or native
if [ -f /.dockerenv ]; then
    # Running in Docker
    exec /app/mariadb-extractor mcp "$@"
else
    # Running natively
    # Check if binary exists
    if [ -f ./mariadb-extractor ]; then
        exec ./mariadb-extractor mcp "$@"
    elif [ -f /usr/local/bin/mariadb-extractor ]; then
        exec /usr/local/bin/mariadb-extractor mcp "$@"
    else
        # Try to build if Go is available
        if command -v go &> /dev/null; then
            echo "Building mariadb-extractor..." >&2
            go build -o mariadb-extractor
            exec ./mariadb-extractor mcp "$@"
        else
            echo "Error: mariadb-extractor binary not found" >&2
            echo "Please build with: go build -o mariadb-extractor" >&2
            exit 1
        fi
    fi
fi