# Design Document: Fast File Deletion

## Overview

Fast File Deletion is a Go command-line tool optimized for Windows that addresses the performance bottleneck of deleting directories with millions of small files. The tool uses goroutines for true parallelism, direct Windows API calls, and optimized filesystem operations to achieve deletion speeds 5-10x faster than Windows Explorer.

Go was chosen for this project because:
- **True Concurrency**: Goroutines provide lightweight, efficient parallelism without GIL limitations
- **Native Performance**: Compiled binary runs 10-20x faster than interpreted Python
- **Single Binary**: Zero runtime dependencies, trivial distribution
- **Excellent Windows API Support**: `golang.org/x/sys/windows` provides direct syscall access
- **Fast Compilation**: Quick iteration during development
- **Memory Efficient**: Lower memory footprint than Python for processing millions of files

The architecture follows a modular design with clear separation between:
- Command-line interface and argument parsing
- Safety validation and confirmation logic
- File discovery and filtering
- Deletion engine with platform-specific optimizations
- Progress reporting and error handling

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI Entry Point                          │
│                  (Argument Parsing)                          │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  Safety Validator                            │
│         (Path validation, confirmation prompts)              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  File Scanner                                │
│         (Directory traversal, age filtering)                 │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  Deletion Engine                             │
│    ┌──────────────────┬──────────────────┐                  │
│    │  Windows Backend │  Generic Backend │                  │
│    │  (Win32 API)     │  (os/pathlib)    │                  │
│    └──────────────────┴──────────────────┘                  │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Progress Reporter                               │
│         (Real-time stats, ETA calculation)                   │
└─────────────────────────────────────────────────────────────┘
```

### Component Interaction Flow

1. **CLI Entry Point** parses arguments and validates input
2. **Safety Validator** checks for dangerous paths and gets user confirmation
3. **File Scanner** traverses directory tree and applies age filters
4. **Deletion Engine** deletes files using platform-optimized backend
5. **Progress Reporter** displays real-time statistics throughout the process

## Components and Interfaces

### 1. CLI Entry Point (`main.go`)

**Responsibility**: Parse command-line arguments and orchestrate the deletion workflow.

**Interface**:
```go
func main() {
    // Main entry point for the CLI application.
    // Exits with code 0 for success, non-zero for errors.
}

func parseArguments() *Config {
    // Parse and validate command-line arguments.
    // Returns parsed configuration struct.
}
```

**Arguments**:
- `target_dir` (positional): Path to directory to delete
- `--force`: Skip confirmation prompts
- `--dry-run`: Simulate deletion without actually deleting
- `--verbose`: Enable detailed logging
- `--log-file PATH`: Write logs to specified file
- `--keep-days N`: Only delete files older than N days
- `--workers N`: Number of parallel workers (default: auto-detect)

### 2. Safety Validator (`safety/validator.go`)

**Responsibility**: Validate paths and prevent accidental deletion of critical directories.

**Interface**:
```go
func IsSafePath(path string) (bool, string) {
    // Check if path is safe to delete.
    // Returns (isSafe, reason).
}

func GetUserConfirmation(path string, fileCount int, dryRun bool) bool {
    // Prompt user for deletion confirmation.
    // Returns true if user confirms, false otherwise.
}

var ProtectedPaths = []string{
    "C:\\Windows",
    "C:\\Program Files",
    "C:\\Program Files (x86)",
    // ... other system paths
}
```

**Validation Rules**:
- Reject paths in `ProtectedPaths` list
- Require additional confirmation for drive roots (C:\, D:\, etc.)
- Verify path exists before proceeding
- Check for write permissions on parent directory

### 3. File Scanner (`scanner/scanner.go`)

**Responsibility**: Traverse directory tree and collect files for deletion with optional age filtering.

**Interface**:
```go
type Scanner struct {
    rootPath  string
    keepDays  *int
}

func NewScanner(rootPath string, keepDays *int) *Scanner {
    // Initialize scanner with root path and optional age filter.
}

func (s *Scanner) Scan() (*ScanResult, error) {
    // Scan directory tree and return files to delete.
    // Returns ScanResult with file list and statistics.
}

type ScanResult struct {
    Files           []string
    TotalScanned    int
    TotalToDelete   int
    TotalRetained   int
    TotalSizeBytes  int64
}
```

**Scanning Strategy**:
- Use `filepath.WalkDir()` for efficient directory traversal
- Build list of files bottom-up (files before directories)
- Apply age filter during scanning to avoid loading unnecessary data
- Calculate total size for progress reporting

### 4. Deletion Engine (`engine/engine.go`)

**Responsibility**: Delete files using platform-optimized methods with goroutine-based parallelism.

**Interface**:
```go
type Engine struct {
    backend          Backend
    workers          int
    progressCallback func(int)
}

