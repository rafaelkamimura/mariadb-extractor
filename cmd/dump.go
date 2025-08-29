/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

// dumpCmd represents the dump command
var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Create full database dumps using mysqldump",
	Long: `Create complete database dumps using mysqldump for local development and backup.
Supports dumping schema only, data only, or both. Can dump all databases or specific ones.
Generated dumps can be used to recreate databases locally with 'mysql < dump.sql'.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDump()
	},
}

var (
	dumpHost             string
	dumpPort             int
	dumpUser             string
	dumpPassword         string
	dumpOutput           string
	dumpDatabases        []string
	dumpSchemaOnly       bool
	dumpDataOnly         bool
	dumpAllDatabases     bool
	dumpAllUserDatabases bool
	dumpCompress         bool
)

func init() {
	rootCmd.AddCommand(dumpCmd)

	// Get defaults from environment variables
	defaultHost := getEnvWithDefault("MARIADB_HOST", "localhost")
	defaultPort := getEnvIntWithDefault("MARIADB_PORT", 3306)
	defaultUser := os.Getenv("MARIADB_USER")
	defaultPassword := os.Getenv("MARIADB_PASSWORD")
	defaultOutput := getEnvWithDefault("MARIADB_OUTPUT_PREFIX", "mariadb-dump")

	// Database connection flags with environment variable defaults
	dumpCmd.Flags().StringVarP(&dumpHost, "host", "H", defaultHost, "MariaDB host (env: MARIADB_HOST)")
	dumpCmd.Flags().IntVarP(&dumpPort, "port", "P", defaultPort, "MariaDB port (env: MARIADB_PORT)")
	dumpCmd.Flags().StringVarP(&dumpUser, "user", "u", defaultUser, "MariaDB username (env: MARIADB_USER)")
	dumpCmd.Flags().StringVarP(&dumpPassword, "password", "p", defaultPassword, "MariaDB password (env: MARIADB_PASSWORD)")
	dumpCmd.Flags().StringVarP(&dumpOutput, "output", "o", defaultOutput, "Output file prefix (env: MARIADB_OUTPUT_PREFIX)")

	// Dump-specific flags
	dumpCmd.Flags().StringSliceVarP(&dumpDatabases, "databases", "d", []string{}, "Specific databases to dump (comma-separated)")
	dumpCmd.Flags().BoolVar(&dumpAllDatabases, "all-databases", false, "Dump all databases (including system databases)")
	dumpCmd.Flags().BoolVar(&dumpAllUserDatabases, "all-user-databases", false, "Dump all user databases (excluding system databases)")
	dumpCmd.Flags().BoolVar(&dumpSchemaOnly, "schema-only", false, "Dump only schema (no data)")
	dumpCmd.Flags().BoolVar(&dumpDataOnly, "data-only", false, "Dump only data (no schema)")
	dumpCmd.Flags().BoolVarP(&dumpCompress, "compress", "c", false, "Compress output with gzip")

	// Only mark as required if not set via environment
	if defaultUser == "" {
		dumpCmd.MarkFlagRequired("user")
	}
	if defaultPassword == "" {
		dumpCmd.MarkFlagRequired("password")
	}
}

func runDump() {
	// Validate dump options
	if dumpSchemaOnly && dumpDataOnly {
		log.Fatal("Cannot specify both --schema-only and --data-only")
	}

	if dumpAllDatabases && dumpAllUserDatabases {
		log.Fatal("Cannot specify both --all-databases and --all-user-databases")
	}

	if !dumpAllDatabases && !dumpAllUserDatabases && len(dumpDatabases) == 0 {
		log.Fatal("Must specify one of: --all-databases, --all-user-databases, or --databases")
	}

	if (dumpAllDatabases || dumpAllUserDatabases) && len(dumpDatabases) > 0 {
		log.Fatal("Cannot specify both --all-* flags and --databases")
	}

	fmt.Printf("Starting database dump from %s:%d\n", dumpHost, dumpPort)

	// Build mysqldump command
	args := buildMysqldumpArgs()

	// Execute mysqldump
	if err := executeMysqldump(args); err != nil {
		log.Fatalf("Failed to execute mysqldump: %v", err)
	}

	fmt.Printf("Database dump completed successfully!\n")
}

func buildMysqldumpArgs() []string {
	var args []string

	// Connection parameters
	args = append(args, "-h", dumpHost)
	args = append(args, "-P", strconv.Itoa(dumpPort))
	args = append(args, "-u", dumpUser)

	// Password (passed via environment to avoid command line exposure)
	os.Setenv("MYSQL_PWD", dumpPassword)

	// Dump options
	if dumpSchemaOnly {
		args = append(args, "--no-data")
	} else if dumpDataOnly {
		args = append(args, "--no-create-info")
	}

	// Add other useful options
	args = append(args, "--single-transaction") // Consistent snapshot
	args = append(args, "--quick")              // Don't buffer entire result sets
	args = append(args, "--lock-tables=false")  // Don't lock tables
	args = append(args, "--routines")           // Include stored procedures and functions
	args = append(args, "--triggers")           // Include triggers

	// Database selection
	if dumpAllDatabases {
		args = append(args, "--all-databases")
		fmt.Printf("Dumping ALL databases (including system databases)...\n")
	} else if dumpAllUserDatabases {
		// Get list of user databases (excluding system databases)
		userDBs, err := getUserDatabases()
		if err != nil {
			log.Fatalf("Failed to get user databases: %v", err)
		}
		if len(userDBs) == 0 {
			log.Fatal("No user databases found to dump")
		}

		// Process databases individually for progress tracking
		fmt.Printf("Found %d user databases to dump\n", len(userDBs))
		if err := dumpDatabasesWithProgress(userDBs); err != nil {
			log.Fatalf("Failed to dump databases: %v", err)
		}
		return nil // Early return since we handled the dump
	} else if len(dumpDatabases) > 0 {
		// If multiple databases specified, use progress mode
		if len(dumpDatabases) > 1 {
			fmt.Printf("Dumping %d specified databases with progress tracking\n", len(dumpDatabases))
			if err := dumpDatabasesWithProgress(dumpDatabases); err != nil {
				log.Fatalf("Failed to dump databases: %v", err)
			}
			return nil // Early return since we handled the dump
		} else {
			// Single database - use regular mode
			fmt.Printf("Dumping database: %s\n", dumpDatabases[0])
			args = append(args, dumpDatabases[0])
		}
	}

	return args
}

func getUserDatabases() ([]string, error) {
	// Build connection string
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/information_schema?charset=utf8mb4&parseTime=true",
		dumpUser, dumpPassword, dumpHost, dumpPort)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Get all user databases (excluding system databases)
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

	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}
		databases = append(databases, dbName)
	}

	return databases, nil
}

func dumpDatabasesWithProgress(databases []string) error {
	totalDBs := len(databases)
	fmt.Printf("Starting dump of %d databases...\n\n", totalDBs)

	startTime := time.Now()
	var successfulDumps, failedDumps int

	for i, dbName := range databases {
		dbStartTime := time.Now()
		fmt.Printf("[%d/%d] Dumping database: %s\n", i+1, totalDBs, dbName)

		// Build mysqldump args for this specific database
		args := []string{
			"-h", dumpHost,
			"-P", strconv.Itoa(dumpPort),
			"-u", dumpUser,
			"--single-transaction",
			"--quick",
			"--lock-tables=false",
			"--routines",
			"--triggers",
		}

		// Add schema/data options
		if dumpSchemaOnly {
			args = append(args, "--no-data")
		} else if dumpDataOnly {
			args = append(args, "--no-create-info")
		}

		// Add the database name
		args = append(args, dbName)

		// Execute mysqldump for this database
		if err := executeMysqldumpForDB(args, dbName, dumpPassword, i+1, totalDBs); err != nil {
			fmt.Printf("❌ Failed to dump %s: %v\n", dbName, err)
			failedDumps++
			// Continue with next database even if this one fails
		} else {
			dbDuration := time.Since(dbStartTime)
			fmt.Printf("✅ Completed %s in %v\n", dbName, dbDuration.Round(time.Second))
			successfulDumps++
		}

		// Show progress
		elapsed := time.Since(startTime)
		avgTimePerDB := elapsed / time.Duration(i+1)
		remaining := time.Duration(totalDBs-i-1) * avgTimePerDB
		fmt.Printf("Progress: %d/%d completed | Elapsed: %v | ETA: %v\n\n",
			i+1, totalDBs, elapsed.Round(time.Second), remaining.Round(time.Second))
	}

	// Final summary
	totalDuration := time.Since(startTime)
	fmt.Printf("🎉 Dump Summary:\n")
	fmt.Printf("   Total databases: %d\n", totalDBs)
	fmt.Printf("   Successful: %d\n", successfulDumps)
	fmt.Printf("   Failed: %d\n", failedDumps)
	fmt.Printf("   Total time: %v\n", totalDuration.Round(time.Second))
	fmt.Printf("   Average per database: %v\n", (totalDuration / time.Duration(totalDBs)).Round(time.Second))

	if failedDumps > 0 {
		return fmt.Errorf("dump completed with %d failures", failedDumps)
	}

	return nil
}

func executeMysqldumpForDB(args []string, dbName string, password string, current, total int) error {
	// Determine output file
	outputFile := dumpOutput
	if dumpCompress {
		outputFile += ".sql.gz"
	} else {
		outputFile += ".sql"
	}

	// For multiple databases, append to the same file
	file, err := os.OpenFile(outputFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer file.Close()

	// Add database header to the dump file
	header := fmt.Sprintf("\n-- Database: %s\n-- Dumped at: %s\n\n", dbName, time.Now().Format("2006-01-02 15:04:05"))
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Create a temporary my.cnf file for secure password passing
	tmpFile, err := os.CreateTemp("", "mariadb-extractor-*.cnf")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write MySQL config with credentials
	configContent := fmt.Sprintf(`[client]
