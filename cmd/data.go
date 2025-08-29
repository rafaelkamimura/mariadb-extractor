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
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

// ForeignKeyInfo represents a foreign key relationship
type ForeignKeyInfo struct {
	ConstraintName string
	TableName      string
	ColumnName     string
	RefTableName   string
	RefColumnName  string
}

// TableExtractionPlan represents the plan for extracting a single table
type TableExtractionPlan struct {
	DatabaseName string
	TableName    string
	RowCount     int64
	SampleSize   int64
	WhereClause  string
	Dependencies []string // Tables this table depends on
	Order        int      // Extraction order based on dependencies
}

// dataCmd represents the data command
var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "Extract selective data from MariaDB preserving referential integrity",
	Long: `Extract data from MariaDB with advanced filtering, sampling, and foreign key handling.
Preserves referential integrity and allows resumable extractions for large datasets.
This command should be run after DDL extraction to seed your local database with data.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDataExtraction()
	},
}

var (
	// Connection params (reuse from other commands)
	dataHost     string
	dataPort     int
	dataUser     string
	dataPassword string
	dataOutput   string

	// Database selection
	dataDatabases        []string
	dataAllDatabases     bool
	dataAllUserDatabases bool
	dataExcludeDatabases []string

	// Table filtering
	dataIncludeTables []string
	dataExcludeTables []string

	// Data sampling
	dataSampleTables   []string // Format: "table:count"
	dataSamplePercent  int      // Global sample percentage
	dataMaxRowsPerTable int     // Maximum rows per table

	// Performance
	dataChunkSize  int
	dataBatchSize  int
	dataTimeout    int

	// Options
	dataNoForeignKeyCheck bool
	dataProgressInterval  int
	dataResume            string
)

func init() {
	rootCmd.AddCommand(dataCmd)

	// Get defaults from environment variables
	defaultHost := getEnvWithDefault("MARIADB_HOST", "localhost")
	defaultPort := getEnvIntWithDefault("MARIADB_PORT", 3306)
	defaultUser := os.Getenv("MARIADB_USER")
	defaultPassword := os.Getenv("MARIADB_PASSWORD")
	defaultOutput := getEnvWithDefault("MARIADB_OUTPUT_PREFIX", "data-extract")
	defaultTimeout := getEnvIntWithDefault("MARIADB_TIMEOUT", 300)
	defaultChunkSize := getEnvIntWithDefault("MARIADB_CHUNK_SIZE", 10000)
	defaultBatchSize := getEnvIntWithDefault("MARIADB_BATCH_SIZE", 100)

	// Database connection flags
	dataCmd.Flags().StringVarP(&dataHost, "host", "H", defaultHost, "MariaDB host (env: MARIADB_HOST)")
	dataCmd.Flags().IntVarP(&dataPort, "port", "P", defaultPort, "MariaDB port (env: MARIADB_PORT)")
	dataCmd.Flags().StringVarP(&dataUser, "user", "u", defaultUser, "MariaDB username (env: MARIADB_USER)")
	dataCmd.Flags().StringVarP(&dataPassword, "password", "p", defaultPassword, "MariaDB password (env: MARIADB_PASSWORD)")
	dataCmd.Flags().StringVarP(&dataOutput, "output", "o", defaultOutput, "Output file prefix (env: MARIADB_OUTPUT_PREFIX)")

	// Database selection flags
	dataCmd.Flags().StringSliceVarP(&dataDatabases, "databases", "d", []string{}, "Specific databases to extract (comma-separated)")
	dataCmd.Flags().BoolVar(&dataAllDatabases, "all-databases", false, "Extract all databases (including system databases)")
	dataCmd.Flags().BoolVar(&dataAllUserDatabases, "all-user-databases", false, "Extract all user databases (excluding system databases)")
	dataCmd.Flags().StringSliceVar(&dataExcludeDatabases, "exclude-databases", []string{}, "Databases to exclude")

	// Table filtering flags
	dataCmd.Flags().StringSliceVar(&dataIncludeTables, "include-tables", []string{}, "Tables to include (supports wildcards)")
	dataCmd.Flags().StringSliceVar(&dataExcludeTables, "exclude-tables", []string{}, "Tables to exclude (supports wildcards)")

	// Data sampling flags
	dataCmd.Flags().StringSliceVar(&dataSampleTables, "sample-tables", []string{}, "Sample specific tables (format: table:count)")
	dataCmd.Flags().IntVar(&dataSamplePercent, "sample-percent", 0, "Global sample percentage (0-100)")
	dataCmd.Flags().IntVar(&dataMaxRowsPerTable, "max-rows", 0, "Maximum rows per table (0=unlimited)")

	// Performance flags
	dataCmd.Flags().IntVar(&dataChunkSize, "chunk-size", defaultChunkSize, "Rows per chunk for large tables (env: MARIADB_CHUNK_SIZE)")
	dataCmd.Flags().IntVar(&dataBatchSize, "batch-size", defaultBatchSize, "Batch size for INSERT statements (env: MARIADB_BATCH_SIZE)")
	dataCmd.Flags().IntVarP(&dataTimeout, "timeout", "t", defaultTimeout, "Query timeout in seconds (env: MARIADB_TIMEOUT)")

	// Options
	dataCmd.Flags().BoolVar(&dataNoForeignKeyCheck, "no-foreign-key-check", false, "Skip foreign key dependency ordering")
	dataCmd.Flags().IntVar(&dataProgressInterval, "progress-interval", 1000, "Show progress every N rows")
	dataCmd.Flags().StringVar(&dataResume, "resume", "", "Resume extraction with ID")

	// Mark required flags if not set via environment
	if defaultUser == "" {
		dataCmd.MarkFlagRequired("user")
	}
	if defaultPassword == "" {
		dataCmd.MarkFlagRequired("password")
	}
}

func runDataExtraction() {
	// Validate options
	if !dataAllDatabases && !dataAllUserDatabases && len(dataDatabases) == 0 {
		log.Fatal("Must specify one of: --all-databases, --all-user-databases, or --databases")
	}

	if dataAllDatabases && dataAllUserDatabases {
		log.Fatal("Cannot specify both --all-databases and --all-user-databases")
	}

	// Build connection string with timeout
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/information_schema?charset=utf8mb4&parseTime=true&timeout=%ds&readTimeout=%ds&writeTimeout=%ds",
		dataUser, dataPassword, dataHost, dataPort, dataTimeout, dataTimeout, dataTimeout)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Duration(dataTimeout) * time.Second)

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Printf("Connected to MariaDB at %s:%d (timeout: %ds)\n", dataHost, dataPort, dataTimeout)
	fmt.Printf("Data extraction starting...\n\n")

	// Get databases to extract
	databases, err := getDatabasesForExtraction(db)
	if err != nil {
		log.Fatalf("Failed to get databases: %v", err)
	}

	if len(databases) == 0 {
		log.Fatal("No databases found to extract")
	}

	fmt.Printf("Found %d databases to process\n", len(databases))

	// Create extraction plan
	plan, err := createExtractionPlan(db, databases)
	if err != nil {
		log.Fatalf("Failed to create extraction plan: %v", err)
	}

	fmt.Printf("Created extraction plan for %d tables\n", len(plan))

	// Execute extraction
	if err := executeExtractionPlan(db, plan); err != nil {
		log.Fatalf("Failed to execute extraction: %v", err)
	}

	fmt.Printf("\nData extraction completed successfully!\n")
	fmt.Printf("Output file: %s.sql\n", dataOutput)
}

func getDatabasesForExtraction(db *sql.DB) ([]string, error) {
	var databases []string

	if dataAllDatabases {
		// Get all databases
		query := `SELECT SCHEMA_NAME FROM information_schema.SCHEMATA ORDER BY SCHEMA_NAME`
		rows, err := db.Query(query)
		if err != nil {
			return nil, fmt.Errorf("failed to query databases: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var dbName string
			if err := rows.Scan(&dbName); err != nil {
				return nil, fmt.Errorf("failed to scan database name: %w", err)
			}
			databases = append(databases, dbName)
		}
	} else if dataAllUserDatabases {
		// Get user databases only (exclude system databases)
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

		for rows.Next() {
			var dbName string
			if err := rows.Scan(&dbName); err != nil {
				return nil, fmt.Errorf("failed to scan database name: %w", err)
			}
			databases = append(databases, dbName)
		}
	} else {
		// Use specified databases
		databases = dataDatabases
	}

	// Apply exclusions
	if len(dataExcludeDatabases) > 0 {
		filtered := []string{}
		excludeMap := make(map[string]bool)
		for _, exc := range dataExcludeDatabases {
			excludeMap[exc] = true
		}
		for _, db := range databases {
			if !excludeMap[db] {
				filtered = append(filtered, db)
			}
		}
		databases = filtered
	}

	// Filter out trash databases
	finalDatabases := []string{}
	for _, dbName := range databases {
		if !isTrashDatabase(dbName) {
			finalDatabases = append(finalDatabases, dbName)
		}
	}

	return finalDatabases, nil
}

func createExtractionPlan(db *sql.DB, databases []string) ([]TableExtractionPlan, error) {
	var allPlans []TableExtractionPlan

	for _, dbName := range databases {
		fmt.Printf("Analyzing database: %s\n", dbName)

		// Get tables for this database
		tables, err := getTablesForDatabase(db, dbName)
		if err != nil {
			log.Printf("Warning: Failed to get tables for %s: %v", dbName, err)
			continue
		}

		// Get foreign key relationships if needed
		var foreignKeys map[string][]ForeignKeyInfo
		if !dataNoForeignKeyCheck {
			foreignKeys, err = getForeignKeyRelationships(db, dbName)
			if err != nil {
				log.Printf("Warning: Failed to get foreign keys for %s: %v", dbName, err)
			}
		}

		// Create extraction plan for each table
		tablePlans := createTableExtractionPlans(dbName, tables, foreignKeys)
		allPlans = append(allPlans, tablePlans...)
	}

	// Sort by dependencies if foreign key checking is enabled
	if !dataNoForeignKeyCheck {
		allPlans = sortByDependencies(allPlans)
	}

	return allPlans, nil
}

func getTablesForDatabase(db *sql.DB, dbName string) ([]string, error) {
	query := `
		SELECT TABLE_NAME 
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`

	rows, err := db.Query(query, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}

		// Apply include/exclude filters
		if shouldIncludeTable(tableName) {
			tables = append(tables, tableName)
		}
	}

	return tables, nil
}

func shouldIncludeTable(tableName string) bool {
	// Check exclude patterns first
	for _, pattern := range dataExcludeTables {
		if matchesPattern(tableName, pattern) {
			return false
		}
	}

	// If include patterns specified, table must match one
	if len(dataIncludeTables) > 0 {
		for _, pattern := range dataIncludeTables {
			if matchesPattern(tableName, pattern) {
				return true
			}
		}
		return false
	}

	// Default: include the table
	return true
}

func matchesPattern(text, pattern string) bool {
	// Simple wildcard matching (* = any characters)
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = "^" + pattern + "$"
	// This is a simplified pattern matcher
	// In production, you'd want to use regexp.MatchString
	return strings.Contains(text, strings.ReplaceAll(strings.ReplaceAll(pattern, "^.*", ""), ".*$", ""))
}

func getForeignKeyRelationships(db *sql.DB, dbName string) (map[string][]ForeignKeyInfo, error) {
	query := `
		SELECT 
			CONSTRAINT_NAME,
			TABLE_NAME,
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ?
			AND REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY TABLE_NAME, ORDINAL_POSITION
	`

	rows, err := db.Query(query, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer rows.Close()

	foreignKeys := make(map[string][]ForeignKeyInfo)
	for rows.Next() {
		var fk ForeignKeyInfo
		if err := rows.Scan(&fk.ConstraintName, &fk.TableName, &fk.ColumnName, 
			&fk.RefTableName, &fk.RefColumnName); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}
		foreignKeys[fk.TableName] = append(foreignKeys[fk.TableName], fk)
	}

	return foreignKeys, nil
}

func createTableExtractionPlans(dbName string, tables []string, foreignKeys map[string][]ForeignKeyInfo) []TableExtractionPlan {
	var plans []TableExtractionPlan

	// Parse sample table specifications
	sampleMap := make(map[string]int64)
	for _, spec := range dataSampleTables {
		parts := strings.Split(spec, ":")
		if len(parts) == 2 {
			if count, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				sampleMap[parts[0]] = count
			}
		}
	}

	for _, tableName := range tables {
		plan := TableExtractionPlan{
			DatabaseName: dbName,
			TableName:    tableName,
		}

		// Set sample size
		if sampleSize, ok := sampleMap[tableName]; ok {
			plan.SampleSize = sampleSize
		} else if dataSamplePercent > 0 {
			// Will be calculated based on actual row count later
			plan.SampleSize = -int64(dataSamplePercent) // Negative indicates percentage
		} else if dataMaxRowsPerTable > 0 {
			plan.SampleSize = int64(dataMaxRowsPerTable)
		}

		// Set dependencies
		if fks, ok := foreignKeys[tableName]; ok {
			for _, fk := range fks {
				// Only add dependency if it's a different table
				if fk.RefTableName != tableName {
					plan.Dependencies = append(plan.Dependencies, fk.RefTableName)
				}
			}
		}

		plans = append(plans, plan)
	}

	return plans
}

func sortByDependencies(plans []TableExtractionPlan) []TableExtractionPlan {
	// Simple topological sort for foreign key dependencies
	// This is a basic implementation - in production you'd want cycle detection
	
	sorted := make([]TableExtractionPlan, 0, len(plans))
	visited := make(map[string]bool)
	
	var visit func(string) 
	visit = func(tableName string) {
		if visited[tableName] {
			return
		}
		visited[tableName] = true
		
		// Find the plan for this table
		for _, plan := range plans {
			if plan.TableName == tableName {
				// Visit dependencies first
				for _, dep := range plan.Dependencies {
					visit(dep)
				}
				sorted = append(sorted, plan)
				break
			}
		}
	}
	
	// Visit all tables
	for _, plan := range plans {
		visit(plan.TableName)
	}
	
	return sorted
}

func executeExtractionPlan(db *sql.DB, plans []TableExtractionPlan) error {
	// Ensure output directory exists
	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load progress if resuming
	var completedTables map[string]bool
	if dataResume != "" {
		completedTables = loadExtractionProgress()
		fmt.Printf("Resuming extraction with %d completed tables\n", len(completedTables))
	} else {
		completedTables = make(map[string]bool)
	}

	// Create or append to output file
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s.sql", dataOutput))
	var file *os.File
	var err error
	if dataResume != "" && len(completedTables) > 0 {
		file, err = os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			file, err = os.Create(outputFile)
		}
	} else {
		file, err = os.Create(outputFile)
	}
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Write header (only if new file)
	if dataResume == "" || len(completedTables) == 0 {
		fmt.Fprintf(file, "-- MariaDB Data Extract\n")
		fmt.Fprintf(file, "-- Generated on: %s\n", time.Now().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(file, "-- Source: %s:%d\n\n", dataHost, dataPort)

		// Disable foreign key checks for import
		fmt.Fprintf(file, "-- Disable foreign key checks for data import\n")
		fmt.Fprintf(file, "SET FOREIGN_KEY_CHECKS=0;\n\n")
	}

	// Track progress
	totalTables := len(plans)
	startTime := time.Now()
	successCount := len(completedTables)
	failCount := 0

	// Execute extraction for each table
	for i, plan := range plans {
		tableKey := fmt.Sprintf("%s.%s", plan.DatabaseName, plan.TableName)
		
		// Skip if already completed
		if completedTables[tableKey] {
			fmt.Printf("[%d/%d] Skipping %s (already completed)\n", i+1, totalTables, tableKey)
			continue
		}

		tableStartTime := time.Now()
		fmt.Printf("[%d/%d] Extracting %s.%s", i+1, totalTables, plan.DatabaseName, plan.TableName)

		// Get actual row count
		rowCount, err := getTableRowCount(db, plan.DatabaseName, plan.TableName)
		if err != nil {
			log.Printf(" - Warning: Failed to get row count: %v", err)
			rowCount = 0
		}
		plan.RowCount = rowCount

		// Calculate sample size if percentage-based
		if plan.SampleSize < 0 {
			percentage := -plan.SampleSize
			plan.SampleSize = (rowCount * int64(percentage)) / 100
		}

		// Determine extraction size
		extractSize := rowCount
		if plan.SampleSize > 0 && plan.SampleSize < rowCount {
			extractSize = plan.SampleSize
			fmt.Printf(" (sampling %d of %d rows)", extractSize, rowCount)
		} else {
			fmt.Printf(" (%d rows)", rowCount)
		}

		// Extract table data
		if err := extractTableData(db, file, plan); err != nil {
			fmt.Printf(" - Failed: %v\n", err)
			failCount++
			// Continue with next table even if one fails
			continue
		}

		// Mark as completed
		successCount++
		saveExtractionProgress(tableKey)

		duration := time.Since(tableStartTime)
		fmt.Printf(" - Completed in %v\n", duration.Round(time.Millisecond))

		// Show overall progress
		elapsed := time.Since(startTime)
		avgTimePerTable := elapsed / time.Duration(successCount)
		remainingTables := totalTables - (i + 1)
		eta := time.Duration(remainingTables) * avgTimePerTable
		fmt.Printf("Progress: %d/%d tables | Elapsed: %v | ETA: %v\n\n", 
			i+1, totalTables, elapsed.Round(time.Second), eta.Round(time.Second))
	}

	// Re-enable foreign key checks
	fmt.Fprintf(file, "\n-- Re-enable foreign key checks\n")
	fmt.Fprintf(file, "SET FOREIGN_KEY_CHECKS=1;\n")

	totalDuration := time.Since(startTime)
	fmt.Printf("\nExtraction Summary:\n")
	fmt.Printf("  Total tables: %d\n", totalTables)
	fmt.Printf("  Successful: %d\n", successCount)
	fmt.Printf("  Failed: %d\n", failCount)
	fmt.Printf("  Total time: %v\n", totalDuration.Round(time.Second))

	return nil
}

// Progress tracking functions
func loadExtractionProgress() map[string]bool {
	progressFile := dataOutput + ".progress"
	completedTables := make(map[string]bool)

	if data, err := os.ReadFile(progressFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if line = strings.TrimSpace(line); line != "" {
				completedTables[line] = true
			}
		}
	}

	return completedTables
}

func saveExtractionProgress(tableKey string) {
	progressFile := dataOutput + ".progress"
	
	// Read existing progress
	completedTables := loadExtractionProgress()
	completedTables[tableKey] = true

	// Write back to file
	var lines []string
	for table := range completedTables {
		lines = append(lines, table)
	}
	sort.Strings(lines)

	data := strings.Join(lines, "\n") + "\n"
	os.WriteFile(progressFile, []byte(data), 0644)
}

func getTableRowCount(db *sql.DB, dbName, tableName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`.`%s`", dbName, tableName)
	var count int64
	err := db.QueryRow(query).Scan(&count)
	return count, err
}

