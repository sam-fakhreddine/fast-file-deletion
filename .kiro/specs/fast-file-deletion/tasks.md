# Implementation Plan: Fast File Deletion

## Overview

This implementation plan breaks down the Fast File Deletion tool into discrete coding tasks. The approach follows a bottom-up strategy: building core components first, then integrating them into the CLI application. Each task builds on previous work, with checkpoints to ensure quality and correctness.

The tool is implemented in Go for superior performance, true concurrency with goroutines, and single-binary distribution.

## Tasks

- [x] 1. Set up project structure and dependencies
  - Initialize Go module with `go mod init github.com/yourusername/fast-file-deletion`
  - Create directory structure: `cmd/`, `internal/`, `pkg/`
  - Set up `go.mod` with required dependencies
  - Add `golang.org/x/sys/windows` for Windows API access
  - Add `github.com/flyingmutant/rapid` for property-based testing
  - Create basic `main.go` entry point
  - Set up `.gitignore` for Go projects
  - _Requirements: 8.1, 9.5_

- [x] 2. Implement safety validation module
  - [x] 2.1 Create `internal/safety/validator.go` with path validation functions
    - Implement `IsSafePath()` function with protected paths list
    - Implement path existence and permission checking
    - Implement drive root detection logic
    - _Requirements: 2.2, 2.5_

  - [x] 2.2 Write property test for protected path rejection
    - **Property 3: Protected Path Rejection**
    - **Validates: Requirements 2.2**

  - [x] 2.3 Implement confirmation prompt logic
    - Create `GetUserConfirmation()` function
    - Implement path matching validation for confirmation
    - Add support for force mode (skip confirmation)
    - _Requirements: 2.1, 2.4, 6.3_

  - [x] 2.4 Write property test for confirmation path matching
    - **Property 5: Confirmation Path Matching**
    - **Validates: Requirements 2.4**

  - [x] 2.5 Write property test for force flag behavior
    - **Property 11: Force Flag Behavior**
    - **Validates: Requirements 6.3**

  - [x] 2.6 Write property test for confirmation prompt display
    - **Property 17: Confirmation Prompt Display**
    - **Validates: Requirements 2.1**

  - [x] 2.7 Write unit tests for safety validation edge cases
    - Test non-existent paths
    - Test drive root paths
    - Test relative vs absolute paths
    - _Requirements: 1.5, 2.2, 2.5_

- [x] 3. Implement file scanner module
  - [x] 3.1 Create `internal/scanner/scanner.go` with directory traversal
    - Implement `Scanner` struct with `Scan()` method
    - Use `filepath.WalkDir()` for efficient traversal
    - Build file list bottom-up (files before directories)
    - Calculate total size for progress reporting
    - _Requirements: 1.1, 3.1_

  - [x] 3.2 Add age-based filtering to scanner
    - Implement `shouldDelete()` function using ModTime
    - Add `keepDays` parameter to scanner
    - Track scanned vs to-delete counts separately
    - Handle `keepDays=0` edge case
    - _Requirements: 7.1, 7.2, 7.3, 7.5_

  - [x] 3.3 Write property test for age-based filtering
    - **Property 5: Age-Based Filtering**
    - **Validates: Requirements 7.1, 7.3**
    - **PBT Status: passed** (100 iterations)

  - [x] 3.4 Write property test for modification timestamp usage
    - **Property 6: Modification Timestamp Usage**
    - **Validates: Requirements 7.2**
    - **PBT Status: passed** (100 iterations)

  - [x] 3.5 Write property test for retention count accuracy
    - **Property 7: Retention Count Accuracy**
    - **Validates: Requirements 7.6**

  - [x] 3.6 Write unit tests for scanner edge cases
    - Test empty directories
    - Test single file
    - Test deeply nested structures
    - Test symlinks (if applicable)
    - _Requirements: 1.1, 7.1_

- [x] 4. Checkpoint - Ensure scanner and safety tests pass
  - Run `go test ./...` to ensure all tests pass
  - Ask the user if questions arise

