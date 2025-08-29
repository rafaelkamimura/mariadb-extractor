package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
	"mariadb-extractor/internal/config"
)

var (
	// Query command flags
	queryString      string
	queryFile        string
	queryDatabase    string
	queryLimit       int
	queryTimeout     int
	queryFormat      string
	queryInteractive bool
	queryMCPMode     bool
	queryNoRedact    bool
	queryAuditLog    string
	
	// Rate limiting
	queryRateLimit   int
	queryMaxConcurrent int
	
	// Connection flags (reuse pattern from other commands)
	queryHost     string
	queryPort     int
	queryUser     string
	queryPassword string
)

// QueryValidator provides SQL injection prevention and query validation
type QueryValidator struct {
	allowedOperations []string
	blockedPatterns   []*regexp.Regexp
	maxQueryLength    int
}

// NewQueryValidator creates a validator with security rules
func NewQueryValidator() *QueryValidator {
	return &QueryValidator{
		allowedOperations: []string{"SELECT", "SHOW", "DESCRIBE", "DESC", "EXPLAIN"},
		blockedPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\b(DROP|DELETE|UPDATE|INSERT|ALTER|CREATE|TRUNCATE|EXEC|EXECUTE)\b`),
			regexp.MustCompile(`(?i)\b(INTO\s+OUTFILE|LOAD_FILE|SYSTEM)\b`),
			regexp.MustCompile(`(?i)\b(GRANT|REVOKE|SET|KILL)\b`),
			regexp.MustCompile(`(?i)(\-\-|\#|\/\*|\*\/)`), // SQL comments that could hide injections
		},
		maxQueryLength: 10000,
	}
}

// Validate checks if a query is safe to execute
func (qv *QueryValidator) Validate(query string) error {
	// Check query length
	if len(query) > qv.maxQueryLength {
		return fmt.Errorf("query exceeds maximum length of %d characters", qv.maxQueryLength)
	}
	
	// Normalize for checking
	normalized := strings.TrimSpace(strings.ToUpper(query))
	if normalized == "" {
		return fmt.Errorf("empty query")
	}
	
	// Check if query starts with allowed operation
	allowed := false
	for _, op := range qv.allowedOperations {
		if strings.HasPrefix(normalized, op) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("query must start with one of: %s", strings.Join(qv.allowedOperations, ", "))
	}
	
	// Check for blocked patterns
	for _, pattern := range qv.blockedPatterns {
		if pattern.MatchString(query) {
			return fmt.Errorf("query contains prohibited operation or pattern")
		}
	}
	
	// Check for multiple statements (semicolon not at end)
	if strings.Count(query, ";") > 1 || (strings.Contains(query, ";") && !strings.HasSuffix(strings.TrimSpace(query), ";")) {
		return fmt.Errorf("multiple statements not allowed")
	}
	
	return nil
}

// QueryExecutor handles safe query execution
type QueryExecutor struct {
	db           *sql.DB
	validator    *QueryValidator
	timeout      time.Duration
	rateLimiter  *RateLimiter
	auditLogger  *AuditLogger
}

// RateLimiter provides query rate limiting
type RateLimiter struct {
	mu            sync.Mutex
	maxPerSecond  int
	maxConcurrent int
	lastReset     time.Time
	count         int
	concurrent    int
}

// NewRateLimiter creates a rate limiter
func NewRateLimiter(maxPerSecond, maxConcurrent int) *RateLimiter {
	return &RateLimiter{
		maxPerSecond:  maxPerSecond,
		maxConcurrent: maxConcurrent,
		lastReset:     time.Now(),
	}
}