host=%s
port=%d
user=%s
password=%s
`, dumpHost, dumpPort, dumpUser, password)

	if _, err := tmpFile.WriteString(configContent); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	tmpFile.Close()

	// Add --defaults-file to use our secure config
	secureArgs := append([]string{"--defaults-file=" + tmpFile.Name()}, args...)

	// Create the mysqldump command
	cmd := exec.Command("mysqldump", secureArgs...)

	// Set up output
	cmd.Stdout = file
	cmd.Stderr = os.Stderr

	// Execute the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w", err)
	}

	return nil
}

func executeMysqldump(args []string) error {
	// Check if mysqldump is available
	if _, err := exec.LookPath("mysqldump"); err != nil {
		return fmt.Errorf("mysqldump not found in PATH. Please install MariaDB/MySQL client tools:\n\n" +
			"  Ubuntu/Debian: sudo apt-get install mariadb-client\n" +
			"  CentOS/RHEL: sudo yum install mariadb\n" +
			"  macOS: brew install mariadb\n" +
			"  Or download from: https://mariadb.com/downloads/")
	}

	// Determine output file
	outputFile := dumpOutput
	if dumpCompress {
		outputFile += ".sql.gz"
	} else {
		outputFile += ".sql"
	}

	// Create a temporary my.cnf file for secure password passing
	tmpFile, err := os.CreateTemp("", "mariadb-extractor-*.cnf")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write MySQL config with credentials
	configContent := fmt.Sprintf(`[client]
