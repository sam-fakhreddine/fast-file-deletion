// Package main provides the command-line interface for the Fast File Deletion tool.
// This tool is optimized for Windows systems and provides high-performance deletion
// of directories containing millions of small files using parallel processing and
// direct Windows API calls.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/yourusername/fast-file-deletion/internal/backend"
	"github.com/yourusername/fast-file-deletion/internal/engine"
	"github.com/yourusername/fast-file-deletion/internal/logger"
	"github.com/yourusername/fast-file-deletion/internal/progress"
	"github.com/yourusername/fast-file-deletion/internal/safety"
	"github.com/yourusername/fast-file-deletion/internal/scanner"
)

// Config holds the parsed command-line configuration.
type Config struct {
	TargetDir string
	Force     bool
	DryRun    bool
	Verbose   bool
	LogFile   string
	KeepDays  *int
	Workers   int
}

func main() {
	// Parse command-line arguments
	config, err := parseArguments()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		printUsage()
		os.Exit(2)
	}

	// If no target directory specified, show usage and exit
	if config == nil {
		printUsage()
		os.Exit(0)
	}

	// Run the main deletion workflow
	exitCode := run(config)
	os.Exit(exitCode)
}

// parseArguments parses and validates command-line arguments.
// Returns the parsed Config, or an error if arguments are invalid.
// Returns nil config if help was requested (no error).
func parseArguments() (*Config, error) {
	// Define flags
	targetDir := flag.String("target-directory", "", "Directory to delete (required)")
	flag.StringVar(targetDir, "td", "", "Directory to delete (shorthand)")
	force := flag.Bool("force", false, "Skip confirmation prompts")
	dryRun := flag.Bool("dry-run", false, "Simulate deletion without actually deleting")
	verbose := flag.Bool("verbose", false, "Enable detailed logging")
	logFile := flag.String("log-file", "", "Write logs to specified file")
	keepDays := flag.Int("keep-days", -1, "Only delete files older than N days")
	workers := flag.Int("workers", 0, "Number of parallel workers (default: auto-detect)")

	// Custom usage function
	flag.Usage = printUsage

	// Parse flags
	flag.Parse()

	// Check if target directory was provided
	if *targetDir == "" {
		// Check if user provided positional arguments (old syntax)
		if flag.NArg() > 0 {
			return nil, fmt.Errorf("positional arguments are not supported\n"+
				"   Use --target-directory or -td flag instead\n"+
				"   Example: fast-file-deletion -td \"%s\"", flag.Arg(0))
		}
		// No target directory, return nil to show usage
		return nil, nil
	}

	// Check for unexpected positional arguments
	if flag.NArg() > 0 {
		return nil, fmt.Errorf("unexpected positional arguments: %v\n"+
			"   All options must be specified as flags", flag.Args())
	}

	// Validate argument combinations
	if *keepDays < -1 {
		return nil, fmt.Errorf("invalid --keep-days value: must be >= 0 (got %d)", *keepDays)
	}

	if *workers < 0 {
		return nil, fmt.Errorf("invalid --workers value: must be >= 0 (got %d)", *workers)
	}

	// Convert keepDays to pointer (nil if not specified)
	var keepDaysPtr *int
	if *keepDays >= 0 {
		keepDaysPtr = keepDays
	}

	config := &Config{
		TargetDir: *targetDir,
		Force:     *force,
		DryRun:    *dryRun,
		Verbose:   *verbose,
		LogFile:   *logFile,
		KeepDays:  keepDaysPtr,
		Workers:   *workers,
	}

	return config, nil
}

// printUsage displays usage information and examples.
func printUsage() {
	fmt.Println("Fast File Deletion Tool")
	fmt.Println("Version: 0.1.0")
	fmt.Println()
	fmt.Println("Usage: fast-file-deletion --target-directory <path> [options]")
	fmt.Println("   or: fast-file-deletion -td <path> [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --target-directory PATH")
	fmt.Println("  -td PATH                Directory to delete (required)")
	fmt.Println("  --force                 Skip confirmation prompts")
	fmt.Println("  --dry-run               Simulate deletion without actually deleting")
	fmt.Println("  --verbose               Enable detailed logging")
	fmt.Println("  --log-file PATH         Write logs to specified file")
	fmt.Println("  --keep-days N           Only delete files older than N days")
	fmt.Println("  --workers N             Number of parallel workers (default: auto-detect)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  fast-file-deletion -td C:\\temp\\old-logs")
	fmt.Println("  fast-file-deletion -td \"C:\\Program Files\\old-cache\" --force")
	fmt.Println("  fast-file-deletion --target-directory C:\\temp\\cache --dry-run")
	fmt.Println("  fast-file-deletion -td C:\\data\\archive --keep-days 30 --verbose")
	fmt.Println("  fast-file-deletion -td \"/tmp/old data\" --workers 8 --log-file deletion.log")
}

