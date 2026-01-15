# Requirements Document: Windows Performance Optimization

## Introduction

This specification defines performance optimization requirements for the Fast File Deletion (FFD) tool on Windows systems. The current implementation achieves 659-790 files/sec using basic Win32 APIs. This optimization phase targets 1500-2000+ files/sec through advanced Windows-specific techniques while maintaining backward compatibility and graceful degradation.

## Glossary

- **FFD**: Fast File Deletion tool
- **Win32_API**: Windows 32-bit Application Programming Interface
- **NT_API**: Windows Native API (lower-level than Win32)
- **UTF-16**: Unicode Transformation Format, 16-bit encoding used by Windows
- **Worker**: Goroutine that processes deletion tasks from a work queue
- **Atomic_Operation**: Lock-free concurrent operation using CPU-level primitives
- **POSIX_Semantics**: Unix-style file deletion behavior (delete-on-close)
- **Syscall**: System call to the operating system kernel
- **SSD**: Solid State Drive
- **HDD**: Hard Disk Drive
- **Deletion_Backend**: Platform-specific implementation of file/directory deletion
- **Scanner**: Component that traverses directory trees and identifies files for deletion
- **Work_Queue**: Buffered channel containing paths to delete
- **FileDispositionInfoEx**: Windows API structure for advanced file deletion
- **FindFirstFileEx**: Windows API for directory enumeration
- **NtDeleteFile**: Native NT API function for file deletion

## Requirements

### Requirement 1: Advanced Deletion Methods

**User Story:** As a Windows system administrator, I want FFD to use the fastest available deletion APIs, so that I can delete millions of files in minimal time.

#### Acceptance Criteria

1. WHEN FileDispositionInfoEx is available (Windows 10 RS1+), THE Deletion_Backend SHALL use SetFileInformationByHandle with FileDispositionInfoEx for file deletion
2. WHEN FileDispositionInfoEx fails or is unavailable, THE Deletion_Backend SHALL fall back to FileDispositionInfo
3. WHEN FileDispositionInfo fails or is unavailable, THE Deletion_Backend SHALL fall back to windows.DeleteFile
4. WHEN deleting a file with read-only attributes, THE Deletion_Backend SHALL automatically clear the read-only attribute and retry deletion
5. THE Deletion_Backend SHALL support FILE_FLAG_DELETE_ON_CLOSE as an alternative deletion method
6. THE Deletion_Backend SHALL support NtDeleteFile native API calls as an alternative deletion method

### Requirement 2: Performance Improvement

**User Story:** As a Windows system administrator, I want FFD to achieve measurable performance improvements over the baseline, so that I can delete files faster than the current implementation.

#### Acceptance Criteria

1. WHEN deleting files using optimized methods, THE FFD SHALL achieve higher files-per-second throughput than the baseline implementation (659-790 files/sec)
2. WHEN scanning directories, THE Scanner SHALL complete traversal faster than the baseline filepath.WalkDir implementation
3. WHEN measuring memory usage during deletion, THE FFD SHALL maintain reasonable memory consumption that scales sub-linearly with file count
4. THE FFD SHALL provide performance metrics that allow comparison between baseline and optimized implementations
5. WHEN benchmarking is enabled, THE FFD SHALL report percentage improvement over baseline for each optimization

### Requirement 3: Parallel Directory Scanning

**User Story:** As a Windows system administrator, I want FFD to scan directories in parallel, so that the scanning phase doesn't become a bottleneck for large directory trees.

#### Acceptance Criteria

1. THE Scanner SHALL use FindFirstFileEx instead of filepath.WalkDir for directory enumeration
2. WHEN scanning a directory tree, THE Scanner SHALL process subdirectories in parallel using multiple goroutines
3. WHEN scanning completes, THE Scanner SHALL provide paths in bottom-up order (files before their parent directories)
4. THE Scanner SHALL pre-convert all paths to UTF-16 during the scan phase
5. WHEN parallel scanning is unavailable, THE Scanner SHALL fall back to sequential filepath.WalkDir

### Requirement 4: Concurrency Optimization

**User Story:** As a Windows system administrator, I want FFD to maximize concurrent deletion operations, so that multi-core systems achieve optimal throughput.

#### Acceptance Criteria

1. THE FFD SHALL use atomic operations (sync/atomic) instead of mutex-protected counters for statistics tracking
2. WHEN determining worker count, THE FFD SHALL default to NumCPU * 4 workers
3. WHEN the Work_Queue buffer size is configured, THE FFD SHALL use min(fileCount, 10000) as the buffer size
4. THE FFD SHALL support adaptive worker tuning based on deletion rate measurements
5. WHEN a file is identified as a directory during deletion, THE Deletion_Backend SHALL skip attempting file deletion and directly call directory deletion

### Requirement 5: Memory Optimization

**User Story:** As a Windows system administrator, I want FFD to minimize memory allocations during deletion, so that it can handle directories with tens of millions of files without excessive memory usage.

#### Acceptance Criteria

