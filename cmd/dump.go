/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

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
	dumpHost         string
	dumpPort         int
	dumpUser         string
	dumpPassword     string
	dumpOutput       string
	dumpDatabases    []string
	dumpSchemaOnly   bool
	dumpDataOnly     bool
	dumpAllDatabases bool
	dumpCompress     bool
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
	dumpCmd.Flags().BoolVar(&dumpAllDatabases, "all-databases", false, "Dump all databases")
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

	if !dumpAllDatabases && len(dumpDatabases) == 0 {
		log.Fatal("Must specify either --all-databases or --databases")
	}

	if dumpAllDatabases && len(dumpDatabases) > 0 {
		log.Fatal("Cannot specify both --all-databases and --databases")
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
	} else if len(dumpDatabases) > 0 {
		args = append(args, strings.Join(dumpDatabases, " "))
	}

	return args
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

	fmt.Printf("Executing: mysqldump %s > %s\n", strings.Join(args, " "), outputFile)

	// Create the mysqldump command
	cmd := exec.Command("mysqldump", args...)

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

	fmt.Printf("Dump saved to: %s\n", outputFile)
	return nil
}