func NewEngine(backend Backend, workers int, progressCallback func(int)) *Engine {
    // Initialize engine with backend and worker count.
}

func (e *Engine) Delete(files []string, dryRun bool) (*DeletionResult, error) {
    // Delete files using parallel goroutines.
    // Returns DeletionResult with statistics and errors.
}

type DeletionResult struct {
    DeletedCount    int
    FailedCount     int
    Errors          []FileError
    DurationSeconds float64
}

type FileError struct {
    Path  string
    Error string
}
```

**Parallelization Strategy**:
- Use goroutines with worker pool pattern
- Default worker count: `runtime.NumCPU() * 2`
- Use buffered channels for work distribution
- Handle interruption (Ctrl+C) gracefully with context cancellation

### 5. Deletion Backends

**Responsibility**: Provide platform-specific deletion implementations.

**Interface**:
```go
type Backend interface {
    DeleteFile(path string) error
    DeleteDirectory(path string) error
}

type WindowsBackend struct {
    // Windows-optimized backend using syscalls
}

type GenericBackend struct {
    // Cross-platform backend using os package
}
```

**Windows Optimization**:
- Use `golang.org/x/sys/windows` for direct syscall access
- Call `windows.DeleteFile()` for file deletion
- Use extended-length path prefix (`\\?\`) to handle long paths
- Call `windows.RemoveDirectory()` for directory deletion
- Handle Windows-specific errors with proper error codes

**Generic Fallback**:
- Use `os.Remove()` for files
- Use `os.RemoveAll()` for directories (with caution)
- Standard Go error handling

### 6. Progress Reporter (`progress/reporter.go`)

**Responsibility**: Display real-time progress and statistics during deletion.

**Interface**:
```go
type Reporter struct {
    totalFiles  int
    totalBytes  int64
    startTime   time.Time
}

func NewReporter(totalFiles int, totalBytes int64) *Reporter {
    // Initialize reporter with total counts.
}

func (r *Reporter) Update(deletedCount int) {
    // Update progress with current deletion count.
}

func (r *Reporter) Finish(result *DeletionResult) {
    // Display final statistics.
}
```

**Display Format**:
```
Deleting: 1,234,567 / 5,000,000 files (24.7%)
Rate: 15,234 files/sec | Elapsed: 1m 21s | ETA: 4m 12s
```

**Statistics Tracked**:
- Files deleted / total files
- Current deletion rate (files/second)
- Elapsed time
- Estimated time remaining (ETA)
- Progress percentage

### 7. Error Logger (`logger/logger.go`)

**Responsibility**: Log errors and detailed operation information.

**Interface**:
```go
func SetupLogging(verbose bool, logFile string) error {
    // Configure logging based on CLI arguments.
}

func LogError(path string, err error) {
    // Log a deletion error with context.
}
```

**Logging Levels**:
- ERROR: Failed deletions, permission errors
- WARNING: Skipped files, locked files
- INFO: Major milestones (scan complete, deletion started)
- DEBUG: Individual file operations (verbose mode only)

## Data Models

### Path Representation

All paths are represented as strings with proper handling for Windows extended-length paths (`\\?\` prefix).

### Configuration

```go
type Config struct {
    TargetDir string
    Force     bool
    DryRun    bool
    Verbose   bool
    LogFile   string
    KeepDays  *int
    Workers   int
}
```

### Statistics

```go
type DeletionStats struct {
    TotalScanned    int
    TotalToDelete   int
    TotalRetained   int
    DeletedCount    int
    FailedCount     int
    DurationSeconds float64
    AverageRate     float64 // files per second
}
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*


### Property 1: Complete Directory Removal

*For any* valid directory structure, when deletion completes successfully, the target directory and all its contents should no longer exist on the filesystem.

**Validates: Requirements 1.1, 1.4**

### Property 2: Deletion Isolation

*For any* set of directories where one is the target for deletion, deleting the target directory should not affect files or directories outside the target path.

**Validates: Requirements 1.3**

### Property 3: Protected Path Rejection

*For any* path in the protected paths list (system-critical directories), the safety validator should reject the path and prevent deletion.

**Validates: Requirements 2.2**

### Property 4: Dry-Run Preservation

*For any* directory structure, running deletion in dry-run mode should leave all files and directories unchanged on the filesystem.

**Validates: Requirements 2.3, 6.4**

### Property 5: Confirmation Path Matching

*For any* target directory path, the confirmation validator should only accept confirmations that exactly match the originally specified path.

**Validates: Requirements 2.4**

### Property 5: Age-Based Filtering

*For any* directory containing files with various modification times, when a keep-days threshold is specified, only files older than the threshold should be deleted, and newer files should be preserved.

**Validates: Requirements 7.1, 7.3**