// Allow checks if a query can proceed
func (rl *RateLimiter) Allow() (bool, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Reset counter every second
	if time.Since(rl.lastReset) > time.Second {
		rl.count = 0
		rl.lastReset = time.Now()
	}
	
	// Check rate limit
	if rl.count >= rl.maxPerSecond {
		return false, fmt.Errorf("rate limit exceeded: max %d queries per second", rl.maxPerSecond)
	}
	
	// Check concurrent limit
	if rl.concurrent >= rl.maxConcurrent {
		return false, fmt.Errorf("concurrent limit exceeded: max %d concurrent queries", rl.maxConcurrent)
	}
	
	rl.count++
	rl.concurrent++
	return true, nil
}

// Release decrements concurrent counter
func (rl *RateLimiter) Release() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.concurrent > 0 {
		rl.concurrent--
	}
}

// AuditLogger logs all query attempts
type AuditLogger struct {
	mu       sync.Mutex
	filePath string
	file     *os.File
}

// QueryAuditEvent represents a query execution attempt
type QueryAuditEvent struct {
	Timestamp     time.Time     `json:"timestamp"`
	Query         string        `json:"query"`
	Database      string        `json:"database"`
	User          string        `json:"user"`
	ExecutionTime time.Duration `json:"execution_time_ms"`
	RowCount      int           `json:"row_count"`
	Success       bool          `json:"success"`
	Error         string        `json:"error,omitempty"`
}

// NewAuditLogger creates an audit logger
func NewAuditLogger(filePath string) (*AuditLogger, error) {
	if filePath == "" {
		return &AuditLogger{}, nil // No-op logger
	}
	
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}
	
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}
	
	return &AuditLogger{
		filePath: filePath,
		file:     file,
	}, nil
}

// Log writes an audit event
func (al *AuditLogger) Log(event QueryAuditEvent) error {
	if al.file == nil {
		return nil // No-op
	}
	
	al.mu.Lock()
	defer al.mu.Unlock()
	
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	
	_, err = fmt.Fprintf(al.file, "%s\n", data)
	return err
}

// Close closes the audit log file
func (al *AuditLogger) Close() error {
	if al.file != nil {
		return al.file.Close()
	}
	return nil
}

// QueryResult represents query execution results
type QueryResult struct {
	Query         string                   `json:"query"`
	Database      string                   `json:"database"`
	Columns       []string                 `json:"columns"`
	Rows          []map[string]interface{} `json:"rows"`
	RowCount      int                      `json:"row_count"`
	ExecutionTime string                   `json:"execution_time"`
	Timestamp     string                   `json:"timestamp"`
}

// DataRedactor redacts sensitive information from results
type DataRedactor struct {
	patterns []*regexp.Regexp
	enabled  bool
}

