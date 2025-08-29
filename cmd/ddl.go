/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"database/sql"
	"fmt"
	"log"
	"os"
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
	ddlHost     string
	ddlPort     int
	ddlUser     string
	ddlPassword string
	ddlOutput   string
)

func init() {
	rootCmd.AddCommand(ddlCmd)

	// Get defaults from environment variables
	defaultHost := getEnvWithDefault("MARIADB_HOST", "localhost")
	defaultPort := getEnvIntWithDefault("MARIADB_PORT", 3306)
	defaultUser := os.Getenv("MARIADB_USER")
	defaultPassword := os.Getenv("MARIADB_PASSWORD")
	defaultOutput := getEnvWithDefault("MARIADB_OUTPUT_PREFIX", "mariadb-ddl")

	// Database connection flags with environment variable defaults
	ddlCmd.Flags().StringVarP(&ddlHost, "host", "H", defaultHost, "MariaDB host (env: MARIADB_HOST)")
	ddlCmd.Flags().IntVarP(&ddlPort, "port", "P", defaultPort, "MariaDB port (env: MARIADB_PORT)")
	ddlCmd.Flags().StringVarP(&ddlUser, "user", "u", defaultUser, "MariaDB username (env: MARIADB_USER)")
	ddlCmd.Flags().StringVarP(&ddlPassword, "password", "p", defaultPassword, "MariaDB password (env: MARIADB_PASSWORD)")
	ddlCmd.Flags().StringVarP(&ddlOutput, "output", "o", defaultOutput, "Output file prefix (env: MARIADB_OUTPUT_PREFIX)")

	// Only mark as required if not set via environment
	if defaultUser == "" {
		ddlCmd.MarkFlagRequired("user")
	}
	if defaultPassword == "" {
		ddlCmd.MarkFlagRequired("password")
	}
}

func runDDL() {
	// Build connection string
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/information_schema?charset=utf8mb4&parseTime=true",
		ddlUser, ddlPassword, ddlHost, ddlPort)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Printf("Connected to MariaDB at %s:%d\n", ddlHost, ddlPort)

	// Extract DDL information
	ddlStatements, err := extractDDLs(db)
	if err != nil {
		log.Fatalf("Failed to extract DDLs: %v", err)
	}

	// Generate markdown output
	if err := generateDDLMarkdownOutput(ddlStatements, ddlOutput); err != nil {
		log.Fatalf("Failed to generate DDL markdown output: %v", err)
	}

	fmt.Printf("DDL extraction completed! Generated %s.md\n", ddlOutput)
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

	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}

		fmt.Printf("Extracting DDLs from database: %s\n", dbName)

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

			// Get CREATE TABLE statement
			createTableQuery := fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", dbName, tableName)
			var table, createTable string
			err := db.QueryRow(createTableQuery).Scan(&table, &createTable)
			if err != nil {
				log.Printf("Warning: failed to get DDL for %s.%s: %v", dbName, tableName, err)
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
	}

	return allDDLs, nil
}

func generateDDLMarkdownOutput(ddlStatements []DDLInfo, outputPrefix string) error {
	filename := fmt.Sprintf("%s.md", outputPrefix)
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
