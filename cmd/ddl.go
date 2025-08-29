/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

// DDLInfo represents DDL information for a table
type DDLInfo struct {
	DatabaseName string `json:"database_name"`
	TableName    string `json:"table_name"`
	CreateTable  string `json:"create_table"`
}

// ddlCmd represents the ddl command
var ddlCmd = &cobra.Command{
	Use:   "ddl",
	Short: "Extract DDL statements for all tables from MariaDB",
	Long: `Extract CREATE TABLE statements (DDL) for all tables from MariaDB server.
Generates markdown output files with complete table definitions including
columns, indexes, constraints, and other table properties.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDDL()
	},
}

var (
	ddlHost        string
	ddlPort        int
	ddlUser        string
	ddlPassword    string
	ddlOutput      string
	ddlTimeout     int
	ddlMaxRetries  int
	ddlBatchSize   int
)

func init() {
	rootCmd.AddCommand(ddlCmd)

	// Get defaults from environment variables
	defaultHost := getEnvWithDefault("MARIADB_HOST", "localhost")
	defaultPort := getEnvIntWithDefault("MARIADB_PORT", 3306)
	defaultUser := os.Getenv("MARIADB_USER")
	defaultPassword := os.Getenv("MARIADB_PASSWORD")
	defaultOutput := getEnvWithDefault("MARIADB_OUTPUT_PREFIX", "mariadb-ddl")
	defaultTimeout := getEnvIntWithDefault("MARIADB_TIMEOUT", 300)
	defaultMaxRetries := getEnvIntWithDefault("MARIADB_MAX_RETRIES", 3)
	defaultBatchSize := getEnvIntWithDefault("MARIADB_BATCH_SIZE", 10)

	// Database connection flags with environment variable defaults
	ddlCmd.Flags().StringVarP(&ddlHost, "host", "H", defaultHost, "MariaDB host (env: MARIADB_HOST)")
	ddlCmd.Flags().IntVarP(&ddlPort, "port", "P", defaultPort, "MariaDB port (env: MARIADB_PORT)")
	ddlCmd.Flags().StringVarP(&ddlUser, "user", "u", defaultUser, "MariaDB username (env: MARIADB_USER)")
	ddlCmd.Flags().StringVarP(&ddlPassword, "password", "p", defaultPassword, "MariaDB password (env: MARIADB_PASSWORD)")
	ddlCmd.Flags().StringVarP(&ddlOutput, "output", "o", defaultOutput, "Output file prefix (env: MARIADB_OUTPUT_PREFIX)")
	
	// Performance and timeout flags
	ddlCmd.Flags().IntVarP(&ddlTimeout, "timeout", "t", defaultTimeout, "Query timeout in seconds (env: MARIADB_TIMEOUT)")
	ddlCmd.Flags().IntVar(&ddlMaxRetries, "max-retries", defaultMaxRetries, "Maximum retry attempts for failed queries (env: MARIADB_MAX_RETRIES)")
	ddlCmd.Flags().IntVar(&ddlBatchSize, "batch-size", defaultBatchSize, "Number of databases to process before saving intermediate results (env: MARIADB_BATCH_SIZE)")

	// Only mark as required if not set via environment
	if defaultUser == "" {
		ddlCmd.MarkFlagRequired("user")
	}
	if defaultPassword == "" {
		ddlCmd.MarkFlagRequired("password")
	}
}

func runDDL() {
	// Build connection string with performance optimizations
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/information_schema?charset=utf8mb4&parseTime=true&timeout=%ds&readTimeout=%ds&writeTimeout=%ds&maxAllowedPacket=1073741824",
		ddlUser, ddlPassword, ddlHost, ddlPort, ddlTimeout, ddlTimeout, ddlTimeout)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Configure connection pool for better performance
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Duration(ddlTimeout) * time.Second)

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Printf("Connected to MariaDB at %s:%d (timeout: %ds, batch size: %d)\n", 
		ddlHost, ddlPort, ddlTimeout, ddlBatchSize)

	// Extract DDL information
	ddlStatements, err := extractDDLs(db)
	if err != nil {
		log.Fatalf("Failed to extract DDLs: %v", err)
	}

	// Generate markdown output
	fmt.Printf("\n📝 Generating markdown documentation...\n")
	if err := generateDDLMarkdownOutput(ddlStatements, ddlOutput); err != nil {
		log.Fatalf("Failed to generate DDL markdown output: %v", err)
	}
	fmt.Printf("✅ Created: %s.md\n", ddlOutput)

	// Generate init script for Docker
	fmt.Printf("🔧 Generating SQL init script...\n")
	if err := generateDDLInitScript(ddlStatements); err != nil {
		log.Fatalf("Failed to generate DDL init script: %v", err)
	}
	fmt.Printf("✅ Created: init-scripts/01-extracted-schema.sql\n")

	fmt.Printf("\n🎉 DDL extraction completed successfully!\n")
	fmt.Printf("📁 Files generated:\n")
	fmt.Printf("   - %s.md (documentation)\n", ddlOutput)
	fmt.Printf("   - init-scripts/01-extracted-schema.sql (database setup)\n")
}

func extractDDLs(db *sql.DB) ([]DDLInfo, error) {
	// Get all databases (excluding system databases)
	query := `
		SELECT SCHEMA_NAME
		FROM information_schema.SCHEMATA
		WHERE SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
		ORDER BY SCHEMA_NAME
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query databases: %w", err)
	}
	defer rows.Close()

	var allDDLs []DDLInfo
	var dbNames []string

	// First, collect all database names
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}
		dbNames = append(dbNames, dbName)
	}

	totalDBs := len(dbNames)
	fmt.Printf("Found %d user databases to process\n\n", totalDBs)

	// Process each database with progress tracking
	for i, dbName := range dbNames {
		// Check if this is a "trash" database to skip
		if isTrashDatabase(dbName) {
			fmt.Printf("[%d/%d] ⏭️  Skipping trash database: %s\n", i+1, totalDBs, dbName)
			continue
		}

		fmt.Printf("[%d/%d] 📦 Extracting DDLs from database: %s\n", i+1, totalDBs, dbName)

		// Get all tables for this database
		tableQuery := `
			SELECT TABLE_NAME
			FROM information_schema.TABLES
			WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
			ORDER BY TABLE_NAME
		`

		tableRows, err := db.Query(tableQuery, dbName)
		if err != nil {
			log.Printf("Warning: failed to query tables for %s: %v", dbName, err)
			continue
		}

		for tableRows.Next() {
			var tableName string
			if err := tableRows.Scan(&tableName); err != nil {
				tableRows.Close()
				return nil, fmt.Errorf("failed to scan table name: %w", err)
			}

			// Get CREATE TABLE statement with retry logic
			createTableQuery := fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", dbName, tableName)
			row, err := executeWithRetry(db, createTableQuery)
			if err != nil {
				log.Printf("Warning: failed to get DDL for %s.%s after %d retries: %v", dbName, tableName, ddlMaxRetries, err)
				continue
			}
			
			var table, createTable string
			if err := row.Scan(&table, &createTable); err != nil {
				log.Printf("Warning: failed to scan DDL for %s.%s: %v", dbName, tableName, err)
				continue
			}

			ddlInfo := DDLInfo{
				DatabaseName: dbName,
				TableName:    tableName,
				CreateTable:  createTable,
			}

			allDDLs = append(allDDLs, ddlInfo)
		}
		tableRows.Close()

		fmt.Printf("✅ Completed database: %s\n", dbName)

		// Write intermediate results every N databases to prevent data loss
		if (i+1)%ddlBatchSize == 0 {
			fmt.Printf("💾 Saving intermediate results... (%d/%d databases)\n", i+1, totalDBs)
			if err := generateDDLMarkdownOutput(allDDLs, ddlOutput+".partial"); err != nil {
				fmt.Printf("⚠️  Warning: Failed to save intermediate markdown: %v\n", err)
			}
			if err := generateDDLInitScript(allDDLs); err != nil {
				fmt.Printf("⚠️  Warning: Failed to save intermediate SQL: %v\n", err)
			}
		}
	}

	fmt.Printf("\n🎉 DDL extraction completed! Processed %d databases\n", totalDBs)
	return allDDLs, nil
}