1. WHEN scanning directories, THE Scanner SHALL convert paths to UTF-16 once and store the converted representation
2. WHEN deleting files, THE Deletion_Backend SHALL reuse pre-converted UTF-16 paths without re-conversion
3. THE FFD SHALL avoid allocating new UTF-16 buffers for each deletion operation
4. WHEN the Work_Queue is sized, THE FFD SHALL cap the buffer at 10000 entries to prevent unbounded memory growth
5. THE FFD SHALL release memory from completed deletion batches before processing subsequent batches

### Requirement 6: Benchmarking and Measurement

**User Story:** As a developer, I want to compare different deletion methods, so that I can identify the fastest approach for specific scenarios.

#### Acceptance Criteria

1. WHEN the --benchmark flag is provided, THE FFD SHALL execute deletions using all available methods (FileDispositionInfoEx, FILE_FLAG_DELETE_ON_CLOSE, NtDeleteFile, DeleteFile)
2. WHEN benchmarking completes, THE FFD SHALL report files/sec, total time, and syscall count for each method
3. THE FFD SHALL measure and report scan time separately from deletion time
4. THE FFD SHALL measure and report memory usage at scan completion and deletion completion
5. WHEN benchmarking is enabled, THE FFD SHALL perform deletions in isolated test runs to ensure fair comparison

### Requirement 7: Backward Compatibility

**User Story:** As a Windows user, I want FFD to work on older Windows versions, so that I can use the tool regardless of my OS version.

#### Acceptance Criteria

1. WHEN running on Windows versions older than Windows 10 RS1, THE FFD SHALL automatically fall back to compatible deletion methods
2. WHEN advanced APIs are unavailable, THE FFD SHALL log a warning message indicating fallback behavior
3. THE FFD SHALL maintain identical command-line interface behavior across all Windows versions
4. WHEN running on non-Windows platforms, THE FFD SHALL use the existing generic backend without Windows-specific optimizations
5. THE FFD SHALL detect Windows version at runtime and select appropriate API levels

### Requirement 8: Error Handling and Resilience

**User Story:** As a Windows system administrator, I want FFD to handle errors gracefully during optimized deletion, so that individual failures don't stop the entire operation.

#### Acceptance Criteria

1. WHEN SetFileInformationByHandle fails with access denied, THE Deletion_Backend SHALL attempt to clear read-only attributes and retry
2. WHEN a deletion method fails, THE Deletion_Backend SHALL try the next fallback method in the chain
3. WHEN all deletion methods fail for a file, THE FFD SHALL log the error and continue with remaining files
4. WHEN NtDeleteFile returns an error, THE Deletion_Backend SHALL translate NT status codes to human-readable messages
5. THE FFD SHALL track and report the count of files that failed deletion despite all retry attempts

### Requirement 9: Build Configuration

**User Story:** As a developer, I want Windows-specific optimizations to be conditionally compiled, so that the codebase remains maintainable across platforms.

#### Acceptance Criteria

1. THE FFD SHALL use Go build tags to separate Windows-specific code from generic implementations
2. WHEN building for Windows, THE FFD SHALL include all optimization code in the binary
3. WHEN building for non-Windows platforms, THE FFD SHALL exclude Windows-specific code from compilation
4. THE FFD SHALL provide a factory pattern that selects the appropriate backend based on the target platform
5. THE FFD SHALL maintain separate test files for Windows-specific and generic functionality

### Requirement 10: Testing and Validation

**User Story:** As a developer, I want comprehensive tests for performance optimizations, so that I can verify correctness and prevent regressions.

#### Acceptance Criteria

1. THE FFD SHALL include unit tests for each deletion method (FileDispositionInfoEx, FILE_FLAG_DELETE_ON_CLOSE, NtDeleteFile)
2. THE FFD SHALL include property-based tests that verify deletion correctness across all methods
3. THE FFD SHALL include integration tests that measure actual performance on test directories
4. THE FFD SHALL include tests for fallback behavior when advanced APIs are unavailable
5. THE FFD SHALL include tests for UTF-16 pre-conversion correctness

### Requirement 11: Configuration and Tuning

**User Story:** As a Windows system administrator, I want to tune performance parameters, so that I can optimize FFD for my specific hardware and workload.

#### Acceptance Criteria

1. WHEN the --workers flag is provided, THE FFD SHALL use the specified worker count instead of the default
2. WHEN the --buffer-size flag is provided, THE FFD SHALL use the specified Work_Queue buffer size
3. WHEN the --deletion-method flag is provided, THE FFD SHALL use only the specified method (fileinfo, deleteonclose, ntapi, deleteapi)
4. THE FFD SHALL validate that specified deletion methods are available on the current Windows version
5. WHEN invalid configuration is provided, THE FFD SHALL display an error message and exit with code 2

### Requirement 12: Monitoring and Observability

**User Story:** As a Windows system administrator, I want detailed performance metrics during deletion, so that I can understand bottlenecks and system behavior.

#### Acceptance Criteria

1. WHEN verbose logging is enabled, THE FFD SHALL report which deletion method is being used
2. WHEN verbose logging is enabled, THE FFD SHALL report worker count and buffer size at startup
3. WHEN deletion is in progress, THE FFD SHALL report current deletion rate every 5 seconds
4. WHEN deletion completes, THE FFD SHALL report total files deleted, total time, average rate, and peak rate
5. WHEN benchmarking is enabled, THE FFD SHALL report detailed timing breakdowns for scan, queue, and delete phases