- [x] 5. Implement deletion backends
  - [x] 5.1 Create `internal/backend/backend.go` with interface
    - Define `Backend` interface with `DeleteFile()` and `DeleteDirectory()` methods
    - _Requirements: 8.2, 8.3_

  - [x] 5.2 Implement Windows-optimized backend
    - Create `WindowsBackend` struct in `internal/backend/windows.go`
    - Use `golang.org/x/sys/windows` for syscalls
    - Implement `DeleteFile()` using `windows.DeleteFile()`
    - Use extended-length path prefix (`\\?\`) for long paths
    - Implement `DeleteDirectory()` using `windows.RemoveDirectory()`
    - Handle Windows-specific errors
    - _Requirements: 5.2, 8.2_

  - [x] 5.3 Implement generic cross-platform backend
    - Create `GenericBackend` struct in `internal/backend/generic.go`
    - Implement file deletion with `os.Remove()`
    - Implement directory deletion with `os.Remove()`
    - _Requirements: 8.3_

  - [x] 5.4 Implement platform detection and backend selection
    - Create factory function to select backend based on OS
    - Use `runtime.GOOS` to detect Windows
    - _Requirements: 8.4_

  - [x] 5.5 Write property test for platform backend selection
    - **Property 15: Platform Backend Selection**
    - **Validates: Requirements 8.4**

  - [x] 5.6 Write unit tests for both backends
    - Test file deletion on both backends
    - Test directory deletion on both backends
    - Test error handling (permissions, locked files)
    - Use build tags for Windows-specific tests
    - _Requirements: 4.1, 4.2, 8.2, 8.3_

- [x] 6. Implement deletion engine with goroutines
  - [x] 6.1 Create `internal/engine/engine.go` with Engine struct
    - Implement `NewEngine()` with backend and worker configuration
    - Auto-detect optimal worker count based on CPU cores (`runtime.NumCPU() * 2`)
    - Set up progress callback mechanism
    - _Requirements: 5.1, 5.5_

  - [x] 6.2 Implement parallel deletion logic with goroutines
    - Use worker pool pattern with goroutines
    - Create buffered channel for work distribution
    - Implement error collection and tracking with mutex
    - Track deletion statistics (deleted count, failed count, duration)
    - _Requirements: 5.1, 4.1, 4.2_

  - [x] 6.3 Add dry-run mode support
    - Add `dryRun` parameter to `Delete()` method
    - Skip actual deletion when dry-run is enabled
    - Still traverse and report what would be deleted
    - _Requirements: 2.3, 6.4_

  - [x] 6.4 Implement graceful interruption handling
    - Use `context.Context` for cancellation signaling
    - Handle `os.Interrupt` signal (Ctrl+C) gracefully
    - Report partial progress on interruption
    - _Requirements: 4.3_

  - [x] 6.5 Write property test for complete directory removal
    - **Property 1: Complete Directory Removal**
    - **Validates: Requirements 1.1, 1.4**

  - [x] 6.6 Write property test for deletion isolation
    - **Property 2: Deletion Isolation**
    - **Validates: Requirements 1.3**

  - [x] 6.7 Write property test for dry-run preservation
    - **Property 4: Dry-Run Preservation**
    - **Validates: Requirements 2.3, 6.4**

  - [x] 6.8 Write property test for error resilience
    - **Property 9: Error Resilience**
    - **Validates: Requirements 4.1, 4.2**

  - [x] 6.9 Write property test for error tracking accuracy
    - **Property 10: Error Tracking Accuracy**
    - **Validates: Requirements 4.4, 4.5**

  - [x] 6.10 Write unit tests for deletion engine
    - Test worker count auto-detection
    - Test interruption handling with context
    - Test empty directory handling
    - _Requirements: 4.3, 5.1, 5.5_

- [x] 7. Checkpoint - Ensure deletion engine tests pass
  - Run `go test ./...` to ensure all tests pass
  - Ask the user if questions arise

- [x] 8. Implement progress reporting
  - [x] 8.1 Create `internal/progress/reporter.go` with Reporter struct
    - Implement `NewReporter()` with total counts
    - Implement `Update()` method for progress updates
    - Calculate deletion rate (files/second)
    - Calculate ETA based on current rate
    - Format progress display with percentages
    - _Requirements: 3.2, 3.3, 3.4_

  - [x] 8.2 Implement final statistics reporting
    - Create `Finish()` method to display final stats
    - Show total time, average rate, success/failure counts
    - Display retention statistics when age filtering is used
    - _Requirements: 3.5, 7.6_

  - [x] 8.3 Write unit tests for progress calculations
    - Test rate calculation
    - Test ETA calculation
    - Test percentage calculation
    - Test edge cases (zero files, very fast deletion)
    - _Requirements: 3.2, 3.3, 3.4, 3.5_

- [x] 9. Implement error logging
  - [x] 9.1 Create `internal/logger/logger.go` with logging configuration
    - Implement `SetupLogging()` function
    - Configure log levels based on verbose flag
    - Set up file handler when log-file is specified
    - Format log messages with timestamps and context
    - _Requirements: 4.4, 6.5, 6.6_

  - [x] 9.2 Add error logging throughout codebase
    - Add error logging to deletion engine
    - Add error logging to backends
    - Add warning logging for skipped files
    - _Requirements: 4.1, 4.2, 4.4_

  - [x] 9.3 Write property test for verbose logging
    - **Property 12: Verbose Logging**
    - **Validates: Requirements 6.5**

  - [x] 9.4 Write property test for log file creation
    - **Property 13: Log File Creation**
    - **Validates: Requirements 6.6**

  - [x] 9.5 Write unit tests for logging configuration
    - Test verbose vs non-verbose output
    - Test log file creation
    - Test log message formatting
    - _Requirements: 4.4, 6.5, 6.6_

- [x] 10. Implement CLI interface
  - [x] 10.1 Create `cmd/fast-file-deletion/main.go` with argument parsing
    - Implement argument parsing using `flag` package
    - Define all CLI flags (target_dir, -force, -dry-run, -verbose, -log-file, -keep-days, -workers)
    - Add help text and usage examples
    - Validate argument combinations
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 7.1_

  - [x] 10.1.1 Fix argument parsing to handle paths with spaces
    - Update parseArguments() to correctly handle quoted paths with spaces
    - Ensure paths like "C:\Program Files\..." are treated as single arguments
    - Add validation to detect when path is incorrectly split
    - _Requirements: 6.3_

  - [x] 10.2 Implement main orchestration logic
    - Create main workflow to coordinate all components
    - Call safety validation before deletion
    - Initialize scanner with age filtering if specified
    - Initialize deletion engine with selected backend
    - Set up progress reporter
    - Handle errors and return appropriate exit codes
    - _Requirements: 1.1, 2.1, 2.2, 3.1, 4.5_

  - [x] 10.3 Add platform-specific warning for non-Windows
    - Detect non-Windows platforms using `runtime.GOOS`
    - Display warning about Windows-specific optimizations
    - _Requirements: 8.5_

  - [x] 10.4 Write property test for target directory argument parsing
    - **Property 18: Target Directory Argument Parsing**
    - **Validates: Requirements 6.2**

  - [x] 10.5 Write property test for invalid argument handling
    - **Property 14: Invalid Argument Handling**
    - **Validates: Requirements 6.7**

  - [x] 10.6 Write property test for non-Windows warning
    - **Property 16: Non-Windows Warning**
    - **Validates: Requirements 8.5**

  - [x] 10.7 Write unit tests for CLI
    - Test help display (no arguments)
    - Test argument parsing for all flags
    - Test invalid argument combinations
    - Test exit codes
    - _Requirements: 6.1, 6.7_

- [x] 11. Build and package the application
  - [x] 11.1 Create build scripts
    - Create `Makefile` or `build.sh` for cross-platform builds
    - Add build targets for Windows (amd64, arm64)
    - Add build targets for Linux and macOS (for testing)
    - Use `go build -ldflags="-s -w"` for smaller binaries
    - _Requirements: 9.2, 9.3_

  - [x] 11.2 Set up GitHub releases
    - Create `.github/workflows/release.yml` for automated releases
    - Configure goreleaser for multi-platform builds
    - Generate checksums for release artifacts
    - _Requirements: 9.1, 9.2_

  - [x] 11.3 Write integration tests
    - Test end-to-end deletion workflow
    - Test with various directory structures
    - Test with age filtering
    - Test dry-run mode
    - Test error scenarios
    - _Requirements: 1.1, 2.3, 7.1_

- [x] 12. Create documentation
  - [x] 12.1 Write README.md
    - Add project description and features
    - Add installation instructions (go install and binary download)
    - Add usage examples with all CLI flags
    - Add performance benchmarks and comparisons
    - Add safety warnings and best practices
    - Add troubleshooting section
    - _Requirements: 9.4_

  - [x] 12.2 Add code documentation
    - Add godoc comments to all exported functions and types
    - Add inline comments for complex logic
    - Document Windows API usage
    - _Requirements: 9.4_

- [x] 13. Final checkpoint - Run full test suite
  - Run `go test ./...` to ensure all tests pass
  - Run `go test -race ./...` to check for race conditions
  - Verify all 18 correctness properties are tested
  - Check code coverage with `go test -cover ./...`
  - Run manual smoke tests on Windows
  - Ask the user if questions arise

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- The implementation follows a bottom-up approach: core components first, then integration
- Windows-specific optimizations are isolated in the WindowsBackend for maintainability
- Go's goroutines provide true parallelism without GIL limitations
- Single binary distribution makes deployment trivial
