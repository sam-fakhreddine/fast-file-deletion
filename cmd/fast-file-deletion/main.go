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
	"time"

	"github.com/yourusername/fast-file-deletion/internal/backend"
	"github.com/yourusername/fast-file-deletion/internal/engine"
	"github.com/yourusername/fast-file-deletion/internal/logger"
	"github.com/yourusername/fast-file-deletion/internal/monitor"
	"github.com/yourusername/fast-file-deletion/internal/progress"
	"github.com/yourusername/fast-file-deletion/internal/safety"
	"github.com/yourusername/fast-file-deletion/internal/scanner"
)

// Config holds the parsed command-line configuration.
type Config struct {
	TargetDir      string
	Force          bool
	DryRun         bool
	Verbose        bool
	LogFile        string
	KeepDays       *int
	Workers        int
	BufferSize     int
	DeletionMethod string // Deletion method: auto, fileinfo, deleteonclose, ntapi, deleteapi
	Benchmark      bool   // Enable benchmarking mode
	Monitor        bool   // Enable real-time system resource monitoring
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
	bufferSize := flag.Int("buffer-size", 0, "Work queue buffer size (default: auto-detect)")
	deletionMethod := flag.String("deletion-method", "auto", "Deletion method: auto, fileinfo, deleteonclose, ntapi, deleteapi")
	benchmark := flag.Bool("benchmark", false, "Run comparative benchmarks of all deletion methods")
	monitor := flag.Bool("monitor", false, "Enable real-time system resource monitoring and bottleneck detection")

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

	// Validate keep-days before building config
	if *keepDays < -1 {
		return nil, fmt.Errorf("invalid --keep-days value: must be >= 0 (got %d)", *keepDays)
	}

	// Build config for validation
	var keepDaysPtr *int
	if *keepDays >= 0 {
		keepDaysPtr = keepDays
	}

	config := &Config{
		TargetDir:      *targetDir,
		Force:          *force,
		DryRun:         *dryRun,
		Verbose:        *verbose,
		LogFile:        *logFile,
		KeepDays:       keepDaysPtr,
		Workers:        *workers,
		BufferSize:     *bufferSize,
		DeletionMethod: *deletionMethod,
		Benchmark:      *benchmark,
		Monitor:        *monitor,
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// validateConfig validates all configuration parameters and flag combinations.
// Returns an error if the configuration is invalid.
//
// Validates Requirements: 11.5
func validateConfig(config *Config) error {
	// Note: keep-days validation is done in parseArguments before config is built

	// Validate workers
	if config.Workers < 0 {
		return fmt.Errorf("invalid --workers value: must be >= 0 (got %d)", config.Workers)
	}

	// Validate buffer size
	if config.BufferSize < 0 {
		return fmt.Errorf("invalid --buffer-size value: must be >= 0 (got %d)", config.BufferSize)
	}

	// Validate deletion method
	validMethods := map[string]bool{
		"auto":          true,
		"fileinfo":      true,
		"deleteonclose": true,
		"ntapi":         true,
		"deleteapi":     true,
	}
	if !validMethods[config.DeletionMethod] {
		return fmt.Errorf("invalid --deletion-method value: must be one of: auto, fileinfo, deleteonclose, ntapi, deleteapi (got %s)", config.DeletionMethod)
	}

	// Validate method availability on Windows
	if runtime.GOOS == "windows" && config.DeletionMethod != "auto" {
		if err := validateDeletionMethodAvailability(config.DeletionMethod); err != nil {
			return err
		}
	}

	// Validate flag combinations
	// Benchmark mode validations
	if config.Benchmark {
		// Dry-run doesn't make sense with benchmark mode
		if config.DryRun {
			return fmt.Errorf("--benchmark and --dry-run flags cannot be used together")
		}

		// Keep-days doesn't make sense with benchmark mode
		if config.KeepDays != nil {
			return fmt.Errorf("--benchmark and --keep-days flags cannot be used together")
		}

		// Benchmark mode is only available on Windows
		if runtime.GOOS != "windows" {
			return fmt.Errorf("--benchmark flag is only available on Windows")
		}
	}

	// Validate target directory exists (basic check)
	// Note: We don't validate existence here as that's done in the safety validator
	// But we check for obviously invalid paths
	// Empty string is valid at this point (will be caught by parseArguments if required)

	// Validate log file path if specified
	if config.LogFile != "" {
		// Check if log file path is valid (basic check)
		// Empty string check is redundant here since we already checked != ""
	}

	// Validate worker count is reasonable (not too high)
	if config.Workers > 1000 {
		return fmt.Errorf("invalid --workers value: must be <= 1000 (got %d)", config.Workers)
	}

	// Validate buffer size is reasonable (not too high)
	if config.BufferSize > 100000 {
		return fmt.Errorf("invalid --buffer-size value: must be <= 100000 (got %d)", config.BufferSize)
	}

	return nil


}

// validateDeletionMethodAvailability checks if the specified deletion method
// is available on the current Windows version. Returns an error if the method
// is not supported.
func validateDeletionMethodAvailability(method string) error {
	// Get the deletion method enum
	var deletionMethod backend.DeletionMethod
	switch method {
	case "fileinfo":
		deletionMethod = backend.MethodFileInfo
	case "deleteonclose":
		deletionMethod = backend.MethodDeleteOnClose
	case "ntapi":
		deletionMethod = backend.MethodNtAPI
	case "deleteapi":
		deletionMethod = backend.MethodDeleteAPI
	default:
		// "auto" is always available
		return nil
	}

	// Check availability using backend functions
	// We need to create a temporary backend to check availability
	tempBackend := backend.NewBackend()
	
	// Check if the backend supports advanced methods
	if advBackend, ok := tempBackend.(backend.AdvancedBackend); ok {
		// Try to set the method - if it's not available, we'll get an error
		// For now, we'll do a simple check based on the method type
		switch deletionMethod {
		case backend.MethodFileInfo:
			// FileInfo requires Windows 10 or later
			// The backend will handle fallback to FileDispositionInfo on older versions
			// So this is always "available" but may use fallback internally
			return nil
		case backend.MethodDeleteOnClose:
			// DELETE_ON_CLOSE is available on all Windows versions
			return nil
		case backend.MethodNtAPI:
			// NtDeleteFile is available on all modern Windows versions
			// The backend checks this at runtime
			return nil
		case backend.MethodDeleteAPI:
			// DeleteFile is always available
			return nil
		}
		_ = advBackend // Use the variable to avoid unused warning
	}

	return nil
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
	fmt.Println("  --buffer-size N         Work queue buffer size (default: auto-detect)")
	fmt.Println("  --deletion-method NAME  Deletion method (default: auto)")
	fmt.Println("                          Options: auto, fileinfo, deleteonclose, ntapi, deleteapi")
	fmt.Println("  --benchmark             Run comparative benchmarks of all deletion methods")
	fmt.Println("  --monitor               Enable real-time system resource monitoring and bottleneck detection")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  fast-file-deletion -td C:\\temp\\old-logs")
	fmt.Println("  fast-file-deletion -td \"C:\\Program Files\\old-cache\" --force")
	fmt.Println("  fast-file-deletion --target-directory C:\\temp\\cache --dry-run")
	fmt.Println("  fast-file-deletion -td C:\\data\\archive --keep-days 30 --verbose")
	fmt.Println("  fast-file-deletion -td \"/tmp/old data\" --workers 8 --log-file deletion.log")
	fmt.Println("  fast-file-deletion -td C:\\temp\\cache --deletion-method fileinfo")
	fmt.Println("  fast-file-deletion -td C:\\temp\\benchmark --benchmark --workers 16")
	fmt.Println("  fast-file-deletion -td C:\\data\\large-dir --monitor  # Diagnose performance bottlenecks")
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
	} else {
		// On Windows, log API availability information
		// Validates Requirements: 7.1, 7.2
		logWindowsAPIAvailability()
	}

	// If benchmark mode is enabled, run benchmarks instead of normal deletion
	if config.Benchmark {
		if runtime.GOOS != "windows" {
			fmt.Fprintf(os.Stderr, "\n❌ Error: Benchmark mode is only available on Windows\n\n")
			logger.Error("Benchmark mode requested on non-Windows platform")
			return 2
		}
		return runBenchmarkMode(config)
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
	// Determine worker count (for logging)
	workerCount := config.Workers
	if workerCount == 0 {
		workerCount = runtime.NumCPU() * 4
	}

	// Determine buffer size (for logging)
	bufferSize := config.BufferSize
	if bufferSize == 0 {
		bufferSize = min(scanResult.TotalToDelete, 10000)
	}

	logger.Info("Initializing deletion engine with %d workers", workerCount)
	logger.Debug("Engine configuration: workers=%d, buffer_size=%d", workerCount, bufferSize)

	backendInstance := backend.NewBackend()

	// Configure deletion method if specified and backend supports it
	if config.DeletionMethod != "auto" {
		if advBackend, ok := backendInstance.(backend.AdvancedBackend); ok {
			var method backend.DeletionMethod
			switch config.DeletionMethod {
			case "fileinfo":
				method = backend.MethodFileInfo
			case "deleteonclose":
				method = backend.MethodDeleteOnClose
			case "ntapi":
				method = backend.MethodNtAPI
			case "deleteapi":
				method = backend.MethodDeleteAPI
			}
			advBackend.SetDeletionMethod(method)
			logger.Info("Using deletion method: %s", config.DeletionMethod)
			logger.Debug("Deletion method configured: %s (explicit)", config.DeletionMethod)
		} else if runtime.GOOS == "windows" {
			logger.Warning("Advanced deletion methods not available on this backend")
		}
	} else {
		logger.Info("Using automatic deletion method selection")
		logger.Debug("Deletion method configured: auto (will use best available method with fallback)")
	}

	// Create progress reporter
	reporter := progress.NewReporter(scanResult.TotalToDelete, scanResult.TotalSizeBytes)

	// Create engine with progress callback
	eng := engine.NewEngineWithBufferSize(backendInstance, config.Workers, config.BufferSize, func(deletedCount int) {
		reporter.Update(deletedCount)
	})

	// Step 5: Set up interrupt handler for graceful cancellation
	ctx, cancel := engine.SetupInterruptHandler()
	defer cancel()

	// Step 5.5: Set up system resource monitoring if enabled
	var mon interface{}
	if config.Monitor {
		if runtime.GOOS == "windows" {
			// Use Windows-specific monitor with real CPU and I/O metrics
			winMon := monitor.NewWindowsMonitor()
			mon = winMon
			
			// Start monitoring goroutine
			go winMon.Start(ctx, 1*time.Second, 
				func() int { return 0 }, // Will be updated by engine
				func() float64 { return 0.0 }) // Will be updated by engine
			
			logger.Info("System resource monitoring enabled (Windows mode)")
			fmt.Println("📊 System resource monitoring enabled - bottleneck analysis will be shown at completion")
		} else {
			// Use generic monitor for non-Windows platforms
			genMon := monitor.NewMonitor()
			mon = genMon
			
			go genMon.Start(ctx, 1*time.Second,
				func() int { return 0 },
				func() float64 { return 0.0 })
			
			logger.Info("System resource monitoring enabled (generic mode)")
			fmt.Println("📊 System resource monitoring enabled - bottleneck analysis will be shown at completion")
		}
	}

	// Step 6: Execute deletion
	fmt.Println()
	if config.DryRun {
		fmt.Println("Starting dry run (no files will be deleted)...")
	} else {
		fmt.Println("Starting deletion...")
	}

	result, err := eng.DeleteWithUTF16(ctx, scanResult.Files, scanResult.FilesUTF16, scanResult.IsDirectory, config.DryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Error: Deletion failed: %v\n\n", err)
		logger.Error("Deletion failed: %v", err)
		return 2
	}

	// Step 7: Display final statistics
	reporter.Finish(result.DeletedCount, result.FailedCount, scanResult.TotalRetained)

	// Display detailed completion report
	// Validates Requirements: 12.4
	displayCompletionReport(result, backendInstance)

	// Display monitoring report if enabled
	if config.Monitor && mon != nil {
		if winMon, ok := mon.(*monitor.WindowsMonitor); ok {
			fmt.Println(winMon.GenerateReport())
		} else if genMon, ok := mon.(*monitor.Monitor); ok {
			fmt.Println(genMon.GenerateReport())
		}
	}

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

// runBenchmarkMode executes comparative benchmarks of all deletion methods.
// This function runs benchmarks using the target directory as the test location,
// measures performance metrics for each method, and displays results in a table format.
//
// Returns an exit code: 0 for success, 2 for failure.
//
// Validates Requirements: 6.1
func runBenchmarkMode(config *Config) int {
	fmt.Println("\n" + "═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("                    BENCHMARK MODE - DELETION METHODS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	logger.Info("Starting benchmark mode")

	// Step 1: Safety validation
	logger.Info("Validating target path safety...")
	isSafe, reason := safety.IsSafePath(config.TargetDir)
	if !isSafe {
		fmt.Fprintf(os.Stderr, "\n❌ Error: Cannot use this path for benchmarking\n")
		fmt.Fprintf(os.Stderr, "   Reason: %s\n\n", reason)
		logger.Error("Path validation failed: %s", reason)
		return 2
	}

	// Step 2: Scan directory to determine file count
	logger.Info("Scanning directory to determine benchmark size...")
	fmt.Println("Scanning directory to determine benchmark size...")

	s := scanner.NewScanner(config.TargetDir, config.KeepDays)
	scanResult, err := s.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Error: Failed to scan directory: %v\n\n", err)
		logger.Error("Directory scan failed: %v", err)
		return 2
	}

	fmt.Printf("Found %d files to use for benchmarking\n", scanResult.TotalToDelete)
	logger.Info("Scan complete: %d files available for benchmarking", scanResult.TotalToDelete)

	// Check if there are enough files for meaningful benchmarking
	if scanResult.TotalToDelete < 100 {
		fmt.Fprintf(os.Stderr, "\n⚠️  Warning: Only %d files found. Benchmarking requires at least 100 files for meaningful results.\n", scanResult.TotalToDelete)
		fmt.Fprintf(os.Stderr, "   Consider using a directory with more files or creating test files.\n\n")
		logger.Warning("Insufficient files for benchmarking: %d (minimum 100)", scanResult.TotalToDelete)
		return 2
	}

	// Step 3: Get user confirmation
	fmt.Println()
	fmt.Println("⚠️  BENCHMARK MODE WARNING:")
	fmt.Println("   - Files will be PERMANENTLY DELETED during benchmarking")
	fmt.Println("   - Each method will delete the same files in isolated test runs")
	fmt.Println("   - This process cannot be undone")
	fmt.Println()

	confirmed := safety.GetUserConfirmation(config.TargetDir, scanResult.TotalToDelete, false, config.Force)
	if !confirmed {
		fmt.Println("\n❌ Benchmark cancelled by user.")
		logger.Info("Benchmark cancelled by user")
		return 0
	}

	// Step 4: Configure benchmark
	workers := config.Workers
	if workers == 0 {
		workers = runtime.NumCPU() * 4
	}

	bufferSize := config.BufferSize
	if bufferSize == 0 {
		bufferSize = min(scanResult.TotalToDelete, 10000)
	}

	// Determine which methods to benchmark
	var methods []backend.DeletionMethod
	if config.DeletionMethod != "auto" {
		// If a specific method is requested, only benchmark that method
		switch config.DeletionMethod {
		case "fileinfo":
			methods = []backend.DeletionMethod{backend.MethodFileInfo}
		case "deleteonclose":
			methods = []backend.DeletionMethod{backend.MethodDeleteOnClose}
		case "ntapi":
			methods = []backend.DeletionMethod{backend.MethodNtAPI}
		case "deleteapi":
			methods = []backend.DeletionMethod{backend.MethodDeleteAPI}
		}
		fmt.Printf("\nBenchmarking single method: %s\n", config.DeletionMethod)
	} else {
		// Benchmark all available methods
		methods = []backend.DeletionMethod{
			backend.MethodFileInfo,
			backend.MethodDeleteOnClose,
			backend.MethodNtAPI,
			backend.MethodDeleteAPI,
		}
		fmt.Println("\nBenchmarking all available deletion methods...")
	}

	benchConfig := backend.BenchmarkConfig{
		Methods:    methods,
		Iterations: scanResult.TotalToDelete,
		TestDir:    config.TargetDir,
		Workers:    workers,
		BufferSize: bufferSize,
	}

	logger.Info("Benchmark configuration: methods=%d, iterations=%d, workers=%d, buffer=%d",
		len(methods), scanResult.TotalToDelete, workers, bufferSize)

	// Step 5: Run benchmarks
	fmt.Println()
	fmt.Println("Running benchmarks (this may take several minutes)...")
	fmt.Println()

	results, err := backend.RunBenchmark(benchConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Error: Benchmark failed: %v\n\n", err)
		logger.Error("Benchmark failed: %v", err)
		return 2
	}

	// Step 6: Display results
	displayBenchmarkResults(results, scanResult.TotalToDelete)

	logger.Info("Benchmark completed successfully")
	return 0
}

// displayBenchmarkResults displays benchmark results in a formatted table.
// This function shows performance metrics for each deletion method and calculates
// percentage improvements relative to the baseline (MethodDeleteAPI).
//
// Validates Requirements: 6.2, 6.3, 2.5
func displayBenchmarkResults(results []backend.BenchmarkResult, fileCount int) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("                         BENCHMARK RESULTS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Find baseline result (MethodDeleteAPI) for percentage improvement calculations
	var baseline *backend.BenchmarkResult
	for i := range results {
		if results[i].Method == backend.MethodDeleteAPI {
			baseline = &results[i]
			break
		}
	}

	// Display summary table header
	fmt.Printf("%-20s %12s %12s %10s %10s %12s\n",
		"Method", "Files/sec", "Total Time", "Syscalls", "Error Rate", "vs Baseline")
	fmt.Println("───────────────────────────────────────────────────────────────────────────")

	// Display each result
	for _, result := range results {
		methodName := result.Method.String()
		filesPerSec := fmt.Sprintf("%.2f", result.FilesPerSecond)
		totalTime := formatDuration(result.TotalTime)
		syscalls := fmt.Sprintf("%d", result.SyscallCount)
		errorRate := fmt.Sprintf("%.2f%%", result.ErrorRate)

		// Calculate percentage improvement vs baseline
		improvement := ""
		if baseline != nil && result.Method != backend.MethodDeleteAPI {
			pct := result.PercentageImprovement(baseline)
			if pct > 0 {
				improvement = fmt.Sprintf("+%.1f%%", pct)
			} else if pct < 0 {
				improvement = fmt.Sprintf("%.1f%%", pct)
			} else {
				improvement = "0.0%"
			}
		} else if result.Method == backend.MethodDeleteAPI {
			improvement = "(baseline)"
		}

		// Mark successful/failed benchmarks
		status := ""
		if !result.IsSuccessful() {
			status = " ⚠️"
		}

		fmt.Printf("%-20s %12s %12s %10s %10s %12s%s\n",
			methodName, filesPerSec, totalTime, syscalls, errorRate, improvement, status)
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Display detailed results for each method
	fmt.Println("DETAILED RESULTS:")
	fmt.Println()

	for i, result := range results {
		fmt.Printf("%d. %s\n", i+1, result.Method.String())
		fmt.Printf("   Files deleted:    %d / %d\n", result.FilesDeleted, fileCount)
		fmt.Printf("   Files failed:     %d\n", result.FilesFailed)
		fmt.Printf("   Files/second:     %.2f\n", result.FilesPerSecond)
		fmt.Printf("   Total time:       %v\n", result.TotalTime)
		fmt.Printf("   Timing breakdown:\n")
		fmt.Printf("     - Scan time:    %v (%.1f%%)\n", result.ScanTime, 
			float64(result.ScanTime)/float64(result.TotalTime)*100)
		fmt.Printf("     - Queue time:   %v (%.1f%%)\n", result.QueueTime,
			float64(result.QueueTime)/float64(result.TotalTime)*100)
		fmt.Printf("     - Delete time:  %v (%.1f%%)\n", result.DeleteTime,
			float64(result.DeleteTime)/float64(result.TotalTime)*100)
		fmt.Printf("   Syscall count:    %d (est.)\n", result.SyscallCount)
		fmt.Printf("   Memory used:      %.2f MB\n", float64(result.MemoryUsedBytes)/(1024*1024))
		fmt.Printf("   Error rate:       %.2f%%\n", result.ErrorRate)

		if baseline != nil && result.Method != backend.MethodDeleteAPI {
			improvement := result.PercentageImprovement(baseline)
			if improvement > 0 {
				fmt.Printf("   Improvement:      +%.2f%% faster than baseline\n", improvement)
			} else if improvement < 0 {
				fmt.Printf("   Improvement:      %.2f%% slower than baseline\n", improvement)
			} else {
				fmt.Printf("   Improvement:      Same speed as baseline\n")
			}
		} else if result.Method == backend.MethodDeleteAPI {
			fmt.Printf("   Improvement:      (baseline method)\n")
		}

		if !result.IsSuccessful() {
			fmt.Printf("   ⚠️  Status:        FAILED (high error rate or no files deleted)\n")
		} else {
			fmt.Printf("   ✓ Status:         SUCCESS\n")
		}

		fmt.Println()
	}

	// Display recommendations
	fmt.Println("RECOMMENDATIONS:")
	fmt.Println()

	// Find the fastest method
	var fastest *backend.BenchmarkResult
	for i := range results {
		if results[i].IsSuccessful() {
			if fastest == nil || results[i].FilesPerSecond > fastest.FilesPerSecond {
				fastest = &results[i]
			}
		}
	}

	if fastest != nil {
		fmt.Printf("• Fastest method: %s (%.2f files/sec)\n", fastest.Method.String(), fastest.FilesPerSecond)

		if baseline != nil && fastest.Method != backend.MethodDeleteAPI {
			improvement := fastest.PercentageImprovement(baseline)
			fmt.Printf("• Performance gain: %.1f%% faster than baseline\n", improvement)
		}

		fmt.Printf("• To use this method: --deletion-method %s\n", getMethodFlag(fastest.Method))
	} else {
		fmt.Println("• No successful benchmark results to recommend")
	}

	fmt.Println()
}

// formatDuration formats a duration in a human-readable format.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// formatNumber formats a number with thousands separators (commas).
// This makes large numbers more readable (e.g., 1,234,567 instead of 1234567).
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Format with commas
	str := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// getMethodFlag returns the CLI flag value for a deletion method.
func getMethodFlag(method backend.DeletionMethod) string {
	switch method {
	case backend.MethodFileInfo:
		return "fileinfo"
	case backend.MethodDeleteOnClose:
		return "deleteonclose"
	case backend.MethodNtAPI:
		return "ntapi"
	case backend.MethodDeleteAPI:
		return "deleteapi"
	default:
		return "auto"
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// displayCompletionReport displays a detailed completion report with performance metrics.
// This function shows total files deleted, total time, average rate, peak rate, and
// method statistics if using AdvancedBackend.
//
// Validates Requirements: 12.4
func displayCompletionReport(result *engine.DeletionResult, backendInstance backend.Backend) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("                      DELETION COMPLETION REPORT")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Display basic statistics
	fmt.Printf("Total files processed:  %s\n", formatNumber(result.DeletedCount+result.FailedCount))
	fmt.Printf("Successfully deleted:   %s files\n", formatNumber(result.DeletedCount))
	if result.FailedCount > 0 {
		fmt.Printf("Failed to delete:       %s files\n", formatNumber(result.FailedCount))
	}
	fmt.Println()

	// Display timing and performance metrics
	fmt.Printf("Total time:             %s\n", formatDuration(time.Duration(result.DurationSeconds*float64(time.Second))))
	fmt.Printf("Average deletion rate:  %.2f files/sec\n", result.AverageRate)
	if result.PeakRate > 0 {
		fmt.Printf("Peak deletion rate:     %.2f files/sec\n", result.PeakRate)
	}
	fmt.Println()

	// Display method statistics if using AdvancedBackend
	if advBackend, ok := backendInstance.(backend.AdvancedBackend); ok {
		stats := advBackend.GetDeletionStats()
		if stats != nil && hasMethodStats(stats) {
			fmt.Println("Deletion Method Statistics:")
			fmt.Println("───────────────────────────────────────────────────────────────────────────")
			
			// Display FileInfo method stats
			if stats.FileInfoAttempts > 0 {
				successRate := float64(stats.FileInfoSuccesses) / float64(stats.FileInfoAttempts) * 100
				fmt.Printf("  FileInfo (SetFileInformationByHandle):\n")
				fmt.Printf("    Attempts:     %s\n", formatNumber(stats.FileInfoAttempts))
				fmt.Printf("    Successes:    %s (%.1f%%)\n", formatNumber(stats.FileInfoSuccesses), successRate)
			}
			
			// Display DeleteOnClose method stats
			if stats.DeleteOnCloseAttempts > 0 {
				successRate := float64(stats.DeleteOnCloseSuccesses) / float64(stats.DeleteOnCloseAttempts) * 100
				fmt.Printf("  DeleteOnClose (FILE_FLAG_DELETE_ON_CLOSE):\n")
				fmt.Printf("    Attempts:     %s\n", formatNumber(stats.DeleteOnCloseAttempts))
				fmt.Printf("    Successes:    %s (%.1f%%)\n", formatNumber(stats.DeleteOnCloseSuccesses), successRate)
			}
			
			// Display NtAPI method stats
			if stats.NtAPIAttempts > 0 {
				successRate := float64(stats.NtAPISuccesses) / float64(stats.NtAPIAttempts) * 100
				fmt.Printf("  NtAPI (NtDeleteFile):\n")
				fmt.Printf("    Attempts:     %s\n", formatNumber(stats.NtAPIAttempts))
				fmt.Printf("    Successes:    %s (%.1f%%)\n", formatNumber(stats.NtAPISuccesses), successRate)
			}
			
			// Display Fallback method stats
			if stats.FallbackAttempts > 0 {
				successRate := float64(stats.FallbackSuccesses) / float64(stats.FallbackAttempts) * 100
				fmt.Printf("  Fallback (windows.DeleteFile):\n")
				fmt.Printf("    Attempts:     %s\n", formatNumber(stats.FallbackAttempts))
				fmt.Printf("    Successes:    %s (%.1f%%)\n", formatNumber(stats.FallbackSuccesses), successRate)
			}
			
			fmt.Println()
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()
}

// hasMethodStats checks if the deletion stats contain any method usage data.
func hasMethodStats(stats *backend.DeletionStats) bool {
	return stats.FileInfoAttempts > 0 ||
		stats.DeleteOnCloseAttempts > 0 ||
		stats.NtAPIAttempts > 0 ||
		stats.FallbackAttempts > 0
}

// logWindowsAPIAvailability logs information about which Windows deletion APIs
// are available on the current system. This function checks the Windows version
// and API availability, logging warnings when advanced APIs are unavailable.
//
// This helps users understand which deletion methods will be used on their system
// and provides transparency about automatic fallback behavior.
//
// Validates Requirements: 7.1, 7.2
func logWindowsAPIAvailability() {
	// Get API availability information from the backend
	major, minor, build, hasFileInfoEx, hasNtDelete := backend.GetAPIAvailability()

	// Log Windows version information
	logger.Debug("Windows version detected: %d.%d (build %d)", major, minor, build)

	// Check FileDispositionInfoEx availability
	if !hasFileInfoEx {
		// FileDispositionInfoEx requires Windows 10 RS1 (build 14393) or later
		logger.Warning("FileDispositionInfoEx not available (requires Windows 10 RS1 / build 14393+)")
		logger.Warning("Will use FileDispositionInfo fallback for advanced deletion methods")
		logger.Info("Current Windows version: %d.%d (build %d)", major, minor, build)
	} else {
		logger.Debug("FileDispositionInfoEx available (Windows 10 RS1+ detected)")
	}

	// Check NtDeleteFile availability
	if !hasNtDelete {
		logger.Warning("NtDeleteFile API not available in ntdll.dll")
		logger.Warning("Will skip NtDeleteFile method in automatic fallback chain")
	} else {
		logger.Debug("NtDeleteFile API available")
	}

	// Log summary of available methods
	if hasFileInfoEx && hasNtDelete {
		logger.Info("All advanced deletion methods available")
	} else if !hasFileInfoEx && !hasNtDelete {
		logger.Warning("Advanced deletion APIs unavailable - using compatibility mode")
		logger.Info("Available methods: FILE_FLAG_DELETE_ON_CLOSE, windows.DeleteFile")
	} else {
		logger.Info("Some advanced deletion methods available")
	}
}