func extractTableData(db *sql.DB, file *os.File, plan TableExtractionPlan) error {
	// Write table header
	fmt.Fprintf(file, "-- Table: %s.%s\n", plan.DatabaseName, plan.TableName)
	fmt.Fprintf(file, "USE `%s`;\n", plan.DatabaseName)

	// Build query
	query := fmt.Sprintf("SELECT * FROM `%s`.`%s`", plan.DatabaseName, plan.TableName)
	
	// Add LIMIT for sampling
	if plan.SampleSize > 0 && plan.SampleSize < plan.RowCount {
		query += fmt.Sprintf(" LIMIT %d", plan.SampleSize)
	}

	// Execute query
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query table data: %w", err)
	}
	defer rows.Close()

	// Get column information
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Prepare scan destinations
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Process rows in batches
	batchCount := 0
	rowCount := 0
	var batchValues []string

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert row to SQL values
		rowValues := make([]string, len(columns))
		for i, v := range values {
			rowValues[i] = formatSQLValue(v)
		}

		batchValues = append(batchValues, fmt.Sprintf("(%s)", strings.Join(rowValues, ",")))
		batchCount++
		rowCount++

		// Write batch if full
		if batchCount >= dataBatchSize {
			fmt.Fprintf(file, "INSERT INTO `%s` VALUES\n%s;\n", 
				plan.TableName, strings.Join(batchValues, ",\n"))
			batchValues = nil
			batchCount = 0
		}

		// Show progress
		if rowCount%dataProgressInterval == 0 {
			fmt.Printf(".")
		}
	}

	// Write remaining batch
	if batchCount > 0 {
		fmt.Fprintf(file, "INSERT INTO `%s` VALUES\n%s;\n", 
			plan.TableName, strings.Join(batchValues, ",\n"))
	}

	fmt.Fprintf(file, "\n")
	return nil
}

func formatSQLValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}

	switch val := v.(type) {
	case []byte:
		// Escape string values
		str := string(val)
		str = strings.ReplaceAll(str, "\\", "\\\\")
		str = strings.ReplaceAll(str, "'", "\\'")
		str = strings.ReplaceAll(str, "\n", "\\n")
		str = strings.ReplaceAll(str, "\r", "\\r")
		str = strings.ReplaceAll(str, "\t", "\\t")
		return fmt.Sprintf("'%s'", str)
	case string:
		// Escape string values
		str := val
		str = strings.ReplaceAll(str, "\\", "\\\\")
		str = strings.ReplaceAll(str, "'", "\\'")
		str = strings.ReplaceAll(str, "\n", "\\n")
		str = strings.ReplaceAll(str, "\r", "\\r")
		str = strings.ReplaceAll(str, "\t", "\\t")
		return fmt.Sprintf("'%s'", str)
	case time.Time:
		return fmt.Sprintf("'%s'", val.Format("2006-01-02 15:04:05"))
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%f", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	default:
		// Default to string representation
		return fmt.Sprintf("'%v'", val)
	}
}