// executeWithRetry executes a database query with retry logic and exponential backoff
func executeWithRetry(db *sql.DB, query string, args ...interface{}) (*sql.Row, error) {
	var row *sql.Row
	var err error
	
	for attempt := 0; attempt < ddlMaxRetries; attempt++ {
		row = db.QueryRow(query, args...)
		// Test the row by attempting to scan into temporary variables
		var test1, test2 string
		if scanErr := row.Scan(&test1, &test2); scanErr != nil {
			err = scanErr
			if attempt < ddlMaxRetries-1 {
				backoffDuration := time.Duration(attempt+1) * time.Second
				fmt.Printf("⚠️  Query failed (attempt %d/%d), retrying in %v: %v\n", 
					attempt+1, ddlMaxRetries, backoffDuration, scanErr)
				time.Sleep(backoffDuration)
				continue
			}
		} else {
			// Query succeeded, return a fresh row for actual use
			return db.QueryRow(query, args...), nil
		}
	}
	
	return nil, fmt.Errorf("query failed after %d attempts: %w", ddlMaxRetries, err)
}

func generateDDLInitScript(ddlStatements []DDLInfo) error {
	// Create output/init-scripts directory if it doesn't exist
	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create init-scripts subdirectory in output
	initScriptsDir := filepath.Join(outputDir, "init-scripts")
	if err := os.MkdirAll(initScriptsDir, 0755); err != nil {
		return fmt.Errorf("failed to create init-scripts directory: %w", err)
	}

	filename := filepath.Join(initScriptsDir, "01-extracted-schema.sql")
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create DDL init script: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "-- MariaDB DDL Init Script\n")
	fmt.Fprintf(file, "-- Auto-generated from production database\n")
	fmt.Fprintf(file, "-- Generated on: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "-- Source: %s:%d\n\n", ddlHost, ddlPort)

	// Disable foreign key checks to allow table creation in any order
	fmt.Fprintf(file, "-- Disable foreign key checks to avoid constraint errors during import\n")
	fmt.Fprintf(file, "SET FOREIGN_KEY_CHECKS=0;\n\n")

	// Group DDLs by database
	dbGroups := make(map[string][]DDLInfo)
	for _, ddl := range ddlStatements {
		dbGroups[ddl.DatabaseName] = append(dbGroups[ddl.DatabaseName], ddl)
	}

	// Write DDLs grouped by database
	for dbName, ddls := range dbGroups {
		fmt.Fprintf(file, "-- Database: %s (%d tables)\n", dbName, len(ddls))
		fmt.Fprintf(file, "CREATE DATABASE IF NOT EXISTS `%s`;\n", dbName)
		fmt.Fprintf(file, "USE `%s`;\n\n", dbName)

		for _, ddl := range ddls {
			// Ensure DDL statement ends with semicolon for proper SQL syntax
			createTableSQL := ddl.CreateTable
			if !strings.HasSuffix(strings.TrimSpace(createTableSQL), ";") {
				createTableSQL += ";"
			}
			fmt.Fprintf(file, "%s\n\n", createTableSQL)
		}

		fmt.Fprintf(file, "-- End of database: %s\n\n", dbName)
	}

	// Re-enable foreign key checks after all tables are created
	fmt.Fprintf(file, "-- Re-enable foreign key checks\n")
	fmt.Fprintf(file, "SET FOREIGN_KEY_CHECKS=1;\n")

	fmt.Printf("✅ DDL init script created: %s\n", filename)
	return nil
}

