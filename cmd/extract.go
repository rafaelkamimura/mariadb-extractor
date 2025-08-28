/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

// DatabaseInfo represents database information
type DatabaseInfo struct {
	Name        string      `json:"name"`
	TableCount  int         `json:"table_count"`
	Tables      []TableInfo `json:"tables"`
	ExtractedAt string      `json:"extracted_at"`
}

// TableInfo represents table information
type TableInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Engine      string `json:"engine,omitempty"`
	RowCount    int64  `json:"row_count,omitempty"`
	DataLength  int64  `json:"data_length,omitempty"`
	IndexLength int64  `json:"index_length,omitempty"`
	Collation   string `json:"collation,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

// extractCmd represents the extract command
var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract database and table information from MariaDB",
	Long: `Extract database names and table information from MariaDB server.
Generates both markdown (.md) and JSON (.json) output files with
structured information about databases and their tables.`,
	Run: func(cmd *cobra.Command, args []string) {
		runExtract()
	},
}

var (
	host     string
	port     int
	user     string
	password string
	output   string
)

// getEnvWithDefault returns environment variable value or default if not set
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntWithDefault returns environment variable as int or default if not set
func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func init() {
	rootCmd.AddCommand(extractCmd)

	// Get defaults from environment variables
	defaultHost := getEnvWithDefault("MARIADB_HOST", "localhost")
	defaultPort := getEnvIntWithDefault("MARIADB_PORT", 3306)
	defaultUser := os.Getenv("MARIADB_USER")
	defaultPassword := os.Getenv("MARIADB_PASSWORD")
	defaultOutput := getEnvWithDefault("MARIADB_OUTPUT_PREFIX", "mariadb-extract")

	// Database connection flags with environment variable defaults
	extractCmd.Flags().StringVarP(&host, "host", "H", defaultHost, "MariaDB host (env: MARIADB_HOST)")
	extractCmd.Flags().IntVarP(&port, "port", "P", defaultPort, "MariaDB port (env: MARIADB_PORT)")
	extractCmd.Flags().StringVarP(&user, "user", "u", defaultUser, "MariaDB username (env: MARIADB_USER)")
	extractCmd.Flags().StringVarP(&password, "password", "p", defaultPassword, "MariaDB password (env: MARIADB_PASSWORD)")
	extractCmd.Flags().StringVarP(&output, "output", "o", defaultOutput, "Output file prefix (env: MARIADB_OUTPUT_PREFIX)")

	// Only mark as required if not set via environment
	if defaultUser == "" {
		extractCmd.MarkFlagRequired("user")
	}
	if defaultPassword == "" {
		extractCmd.MarkFlagRequired("password")
	}
}

func runExtract() {
	// Build connection string
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/information_schema?charset=utf8mb4&parseTime=true",
		user, password, host, port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Printf("Connected to MariaDB at %s:%d\n", host, port)

	// Extract database information
	databases, err := extractDatabases(db)
	if err != nil {
		log.Fatalf("Failed to extract databases: %v", err)
	}

	// Generate outputs
	if err := generateMarkdownOutput(databases, output); err != nil {
		log.Fatalf("Failed to generate markdown output: %v", err)
	}

	if err := generateJSONOutput(databases, output); err != nil {
		log.Fatalf("Failed to generate JSON output: %v", err)
	}

	fmt.Printf("Extraction completed! Generated %s.md and %s.json\n", output, output)
}

func extractDatabases(db *sql.DB) ([]DatabaseInfo, error) {
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

	var databases []DatabaseInfo

	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}

		fmt.Printf("Extracting database: %s\n", dbName)

		tables, err := extractTables(db, dbName)
		if err != nil {
			log.Printf("Warning: failed to extract tables for %s: %v", dbName, err)
			tables = []TableInfo{}
		}

		database := DatabaseInfo{
			Name:        dbName,
			TableCount:  len(tables),
			Tables:      tables,
			ExtractedAt: time.Now().Format(time.RFC3339),
		}

		databases = append(databases, database)
	}

	return databases, nil
}

func extractTables(db *sql.DB, dbName string) ([]TableInfo, error) {
	query := `
		SELECT
			TABLE_NAME,
			TABLE_TYPE,
			ENGINE,
			TABLE_ROWS,
			DATA_LENGTH,
			INDEX_LENGTH,
			TABLE_COLLATION,
			TABLE_COMMENT
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME
	`

	rows, err := db.Query(query, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo

	for rows.Next() {
		var table TableInfo
		var engine, collation, comment sql.NullString
		var rowCount, dataLength, indexLength sql.NullInt64

		err := rows.Scan(
			&table.Name,
			&table.Type,
			&engine,
			&rowCount,
			&dataLength,
			&indexLength,
			&collation,
			&comment,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table info: %w", err)
		}

		if engine.Valid {
			table.Engine = engine.String
		}
		if rowCount.Valid {
			table.RowCount = rowCount.Int64
		}
		if dataLength.Valid {
			table.DataLength = dataLength.Int64
		}
		if indexLength.Valid {
			table.IndexLength = indexLength.Int64
		}
		if collation.Valid {
			table.Collation = collation.String
		}
		if comment.Valid {
			table.Comment = comment.String
		}

		tables = append(tables, table)
	}

	return tables, nil
}

func generateMarkdownOutput(databases []DatabaseInfo, outputPrefix string) error {
	filename := fmt.Sprintf("%s.md", outputPrefix)
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create markdown file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "# MariaDB Database Extraction Report\n\n")
	fmt.Fprintf(file, "**Generated on:** %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "**Server:** %s:%d\n\n", host, port)
	fmt.Fprintf(file, "**Total Databases:** %d\n\n", len(databases))
	fmt.Fprintf(file, "---\n\n")

	totalTables := 0
	for _, db := range databases {
		totalTables += db.TableCount
	}

	fmt.Fprintf(file, "## Summary\n\n")
	fmt.Fprintf(file, "- **Databases:** %d\n", len(databases))
	fmt.Fprintf(file, "- **Total Tables:** %d\n\n", totalTables)
	fmt.Fprintf(file, "---\n\n")

	// Write database details
	for _, db := range databases {
		fmt.Fprintf(file, "## Database: `%s`\n\n", db.Name)
		fmt.Fprintf(file, "**Tables:** %d\n\n", db.TableCount)

		if len(db.Tables) > 0 {
			fmt.Fprintf(file, "### Tables\n\n")
			fmt.Fprintf(file, "| Table Name | Type | Engine | Rows | Data Size | Index Size | Collation |\n")
			fmt.Fprintf(file, "|-----------|------|--------|------|-----------|------------|-----------|\n")

			for _, table := range db.Tables {
				dataSize := formatBytes(table.DataLength)
				indexSize := formatBytes(table.IndexLength)
				fmt.Fprintf(file, "| `%s` | %s | %s | %d | %s | %s | %s |\n",
					table.Name, table.Type, table.Engine,
					table.RowCount, dataSize, indexSize, table.Collation)
			}
		} else {
			fmt.Fprintf(file, "*No tables found*\n")
		}

		fmt.Fprintf(file, "\n---\n\n")
	}

	return nil
}

func generateJSONOutput(databases []DatabaseInfo, outputPrefix string) error {
	filename := fmt.Sprintf("%s.json", outputPrefix)
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	output := map[string]any{
		"metadata": map[string]any{
			"server":          fmt.Sprintf("%s:%d", host, port),
			"user":            user,
			"extracted_at":    time.Now().Format(time.RFC3339),
			"total_databases": len(databases),
		},
		"databases": databases,
	}

	return encoder.Encode(output)
}

func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	return fmt.Sprintf("%.1f %s", size, units[unitIndex])
}