### Property 6: Modification Timestamp Usage

*For any* file, the age calculation should use the file's last modification timestamp (mtime) rather than creation or access time.

**Validates: Requirements 7.2**

### Property 8: Retention Count Accuracy

*For any* deletion operation with age-based filtering, the reported count of retained files should exactly match the number of files that were skipped due to being within the retention period.

**Validates: Requirements 7.6**

### Property 9: Error Resilience

*For any* set of files where some have restricted permissions or are locked, the deletion engine should continue processing remaining files and not halt on individual failures.

**Validates: Requirements 4.1, 4.2**

### Property 10: Error Tracking Accuracy

*For any* deletion operation, the reported counts of successfully deleted and failed files should exactly match the actual number of files deleted and failed during the operation.

**Validates: Requirements 4.4, 4.5**

### Property 11: Force Flag Behavior

*For any* deletion operation, when the force flag is enabled, no confirmation prompts should be displayed to the user.

**Validates: Requirements 6.3**

### Property 12: Verbose Logging

*For any* deletion operation, when verbose mode is enabled, the log output should contain more detailed information than when verbose mode is disabled.

**Validates: Requirements 6.5**

### Property 13: Log File Creation

*For any* deletion operation with a log-file path specified, a log file should be created at that path containing operation details and any errors encountered.

**Validates: Requirements 6.6**

### Property 14: Invalid Argument Handling

*For any* invalid command-line argument combination, the CLI should display an error message and usage information without attempting deletion.

**Validates: Requirements 6.7**

### Property 15: Platform Backend Selection

*For any* operating system, the deletion engine should automatically select the appropriate backend (Windows-optimized or generic) based on the detected platform.

**Validates: Requirements 8.4**

### Property 16: Non-Windows Warning

*For any* non-Windows platform, the tool should display a warning message indicating that performance optimizations are Windows-specific.

**Validates: Requirements 8.5**

### Property 17: Confirmation Prompt Display

*For any* deletion operation without the force flag, a confirmation prompt should be displayed before any files are deleted.

**Validates: Requirements 2.1**

### Property 18: Target Directory Argument Parsing

*For any* valid directory path provided as a command-line argument (including paths with spaces), the CLI should correctly parse and accept it as a single deletion target.

**Validates: Requirements 6.2, 6.3**

## Error Handling

### Error Categories

1. **Path Errors**
   - Non-existent paths: Return clear error message (Requirements 1.5)
   - Protected paths: Reject with warning message
   - Invalid paths: Return error with usage information

2. **Permission Errors**
   - Read permission denied: Log error, skip file, continue
   - Write permission denied: Log error, skip file, continue
   - Directory permission denied: Log error, skip directory tree, continue

3. **File Lock Errors**
   - File locked by process: Log warning, skip file, continue
   - Directory locked: Log warning, skip directory, continue

4. **Interruption**
   - Ctrl+C / SIGINT: Gracefully stop, report progress, exit cleanly
   - System shutdown: Attempt graceful stop if possible

5. **Filesystem Errors**
   - Disk full (during logging): Reduce logging verbosity, continue
   - I/O errors: Log error, skip affected file, continue
   - Network path errors: Log error, skip affected path, continue

### Error Recovery Strategy

- **Continue on Error**: Individual file failures should not stop the entire operation
- **Error Accumulation**: Collect all errors for final reporting
- **Graceful Degradation**: If logging fails, continue deletion but warn user
- **Clean Exit**: Always return appropriate exit code (0 = success, 1 = partial failure, 2 = complete failure)

### Error Logging Format

```
ERROR: Failed to delete file
  Path: C:\path\to\file.txt
  Reason: Permission denied (Access is denied)
  Timestamp: 2026-01-14 10:23:45
```

## Testing Strategy

### Dual Testing Approach

This project will use both unit tests and property-based tests to ensure comprehensive coverage:

- **Unit tests**: Verify specific examples, edge cases, and error conditions
- **Property tests**: Verify universal properties across all inputs

Both testing approaches are complementary and necessary. Unit tests catch concrete bugs and validate specific scenarios, while property tests verify general correctness across a wide range of inputs.

### Property-Based Testing