func generateDDLMarkdownOutput(ddlStatements []DDLInfo, outputPrefix string) error {
	// Ensure output directory exists
	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := filepath.Join(outputDir, fmt.Sprintf("%s.md", outputPrefix))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create DDL markdown file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "# MariaDB DDL Extraction Report\n\n")
	fmt.Fprintf(file, "**Generated on:** %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "**Server:** %s:%d\n\n", ddlHost, ddlPort)
	fmt.Fprintf(file, "**Total DDL Statements:** %d\n\n", len(ddlStatements))
	fmt.Fprintf(file, "---\n\n")

	// Group DDLs by database
	dbGroups := make(map[string][]DDLInfo)
	for _, ddl := range ddlStatements {
		dbGroups[ddl.DatabaseName] = append(dbGroups[ddl.DatabaseName], ddl)
	}

	// Write DDLs grouped by database
	for dbName, ddls := range dbGroups {
		fmt.Fprintf(file, "## Database: `%s`\n\n", dbName)
		fmt.Fprintf(file, "**Tables:** %d\n\n", len(ddls))

		for _, ddl := range ddls {
			fmt.Fprintf(file, "### Table: `%s`\n\n", ddl.TableName)
			fmt.Fprintf(file, "```sql\n")
			fmt.Fprintf(file, "%s\n", ddl.CreateTable)
			fmt.Fprintf(file, "```\n\n")
		}

		fmt.Fprintf(file, "---\n\n")
	}

	return nil
}
