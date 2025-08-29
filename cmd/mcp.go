package cmd

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
	"mariadb-extractor/internal/config"
)

// MCPRequest represents an MCP protocol request
type MCPRequest struct {
	ID     string                 `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// MCPResponse represents an MCP protocol response
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPServer handles MCP protocol requests
type MCPServer struct {
	db          *sql.DB
	validator   *QueryValidator
	rateLimiter *RateLimiter
	auditLogger *AuditLogger
	redactor    *DataRedactor
	timeout     time.Duration
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(db *sql.DB, auditLogPath string) (*MCPServer, error) {
	auditLogger, err := NewAuditLogger(auditLogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	return &MCPServer{
		db:          db,
		validator:   NewQueryValidator(),
		rateLimiter: NewRateLimiter(10, 3), // Higher limits for MCP mode
		auditLogger: auditLogger,
		redactor:    NewDataRedactor(true),
		timeout:     30 * time.Second,
	}, nil
}

// HandleRequest processes an MCP request and returns a response
func (s *MCPServer) HandleRequest(ctx context.Context, req MCPRequest) MCPResponse {
	switch req.Method {
	case "query_database":
		return s.handleQueryDatabase(ctx, req)
	case "get_table_schema":
		return s.handleGetTableSchema(ctx, req)
	case "get_foreign_keys":
		return s.handleGetForeignKeys(ctx, req)
	case "list_databases":
		return s.handleListDatabases(ctx, req)
	case "list_tables":
		return s.handleListTables(ctx, req)
	default:
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("method not found: %s", req.Method),
			},
		}
	}
}

// handleQueryDatabase executes a safe SQL query
func (s *MCPServer) handleQueryDatabase(ctx context.Context, req MCPRequest) MCPResponse {
	// Extract parameters
	query, _ := req.Params["query"].(string)
	database, _ := req.Params["database"].(string)
	format, _ := req.Params["format"].(string)
	if format == "" {
		format = "json"
	}
	
	limitFloat, ok := req.Params["limit"].(float64)
	limit := 1000
	if ok {
		limit = int(limitFloat)
	}

	// Check rate limit
	allowed, err := s.rateLimiter.Allow()
	if !allowed {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32603,
				Message: err.Error(),
			},
		}
	}
	defer s.rateLimiter.Release()

	// Validate query
	if err := s.validator.Validate(query); err != nil {
		s.auditLogger.Log(QueryAuditEvent{
			Timestamp: time.Now(),
			Query:     query,
			Database:  database,
			Success:   false,
			Error:     err.Error(),
		})
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: fmt.Sprintf("invalid query: %v", err),
			},
		}
	}

	// Execute query
	executor := &QueryExecutor{
		db:          s.db,
		validator:   s.validator,
		timeout:     s.timeout,
		rateLimiter: s.rateLimiter,
		auditLogger: s.auditLogger,
	}

	// Create a modified context for the query with the limit
	queryCtx := context.WithValue(ctx, "limit", limit)
	
	result, err := executor.ExecuteQuery(queryCtx, query, database)
	if err != nil {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32603,
				Message: err.Error(),
			},
		}
	}

	// Format response based on requested format
	var output interface{}
	if format == "json" {
		output = result
	} else {
		formatter := &OutputFormatter{}
		var formatted string
		switch format {
		case "markdown":
			formatted = formatter.FormatMarkdown(result)
		case "csv":
			formatted = formatter.FormatCSV(result)
		default:
			formatted, _ = formatter.FormatJSON(result)
		}
		output = map[string]interface{}{
			"formatted": formatted,
			"metadata": map[string]interface{}{
				"row_count":      result.RowCount,
				"execution_time": result.ExecutionTime,
				"database":       result.Database,
			},
		}
	}

	return MCPResponse{
		ID:     req.ID,
		Result: output,
	}
}

// handleGetTableSchema returns schema information for a table
func (s *MCPServer) handleGetTableSchema(ctx context.Context, req MCPRequest) MCPResponse {
	database, _ := req.Params["database"].(string)
	table, _ := req.Params["table"].(string)

	if database == "" || table == "" {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "database and table parameters are required",
			},
		}
	}

	query := fmt.Sprintf(`
		SELECT 
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_KEY,
			EXTRA,
			COLUMN_COMMENT
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'
		ORDER BY ORDINAL_POSITION
	`, database, table)

	return s.handleQueryDatabase(ctx, MCPRequest{
		ID:     req.ID,
		Method: "query_database",
		Params: map[string]interface{}{
			"query":    query,
			"database": "information_schema",
			"format":   "json",
		},
	})
}

// handleGetForeignKeys returns foreign key relationships
func (s *MCPServer) handleGetForeignKeys(ctx context.Context, req MCPRequest) MCPResponse {
	database, _ := req.Params["database"].(string)
	table, _ := req.Params["table"].(string)

	if database == "" {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "database parameter is required",
			},
		}
	}

	query := fmt.Sprintf(`
		SELECT 
			CONSTRAINT_NAME,
			TABLE_NAME,
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE REFERENCED_TABLE_NAME IS NOT NULL
		AND TABLE_SCHEMA = '%s'
	`, database)

	if table != "" {
		query += fmt.Sprintf(" AND TABLE_NAME = '%s'", table)
	}

	return s.handleQueryDatabase(ctx, MCPRequest{
		ID:     req.ID,
		Method: "query_database",
		Params: map[string]interface{}{
			"query":    query,
			"database": "information_schema",
			"format":   "json",
		},
	})
}

// handleListDatabases returns list of all databases
func (s *MCPServer) handleListDatabases(ctx context.Context, req MCPRequest) MCPResponse {
	query := `
		SELECT 
			SCHEMA_NAME as database_name,
			DEFAULT_CHARACTER_SET_NAME as charset,
			DEFAULT_COLLATION_NAME as collation
		FROM information_schema.SCHEMATA
		WHERE SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
		ORDER BY SCHEMA_NAME
	`

	return s.handleQueryDatabase(ctx, MCPRequest{
		ID:     req.ID,
		Method: "query_database",
		Params: map[string]interface{}{
			"query":    query,
			"database": "information_schema",
			"format":   "json",
		},
	})
}

// handleListTables returns list of tables in a database
func (s *MCPServer) handleListTables(ctx context.Context, req MCPRequest) MCPResponse {
	database, _ := req.Params["database"].(string)

	if database == "" {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "database parameter is required",
			},
		}
	}

	query := fmt.Sprintf(`
		SELECT 
			TABLE_NAME,
			TABLE_TYPE,
			ENGINE,
			TABLE_ROWS,
			DATA_LENGTH,
			INDEX_LENGTH,
			TABLE_COMMENT
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = '%s'
		ORDER BY TABLE_NAME
	`, database)

	return s.handleQueryDatabase(ctx, MCPRequest{
		ID:     req.ID,
		Method: "query_database",
		Params: map[string]interface{}{
			"query":    query,
			"database": "information_schema",
			"format":   "json",
		},
	})
}

// RunMCPServer starts the MCP server in stdio mode
func RunMCPServer(db *sql.DB, auditLogPath string) error {
	server, err := NewMCPServer(db, auditLogPath)
	if err != nil {
		return err
	}
	defer server.auditLogger.Close()

	// Log server start
	fmt.Fprintf(os.Stderr, "MariaDB MCP server started\n")
	fmt.Fprintf(os.Stderr, "Reading from stdin, writing to stdout\n")
	
	// Send initial capabilities message
	capabilities := map[string]interface{}{
		"name":    "mariadb-query",
		"version": "1.0.0",
		"tools": []string{
			"query_database",
			"get_table_schema",
			"get_foreign_keys",
			"list_databases",
			"list_tables",
		},
	}
	
	capabilitiesJSON, _ := json.Marshal(capabilities)
	fmt.Printf("%s\n", capabilitiesJSON)

	// Read requests from stdin and write responses to stdout
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large queries

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse request
		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// Send error response
			errResp := MCPResponse{
				Error: &MCPError{
					Code:    -32700,
					Message: fmt.Sprintf("parse error: %v", err),
				},
			}
			respJSON, _ := json.Marshal(errResp)
			fmt.Printf("%s\n", respJSON)
			continue
		}

		// Handle request
		ctx := context.Background()
		resp := server.HandleRequest(ctx, req)

		// Send response
		respJSON, err := json.Marshal(resp)
		if err != nil {
			errResp := MCPResponse{
				ID: req.ID,
				Error: &MCPError{
					Code:    -32603,
					Message: fmt.Sprintf("response encoding error: %v", err),
				},
			}
			respJSON, _ = json.Marshal(errResp)
		}
		fmt.Printf("%s\n", respJSON)
	}

	if err := scanner.Err(); err != nil {
		if err != io.EOF {
			return fmt.Errorf("error reading input: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "MariaDB MCP server stopped\n")
	return nil
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for MariaDB queries",
	Long: `Start an MCP (Model Context Protocol) server that allows safe, read-only
queries to MariaDB databases. This server can be used with Claude Desktop or
other MCP-compatible clients.

The server communicates via stdio (stdin/stdout) using JSON-RPC format.

Available methods:
  - query_database: Execute safe SQL queries
  - get_table_schema: Get table structure information
  - get_foreign_keys: Get foreign key relationships
  - list_databases: List all databases
  - list_tables: List tables in a database

Example usage with Claude Desktop:
  1. Add to claude_desktop_config.json:
     {
       "mcpServers": {
         "mariadb": {
           "command": "/path/to/mariadb-extractor",
           "args": ["mcp"],
           "env": {
             "MARIADB_HOST": "localhost",
             "MARIADB_PORT": "3306",
             "MARIADB_USER": "username",
             "MARIADB_PASSWORD": "password"
           }
         }
       }
     }
  
  2. Restart Claude Desktop
  3. Use in conversations: "Query the users table in mydb database"
`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runMCPServer(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var (
	mcpHost     string
	mcpPort     int
	mcpUser     string
	mcpPassword string
	mcpAuditLog string
	mcpTimeout  int
)

func init() {
	// Load environment variables
	config.LoadEnv()

	// Connection flags
	mcpCmd.Flags().StringVar(&mcpHost, "host", os.Getenv("MARIADB_HOST"), "MariaDB host")
	mcpCmd.Flags().IntVar(&mcpPort, "port", 3306, "MariaDB port")
	mcpCmd.Flags().StringVar(&mcpUser, "user", os.Getenv("MARIADB_USER"), "MariaDB user")
	mcpCmd.Flags().StringVar(&mcpPassword, "password", os.Getenv("MARIADB_PASSWORD"), "MariaDB password")
	
	// MCP specific flags
	mcpCmd.Flags().StringVar(&mcpAuditLog, "audit-log", "", "Audit log file path")
	mcpCmd.Flags().IntVar(&mcpTimeout, "timeout", 30, "Query timeout in seconds")

	// Set default port from environment if available
	if portStr := os.Getenv("MARIADB_PORT"); portStr != "" {
		var port int
		fmt.Sscanf(portStr, "%d", &port)
		if port > 0 {
			mcpCmd.Flags().Set("port", fmt.Sprintf("%d", port))
		}
	}

	// Set default audit log path
	homeDir, _ := os.UserHomeDir()
	defaultAuditLog := filepath.Join(homeDir, ".mariadb-mcp", "audit.log")
	mcpCmd.Flags().Set("audit-log", defaultAuditLog)

	rootCmd.AddCommand(mcpCmd)
}

func runMCPServer() error {
	// Validate connection parameters
	if mcpHost == "" {
		mcpHost = "localhost"
	}
	if mcpUser == "" {
		return fmt.Errorf("user is required (use --user or set MARIADB_USER)")
	}
	if mcpPassword == "" {
		return fmt.Errorf("password is required (use --password or set MARIADB_PASSWORD)")
	}

	// Create database connection
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=true&timeout=%ds",
		mcpUser, mcpPassword, mcpHost, mcpPort, mcpTimeout)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}
	defer db.Close()

	// Configure connection pool for read-only operations
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create audit log directory if needed
	if mcpAuditLog != "" {
		auditDir := filepath.Dir(mcpAuditLog)
		if err := os.MkdirAll(auditDir, 0755); err != nil {
			return fmt.Errorf("failed to create audit log directory: %w", err)
		}
	}

	// Start MCP server
	return RunMCPServer(db, mcpAuditLog)
}