// run executes the main deletion workflow with the given configuration.
// Returns an exit code: 0 for success, 1 for partial failure, 2 for complete failure.
func run(config *Config) int {
	// Setup logging first
	err := logger.SetupLogging(config.Verbose, config.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to setup logging: %v\n", err)
		// Continue anyway with default logging
	}
	defer logger.Close()

	logger.Info("Fast File Deletion Tool v0.1.0")
	logger.Info("Target directory: %s", config.TargetDir)

	// Display platform-specific warning for non-Windows systems
	if runtime.GOOS != "windows" {
		fmt.Println()
		fmt.Println("⚠️  Note: This tool is optimized for Windows systems.")
		fmt.Println("   Performance optimizations are Windows-specific.")
		fmt.Println("   On other platforms, standard file operations will be used.")
		fmt.Println()
		logger.Warning("Running on non-Windows platform (%s): Windows-specific optimizations disabled", runtime.GOOS)
	}

	// Step 1: Safety validation
	logger.Info("Validating target path safety...")
	isSafe, reason := safety.IsSafePath(config.TargetDir)
	if !isSafe {
		fmt.Fprintf(os.Stderr, "\n❌ Error: Cannot delete this path\n")
		fmt.Fprintf(os.Stderr, "   Reason: %s\n\n", reason)
		logger.Error("Path validation failed: %s", reason)
		return 2
	}

	// Step 2: Scan directory
	logger.Info("Scanning directory...")
	fmt.Println("\nScanning directory...")

	s := scanner.NewScanner(config.TargetDir, config.KeepDays)
	scanResult, err := s.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Error: Failed to scan directory: %v\n\n", err)
		logger.Error("Directory scan failed: %v", err)
		return 2
	}

	fmt.Printf("Found %d files and directories", scanResult.TotalScanned)
	if config.KeepDays != nil {
		fmt.Printf(" (%d to delete, %d to retain)", scanResult.TotalToDelete, scanResult.TotalRetained)
	}
	fmt.Println()

	logger.Info("Scan complete: %d total, %d to delete, %d to retain",
		scanResult.TotalScanned, scanResult.TotalToDelete, scanResult.TotalRetained)

	// Check if there's anything to delete
	if scanResult.TotalToDelete == 0 {
		fmt.Println("\n✓ No files to delete.")
		logger.Info("No files to delete, exiting")
		return 0
	}

	// Step 3: Get user confirmation
	confirmed := safety.GetUserConfirmation(config.TargetDir, scanResult.TotalToDelete, config.DryRun, config.Force)
	if !confirmed {
		fmt.Println("\n❌ Deletion cancelled by user.")
		logger.Info("Deletion cancelled by user")
		return 0
	}

	// Step 4: Initialize deletion engine
	logger.Info("Initializing deletion engine with %d workers", config.Workers)
	backend := backend.NewBackend()

	// Create progress reporter
	reporter := progress.NewReporter(scanResult.TotalToDelete, scanResult.TotalSizeBytes)

	// Create engine with progress callback
	eng := engine.NewEngine(backend, config.Workers, func(deletedCount int) {
		reporter.Update(deletedCount)
	})

	// Step 5: Set up interrupt handler for graceful cancellation
	ctx, cancel := engine.SetupInterruptHandler()
	defer cancel()

	// Step 6: Execute deletion
	fmt.Println()
	if config.DryRun {
		fmt.Println("Starting dry run (no files will be deleted)...")
	} else {
		fmt.Println("Starting deletion...")
	}

	result, err := eng.Delete(ctx, scanResult.Files, config.DryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Error: Deletion failed: %v\n\n", err)
		logger.Error("Deletion failed: %v", err)
		return 2
	}

	// Step 7: Display final statistics
	reporter.Finish(result.DeletedCount, result.FailedCount, scanResult.TotalRetained)

	// Log any errors that occurred
	if len(result.Errors) > 0 {
		logger.Warning("Deletion completed with %d errors", len(result.Errors))
		fmt.Printf("⚠️  Warning: %d files could not be deleted\n", result.FailedCount)
		if config.LogFile != "" {
			fmt.Printf("   See log file for details: %s\n", config.LogFile)
		}
		fmt.Println()
	}

	// Determine exit code
	if result.FailedCount > 0 {
		return 1 // Partial failure
	}

	if config.DryRun {
		fmt.Println("✓ Dry run completed successfully.")
	} else {
		fmt.Println("✓ Deletion completed successfully.")
	}

	return 0 // Success
}