We will use **Rapid** (https://github.com/flyingmutant/rapid) as the property-based testing library for Go. Rapid integrates seamlessly with Go's standard testing package and will generate random test inputs to verify that our correctness properties hold across many scenarios.

**Configuration**:
- Each property test will run a minimum of 100 iterations
- Each test will be tagged with a comment referencing its design property
- Tag format: `// Feature: fast-file-deletion, Property N: [property text]`

**Example Property Test Structure**:
```go
func TestCompleteDirectoryRemoval(t *testing.T) {
    // Feature: fast-file-deletion, Property 1: Complete Directory Removal
    rapid.Check(t, func(t *rapid.T) {
        // Test implementation using rapid generators
    })
}
```

### Unit Testing

Unit tests will focus on:
- Specific examples that demonstrate correct behavior
- Edge cases (empty directories, single files, drive roots)
- Error conditions (permission denied, locked files, non-existent paths)
- Integration between components

**Test Organization**:
```
tests/
├── main_test.go             // CLI argument parsing
├── safety_test.go           // Path validation and confirmation
├── scanner_test.go          // Directory scanning and age filtering
├── engine_test.go           // Deletion engine
├── backend_test.go          // Platform-specific backends
├── progress_test.go         // Progress reporting
└── integration_test.go      // End-to-end integration tests
```

### Test Data Strategy

- **Temporary Directories**: Use `t.TempDir()` for isolated test environments
- **Time Mocking**: Use interfaces to inject time providers for testing age-based filtering
- **Platform Mocking**: Use build tags (`//go:build windows`) to test platform-specific code

### Coverage Goals

- Minimum 85% code coverage
- 100% coverage of safety validation logic
- 100% coverage of error handling paths
- All 18 correctness properties implemented as property tests

### Performance Testing

While not part of automated tests, manual performance benchmarks should be conducted:
- Test with directories containing 100K, 1M, and 10M files
- Compare deletion time against Windows Explorer and `rmdir /s`
- Verify 5-10x performance improvement on Windows
- Test resource usage (CPU, memory) under load
- Use Go's built-in benchmarking (`go test -bench`) for micro-benchmarks

## Implementation Notes

### Windows API Integration

Use `golang.org/x/sys/windows` for direct syscall access:
```go
import (
    "golang.org/x/sys/windows"
    "syscall"
)

// Use extended-length path prefix for long paths
func deleteFileWindows(path string) error {
    extendedPath := `\\?\` + path
    pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
    if err != nil {
        return err
    }
    
    return windows.DeleteFile(pathPtr)
}
```

### Goroutine-Based Parallelism

```go
func (e *Engine) Delete(files []string, dryRun bool) (*DeletionResult, error) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Create work channel
    workChan := make(chan string, 100)
    
    // Start worker goroutines
    var wg sync.WaitGroup
    for i := 0; i < e.workers; i++ {
        wg.Add(1)
        go e.worker(ctx, workChan, &wg)
    }
    
    // Send work
    go func() {
        for _, file := range files {
            select {
            case workChan <- file:
            case <-ctx.Done():
                return
            }
        }
        close(workChan)
    }()
    
    // Wait for completion
    wg.Wait()
    
    return result, nil
}
```

### Age Filtering Implementation

```go
func shouldDelete(filePath string, keepDays *int) (bool, error) {
    if keepDays == nil {
        return true, nil
    }
    
    if *keepDays == 0 {
        return true, nil
    }
    
    info, err := os.Stat(filePath)
    if err != nil {
        return false, err
    }
    
    fileAge := time.Since(info.ModTime())
    keepDuration := time.Duration(*keepDays) * 24 * time.Hour
    
    return fileAge > keepDuration, nil
}
```

### Progress Calculation

```go
type ProgressCalculator struct {
    totalFiles int
    startTime  time.Time
}

func (p *ProgressCalculator) CalculateETA(completed int) time.Duration {
    if completed == 0 {
        return time.Duration(math.MaxInt64)
    }
    
    elapsed := time.Since(p.startTime)
    rate := float64(completed) / elapsed.Seconds()
    remaining := p.totalFiles - completed
    
    return time.Duration(float64(remaining)/rate) * time.Second
}
```

### Logging Configuration

```go
import "log"

func SetupLogging(verbose bool, logFile string) error {
    if logFile != "" {
        f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err != nil {
            return err
        }
        log.SetOutput(f)
    }
    
    if verbose {
        log.SetFlags(log.LstdFlags | log.Lshortfile)
    } else {
        log.SetFlags(log.LstdFlags)
    }
    
    return nil
}
```

## Dependencies

- **Go**: 1.21+
- **Standard Library**: `os`, `path/filepath`, `flag`, `log`, `context`, `sync`, `time`
- **Windows API**: `golang.org/x/sys/windows` (Windows-specific syscalls)
- **Testing**: `testing` (standard library), `github.com/flyingmutant/rapid` (property-based testing)
- **Optional**: `github.com/fatih/color` (colored terminal output), `github.com/schollz/progressbar/v3` (progress bars)

## Future Enhancements

Potential features for future versions:
- Pattern-based filtering (glob patterns, regex)
- Size-based filtering (delete files larger/smaller than threshold)
- Content-based filtering (delete files matching content patterns)
- Undo functionality (move to recycle bin instead of permanent deletion)
- Network path optimization
- GUI wrapper for non-technical users
- Scheduling and automation support