// NewDataRedactor creates a data redactor
func NewDataRedactor(enabled bool) *DataRedactor {
	if !enabled {
		return &DataRedactor{enabled: false}
	}
	
	return &DataRedactor{
		enabled: true,
		patterns: []*regexp.Regexp{
			// Email addresses
			regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
			// Phone numbers (various formats)
			regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
			// Credit card patterns
			regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`),
			// SSN patterns
			regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		},
	}
}

// RedactValue redacts sensitive data from a string
func (dr *DataRedactor) RedactValue(value string) string {
	if !dr.enabled {
		return value
	}
	
	result := value
	for _, pattern := range dr.patterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}
	
	// Redact values for sensitive column names
	sensitiveColumns := []string{"password", "token", "secret", "key", "salt", "hash"}
	for _, col := range sensitiveColumns {
		if strings.Contains(strings.ToLower(value), col) {
			return "[REDACTED]"
		}
	}
	
	return result
}

// ExecuteQuery safely executes a query and returns results
func (qe *QueryExecutor) ExecuteQuery(ctx context.Context, query, database string) (*QueryResult, error) {
	// Check rate limit
	allowed, err := qe.rateLimiter.Allow()
	if !allowed {
		qe.auditLogger.Log(QueryAuditEvent{
			Timestamp: time.Now(),
			Query:     query,
			Database:  database,
			User:      queryUser,
			Success:   false,
			Error:     err.Error(),
		})
		return nil, err
	}
	defer qe.rateLimiter.Release()
	
	// Validate query
	if err := qe.validator.Validate(query); err != nil {
		qe.auditLogger.Log(QueryAuditEvent{
			Timestamp: time.Now(),
			Query:     query,
			Database:  database,
			User:      queryUser,
			Success:   false,
			Error:     fmt.Sprintf("validation failed: %v", err),
		})
		return nil, fmt.Errorf("query validation failed: %w", err)
	}
	
	// Switch to specified database if provided
	if database != "" {
		if _, err := qe.db.ExecContext(ctx, fmt.Sprintf("USE `%s`", database)); err != nil {
			return nil, fmt.Errorf("failed to switch to database %s: %w", database, err)
		}
	}
	
	// Execute query with timeout
	queryCtx, cancel := context.WithTimeout(ctx, qe.timeout)
	defer cancel()
	
	start := time.Now()
	rows, err := qe.db.QueryContext(queryCtx, query)
	executionTime := time.Since(start)
	
	if err != nil {
		qe.auditLogger.Log(QueryAuditEvent{
			Timestamp:     time.Now(),
			Query:         query,
			Database:      database,
			User:          queryUser,
			ExecutionTime: executionTime,
			Success:       false,
			Error:         err.Error(),
		})
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()
	
	// Get column information
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	
	// Create data redactor
	redactor := NewDataRedactor(!queryNoRedact)
	
	// Process results
	var results []map[string]interface{}
	for rows.Next() {
		// Create a slice to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		
		// Create map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			
			// Handle different types and redact if necessary
			switch v := val.(type) {
			case []byte:
				strVal := string(v)
				row[col] = redactor.RedactValue(strVal)
			case string:
				row[col] = redactor.RedactValue(v)
			default:
				row[col] = val
			}
		}
		
		results = append(results, row)
		
		// Enforce row limit
		if len(results) >= queryLimit {
			break
		}
	}
	
	// Check for errors from iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading rows: %w", err)
	}
	
	// Log successful query
	qe.auditLogger.Log(QueryAuditEvent{
		Timestamp:     time.Now(),
		Query:         query,
		Database:      database,
		User:          queryUser,
		ExecutionTime: executionTime,
		RowCount:      len(results),
		Success:       true,
	})
	
	return &QueryResult{
		Query:         query,
		Database:      database,
		Columns:       columns,
		Rows:          results,
		RowCount:      len(results),
		ExecutionTime: fmt.Sprintf("%dms", executionTime.Milliseconds()),
		Timestamp:     time.Now().Format(time.RFC3339),
	}, nil
}

// OutputFormatter handles different output formats
type OutputFormatter struct{}

// FormatJSON outputs results as JSON
func (of *OutputFormatter) FormatJSON(result *QueryResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatMarkdown outputs results as Markdown table
func (of *OutputFormatter) FormatMarkdown(result *QueryResult) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("# Query Results\n\n"))
	sb.WriteString(fmt.Sprintf("**Database:** %s\n", result.Database))
	sb.WriteString(fmt.Sprintf("**Execution Time:** %s\n", result.ExecutionTime))
	sb.WriteString(fmt.Sprintf("**Row Count:** %d\n", result.RowCount))
	sb.WriteString(fmt.Sprintf("**Timestamp:** %s\n\n", result.Timestamp))
	
	sb.WriteString("```sql\n")
	sb.WriteString(result.Query)
	sb.WriteString("\n```\n\n")
	
	if len(result.Rows) == 0 {
		sb.WriteString("*No results returned*\n")
		return sb.String()
	}
	
	// Create markdown table
	sb.WriteString("| ")
	for _, col := range result.Columns {
		sb.WriteString(fmt.Sprintf("%s | ", col))
	}
	sb.WriteString("\n| ")
	for range result.Columns {
		sb.WriteString("--- | ")
	}
	sb.WriteString("\n")
	
	for _, row := range result.Rows {
		sb.WriteString("| ")
		for _, col := range result.Columns {
			val := row[col]
			if val == nil {
				sb.WriteString("NULL | ")
			} else {
				sb.WriteString(fmt.Sprintf("%v | ", val))
			}
		}
		sb.WriteString("\n")
	}
	
	return sb.String()
}

// FormatCSV outputs results as CSV
func (of *OutputFormatter) FormatCSV(result *QueryResult) string {
	var sb strings.Builder
	
	// Write headers
	sb.WriteString(strings.Join(result.Columns, ","))
	sb.WriteString("\n")
	
	// Write rows
	for _, row := range result.Rows {
		values := make([]string, len(result.Columns))
		for i, col := range result.Columns {
			val := row[col]
			if val == nil {
				values[i] = ""
			} else {
				// Quote strings that contain commas
				strVal := fmt.Sprintf("%v", val)
				if strings.Contains(strVal, ",") || strings.Contains(strVal, "\"") {
					strVal = fmt.Sprintf("\"%s\"", strings.ReplaceAll(strVal, "\"", "\"\""))
				}
				values[i] = strVal
			}
		}
		sb.WriteString(strings.Join(values, ","))
		sb.WriteString("\n")
	}
	
	return sb.String()
}

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Execute safe read-only queries against MariaDB",
	Long: `Execute SELECT, SHOW, DESCRIBE, and EXPLAIN queries safely.
Supports multiple output formats and includes security features like query validation,
rate limiting, and audit logging. Can also act as an MCP-compatible query interface.

Examples:
  # Execute a simple query
  mariadb-extractor query -q "SELECT * FROM users LIMIT 10" -d mydb
  
  # Query with JSON output
  mariadb-extractor query -q "SHOW TABLES" -d mydb -f json
  
  # Query from file
  mariadb-extractor query -F queries.sql -d mydb
  
  # Get foreign key relationships
  mariadb-extractor query -q "SELECT * FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_NAME IS NOT NULL" -d mydb
`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runQuery(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	// Load environment variables
	config.LoadEnv()
	
	// Connection flags
	queryCmd.Flags().StringVarP(&queryHost, "host", "H", os.Getenv("MARIADB_HOST"), "MariaDB host")
	queryCmd.Flags().IntVarP(&queryPort, "port", "P", 3306, "MariaDB port")
	queryCmd.Flags().StringVarP(&queryUser, "user", "u", os.Getenv("MARIADB_USER"), "MariaDB user")
	queryCmd.Flags().StringVarP(&queryPassword, "password", "p", os.Getenv("MARIADB_PASSWORD"), "MariaDB password")
	
	// Query flags
	queryCmd.Flags().StringVarP(&queryString, "query", "q", "", "SQL query to execute")
	queryCmd.Flags().StringVarP(&queryFile, "file", "F", "", "File containing SQL query")
	queryCmd.Flags().StringVarP(&queryDatabase, "database", "d", "", "Database to use")
	queryCmd.Flags().IntVarP(&queryLimit, "limit", "l", 1000, "Maximum rows to return")
	queryCmd.Flags().IntVarP(&queryTimeout, "timeout", "t", 30, "Query timeout in seconds")
	queryCmd.Flags().StringVarP(&queryFormat, "format", "f", "markdown", "Output format (json, markdown, csv)")
	
	// Security flags
	queryCmd.Flags().BoolVar(&queryNoRedact, "no-redact", false, "Disable automatic PII redaction")
	queryCmd.Flags().StringVar(&queryAuditLog, "audit-log", "", "Path to audit log file")
	queryCmd.Flags().IntVar(&queryRateLimit, "rate-limit", 5, "Max queries per second")
	queryCmd.Flags().IntVar(&queryMaxConcurrent, "max-concurrent", 2, "Max concurrent queries")
	
	// Mode flags
	queryCmd.Flags().BoolVarP(&queryInteractive, "interactive", "i", false, "Interactive query mode")
	queryCmd.Flags().BoolVar(&queryMCPMode, "mcp-server", false, "Start MCP server mode")
	
	// Set default port from environment if available
	if portStr := os.Getenv("MARIADB_PORT"); portStr != "" {
		var port int
		fmt.Sscanf(portStr, "%d", &port)
		if port > 0 {
			queryCmd.Flags().Set("port", fmt.Sprintf("%d", port))
		}
	}
	
	rootCmd.AddCommand(queryCmd)
}

func runQuery() error {
	// Validate connection parameters
	if queryHost == "" {
		return fmt.Errorf("host is required (use --host or set MARIADB_HOST)")
	}
	if queryUser == "" {
		return fmt.Errorf("user is required (use --user or set MARIADB_USER)")
	}
	if queryPassword == "" {
		return fmt.Errorf("password is required (use --password or set MARIADB_PASSWORD)")
	}
	
	// Check query input
	if queryString == "" && queryFile == "" && !queryInteractive && !queryMCPMode {
		return fmt.Errorf("no query provided (use -q, -F, -i, or --mcp-server)")
	}
	
	// Read query from file if specified
	if queryFile != "" {
		data, err := os.ReadFile(queryFile)
		if err != nil {
			return fmt.Errorf("failed to read query file: %w", err)
		}
		queryString = string(data)
	}
	
	// Handle special modes
	if queryInteractive {
		return runInteractiveMode()
	}
	if queryMCPMode {
		return runMCPServer()
	}
	
	// Create database connection
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=true&timeout=%ds",
		queryUser, queryPassword, queryHost, queryPort, queryTimeout)
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}
	defer db.Close()
	
	// Configure connection pool for read-only operations
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Create audit logger
	auditLogger, err := NewAuditLogger(queryAuditLog)
	if err != nil {
		return fmt.Errorf("failed to create audit logger: %w", err)
	}
	defer auditLogger.Close()
	
	// Create query executor
	executor := &QueryExecutor{
		db:          db,
		validator:   NewQueryValidator(),
		timeout:     time.Duration(queryTimeout) * time.Second,
		rateLimiter: NewRateLimiter(queryRateLimit, queryMaxConcurrent),
		auditLogger: auditLogger,
	}
	
	// Execute query
	result, err := executor.ExecuteQuery(context.Background(), queryString, queryDatabase)
	if err != nil {
		return err
	}
	
	// Format and output results
	formatter := &OutputFormatter{}
	var output string
	
	switch strings.ToLower(queryFormat) {
	case "json":
		output, err = formatter.FormatJSON(result)
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
	case "csv":
		output = formatter.FormatCSV(result)
	case "markdown", "md":
		output = formatter.FormatMarkdown(result)
	default:
		return fmt.Errorf("unsupported format: %s (use json, markdown, or csv)", queryFormat)
	}
	
	fmt.Print(output)
	
	// Save to file if output prefix is set
	if outputPrefix := os.Getenv("MARIADB_OUTPUT_PREFIX"); outputPrefix != "" {
		outputDir := "output"
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		
		ext := queryFormat
		if ext == "markdown" {
			ext = "md"
		}
		
		filename := filepath.Join(outputDir, fmt.Sprintf("%s-query.%s", outputPrefix, ext))
		if err := os.WriteFile(filename, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		
		fmt.Fprintf(os.Stderr, "\nResults saved to: %s\n", filename)
	}
	
	return nil
}

// runInteractiveMode starts an interactive query session
func runInteractiveMode() error {
	fmt.Println("Interactive query mode not yet implemented")
	return nil
}

// runMCPServer starts the MCP server mode
func runMCPServer() error {
	fmt.Println("MCP server mode not yet implemented")
	return nil
}