host=%s
port=%d
user=%s
password=%s
`, dumpHost, dumpPort, dumpUser, dumpPassword)

	if _, err := tmpFile.WriteString(configContent); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	tmpFile.Close()

	// Add --defaults-file to use our secure config
	secureArgs := append([]string{"--defaults-file=" + tmpFile.Name()}, args...)

	fmt.Printf("Executing: mysqldump --defaults-file=**** %s > %s\n", strings.Join(args, " "), outputFile)

	// Create the mysqldump command
	cmd := exec.Command("mysqldump", secureArgs...)

	// Set up output file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// If compression is requested, pipe through gzip
	if dumpCompress {
		// Check if gzip is available
		if _, err := exec.LookPath("gzip"); err != nil {
			return fmt.Errorf("gzip not found in PATH. Please install gzip compression:\n\n" +
				"  Ubuntu/Debian: sudo apt-get install gzip\n" +
				"  CentOS/RHEL: sudo yum install gzip\n" +
				"  macOS: gzip is usually pre-installed")
		}

		gzipCmd := exec.Command("gzip")
		gzipCmd.Stdout = file

		// Pipe mysqldump output to gzip
		gzipCmd.Stdin, err = cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}

		// Start gzip first
		if err := gzipCmd.Start(); err != nil {
			return fmt.Errorf("failed to start gzip: %w", err)
		}

		// Start mysqldump
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start mysqldump: %w", err)
		}

		// Wait for mysqldump to complete
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("mysqldump failed: %w", err)
		}

		// Wait for gzip to complete
		if err := gzipCmd.Wait(); err != nil {
			return fmt.Errorf("gzip failed: %w", err)
		}
	} else {
		// Direct output to file
		cmd.Stdout = file

		// Execute mysqldump
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("mysqldump failed: %w", err)
		}
	}

	return nil
}
