# Implementation Plan: Windows Performance Optimization

## Overview

This implementation plan breaks down the Windows performance optimization work into incremental coding tasks. The approach focuses on implementing optimizations one at a time, testing each thoroughly, and maintaining backward compatibility throughout. The implementation follows a layered approach: backend enhancements first, then scanner optimizations, then engine improvements, and finally CLI integration.

## Tasks

- [x] 1. Tag current baseline implementation
  - Initialize git repository if not already initialized
  - Commit all current changes with message "Baseline implementation before Windows optimization"
  - Create git tag v0.1.0 for the baseline
  - This preserves the working baseline (659-790 files/sec) for comparison
  - _Requirements: 2.1, 2.4_

- [x] 2. Set up advanced backend infrastructure
  - Create `AdvancedBackend` interface extending `Backend`
  - Define `DeletionMethod` enum and `DeletionStats` struct
  - Implement Windows version detection functions
  - Add dynamic API loading for NtDeleteFile
  - _Requirements: 1.1, 7.5, 9.1_

- [x] 3. Implement FileDispositionInfoEx deletion method
  - [x] 3.1 Implement `deleteWithFileInfo()` using SetFileInformationByHandle
    - Open file with DELETE access and FILE_FLAG_BACKUP_SEMANTICS
    - Try FileDispositionInfoEx with POSIX semantics and ignore readonly flags
    - Fall back to FileDispositionInfo if InfoEx unavailable
    - Handle errors and translate to readable messages
    - _Requirements: 1.1, 1.2, 1.4_
  
  - [x] 3.2 Write property test for FileDispositionInfoEx fallback chain
    - **Property 1: Fallback chain completeness**
    - **Validates: Requirements 1.1, 1.2, 1.3**
  
  - [x] 3.3 Write property test for read-only file handling
    - **Property 2: Read-only file handling**
    - **Validates: Requirements 1.4**

- [x] 4. Implement FILE_FLAG_DELETE_ON_CLOSE deletion method
  - [x] 4.1 Implement `deleteWithDeleteOnClose()` method
    - Open file with DELETE_ON_CLOSE flag
    - Close handle to trigger deletion
    - Handle errors appropriately
    - _Requirements: 1.5_
  
  - [x] 4.2 Write property test for DELETE_ON_CLOSE correctness
    - **Property 3: FILE_FLAG_DELETE_ON_CLOSE correctness**
    - **Validates: Requirements 1.5**

- [x] 5. Implement NtDeleteFile native API method
  - [x] 5.1 Implement `deleteWithNtAPI()` using NtDeleteFile
    - Load ntdll.dll dynamically
    - Convert path to UNICODE_STRING
    - Initialize OBJECT_ATTRIBUTES
    - Call NtDeleteFile and handle NT status codes
    - _Requirements: 1.6_
  
  - [x] 5.2 Implement NT status code translation
    - Create `translateNTStatus()` function
    - Map common NT status codes to readable messages
    - _Requirements: 8.4_
  
  - [x] 5.3 Write property test for NtDeleteFile correctness
    - **Property 4: NtDeleteFile correctness**
    - **Validates: Requirements 1.6, 8.4**

- [x] 6. Checkpoint - Ensure all deletion method tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 7. Implement deletion method selection and fallback
  - [x] 7.1 Implement `SetDeletionMethod()` and method routing
    - Add method selection logic to AdvancedBackend
    - Route DeleteFile() calls to appropriate method
    - Implement automatic fallback chain
    - _Requirements: 1.1, 1.2, 1.3, 8.2_
  
  - [x] 7.2 Implement read-only attribute clearing with retry
    - Create `clearReadOnlyAndRetry()` function
    - Detect access denied errors
    - Clear FILE_ATTRIBUTE_READONLY and retry
    - _Requirements: 1.4, 8.1_
  
  - [x] 7.3 Write property test for deletion method fallback
    - **Property 20: Deletion method fallback on failure**
    - **Validates: Requirements 8.2, 8.1**
  
  - [x] 7.4 Write property test for error resilience
    - **Property 21: Error resilience**
    - **Validates: Requirements 8.3, 8.5**

- [x] 8. Implement parallel directory scanning
  - [x] 8.1 Create `ParallelScanner` struct and constructor
    - Define ParallelScanner with worker count and configuration
    - Add `NewParallelScanner()` constructor
    - _Requirements: 3.1, 3.2_
  
  - [x] 8.2 Implement FindFirstFileEx-based scanning
    - Replace filepath.WalkDir with FindFirstFileEx on Windows
    - Implement parallel worker pool for subdirectory processing
    - Maintain bottom-up ordering (children before parents)
    - Add fallback to filepath.WalkDir if FindFirstFileEx fails
    - _Requirements: 3.1, 3.2, 3.3, 3.5_
  
  - [x] 8.3 Write property test for parallel subdirectory processing
    - **Property 8: Parallel subdirectory processing**
    - **Validates: Requirements 3.2**
  
  - [x] 8.4 Write property test for bottom-up ordering invariant
    - **Property 9: Bottom-up ordering invariant**
    - **Validates: Requirements 3.3**
  
  - [x] 8.5 Write property test for scan fallback correctness
    - **Property 11: Scan fallback correctness**
    - **Validates: Requirements 3.5**

- [x] 9. Implement UTF-16 pre-conversion
  - [x] 9.1 Add UTF-16 path storage to ScanResult
    - Add `FilesUTF16 []*uint16` field to ScanResult
    - Implement `convertToUTF16()` in scanner
    - Store UTF-16 paths during scan phase
    - _Requirements: 3.4, 5.1_
  
  - [x] 9.2 Modify backend to accept and reuse UTF-16 paths
    - Add `DeleteFileUTF16(*uint16)` method to Backend interface
    - Modify engine to pass UTF-16 paths when available
    - Ensure no re-conversion during deletion
    - _Requirements: 5.2, 5.3_
  
  - [x] 9.3 Write property test for UTF-16 pre-conversion
    - **Property 10: UTF-16 pre-conversion completeness**
    - **Validates: Requirements 3.4, 5.1**
  
  - [x] 9.4 Write property test for UTF-16 reuse without re-conversion
    - **Property 15: UTF-16 reuse without re-conversion**
    - **Validates: Requirements 5.2, 5.3**

- [x] 10. Checkpoint - Ensure scanning and UTF-16 tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Implement atomic counters in engine
  - [x] 11.1 Replace mutex-protected counters with atomic operations
    - Create `atomicCounters` struct using sync/atomic
    - Replace mutex locks with atomic.Add() and atomic.Load()
    - Update worker functions to use atomic counters
    - _Requirements: 4.1_
  
  - [x] 11.2 Add skip-double-call optimization
    - Add `IsDirectory` flag to PathInfo
    - Skip DeleteFile attempt for known directories
    - Call DeleteDirectory directly
    - _Requirements: 4.5_
  
  - [x] 11.3 Write property test for skip double-call optimization
    - **Property 14: Skip double-call optimization**
    - **Validates: Requirements 4.5**

- [x] 12. Implement dynamic buffer sizing and worker tuning
  - [x] 12.1 Implement dynamic buffer size calculation
    - Calculate buffer size as min(fileCount, 10000)
    - Apply buffer size when creating work channel
    - _Requirements: 4.3, 5.4_
  
  - [x] 12.2 Implement adaptive worker tuning
    - Create `adaptiveWorkerTuning()` goroutine
    - Monitor deletion rate every 5 seconds
    - Adjust worker count based on rate measurements
    - _Requirements: 4.4, 12.3_
  
  - [x] 12.3 Update default worker count to NumCPU * 4
    - Change default from NumCPU * 2 to NumCPU * 4
    - _Requirements: 4.2_
  
  - [x] 12.4 Write property test for buffer size calculation
    - **Property 12: Buffer size calculation**
    - **Validates: Requirements 4.3, 5.4**
  
  - [x] 12.5 Write property test for adaptive worker adjustment
    - **Property 13: Adaptive worker adjustment**
    - **Validates: Requirements 4.4**

- [x] 13. Implement batch memory release
  - [x] 13.1 Add batch processing with memory release
    - Process files in batches
    - Release memory from completed batches
    - Track memory usage during processing
    - _Requirements: 5.5_
  
  - [ ]* 13.2 Write property test for batch memory release
    - **Property 16: Batch memory release**
    - **Validates: Requirements 5.5**
    - **Note: Skipped as optional - test requires 100k+ files per iteration which is impractical for property-based testing**

- [x] 14. Checkpoint - Ensure engine optimization tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 15. Implement benchmarking mode
  - [x] 15.1 Create BenchmarkConfig and BenchmarkResult structs
    - Define configuration for benchmark runs
    - Define result structure with timing and metrics
    - _Requirements: 6.1, 6.2_
  
  - [x] 15.2 Implement `RunBenchmark()` function
    - Create test files for each benchmark run
    - Execute deletions with each method in isolation
    - Measure files/sec, total time, memory usage
    - Report results with percentage improvements
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 2.4, 2.5_
  
  - [x] 15.3 Write property test for benchmark isolation
    - **Property 17: Benchmark isolation**
    - **Validates: Requirements 6.5**

- [x] 16. Add CLI flags for configuration
  - [x] 16.1 Add --workers flag
    - Parse worker count from CLI
    - Pass to engine configuration
    - Validate positive integer
    - _Requirements: 11.1_
  
  - [x] 16.2 Add --buffer-size flag
    - Parse buffer size from CLI
    - Pass to engine configuration
    - Validate positive integer
    - _Requirements: 11.2_
  
  - [x] 16.3 Add --deletion-method flag
    - Parse method name (fileinfo, deleteonclose, ntapi, deleteapi)
    - Validate method is available on current Windows version
    - Pass to backend configuration
    - _Requirements: 11.3, 11.4_
  
  - [x] 16.4 Add --benchmark flag
    - Enable benchmarking mode
    - Run comparative benchmarks
    - Display results table
    - _Requirements: 6.1_
  
  - [x] 16.5 Implement configuration validation
    - Validate all flag combinations
    - Display error and exit with code 2 for invalid config
    - _Requirements: 11.5_
  
  - [x] 16.6 Write property tests for CLI flag handling
    - **Property 22: Worker count override**
    - **Property 23: Buffer size override**
    - **Property 24: Deletion method selection**
    - **Property 25: Invalid configuration handling**
    - **Validates: Requirements 11.1, 11.2, 11.3, 11.4, 11.5**

- [x] 17. Implement enhanced monitoring and logging
  - [x] 17.1 Add verbose logging for deletion method selection
    - Log which method is being used at startup
    - Log worker count and buffer size
    - _Requirements: 12.1, 12.2_
  
  - [x] 17.2 Implement periodic rate reporting
    - Report deletion rate every 5 seconds
    - Calculate and display current rate
    - _Requirements: 12.3_
  
  - [x] 17.3 Add detailed completion report
    - Report total files, time, average rate, peak rate
    - Include method statistics if using AdvancedBackend
    - _Requirements: 12.4_
  
  - [x] 17.4 Add benchmark timing breakdowns
    - Report scan time, queue time, delete time separately
    - Display phase-by-phase breakdown
    - _Requirements: 12.5_
  
  - [x] 17.5 Write property test for periodic rate reporting
    - **Property 26: Periodic rate reporting**
    - **Validates: Requirements 12.3**

- [x] 18. Implement backward compatibility and version detection
  - [x] 18.1 Implement Windows version detection
    - Use RtlGetVersion to detect Windows version
    - Implement `supportsFileDispositionInfoEx()` check
    - _Requirements: 7.5_
  
  - [x] 18.2 Add automatic fallback for older Windows versions
    - Select compatible methods based on version
    - Log warnings when advanced APIs unavailable
    - _Requirements: 7.1, 7.2_
  
  - [x] 18.3 Write property test for version-based fallback
    - **Property 18: Version-based fallback**
    - **Validates: Requirements 7.1, 7.5**
  
  - [x] 18.4 Write property test for CLI consistency
    - **Property 19: CLI consistency across versions**
    - **Validates: Requirements 7.3**

- [x] 19. Checkpoint - Ensure all integration tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 20. Add integration tests and performance validation
  - [x] 20.1 Write integration test for performance improvement
    - **Property 5: Optimized throughput improvement**
    - **Validates: Requirements 2.1**
  
  - [x] 20.2 Write integration test for parallel scan performance
    - **Property 6: Parallel scan performance**
    - **Validates: Requirements 2.2**
  
  - [x] 20.3 Write integration test for memory scaling
    - **Property 7: Sub-linear memory scaling**
    - **Validates: Requirements 2.3**

- [x] 21. Update documentation and build configuration
  - [x] 21.1 Update README with new CLI flags
    - Document --workers, --buffer-size, --deletion-method, --benchmark
    - Add performance optimization section
    - Include benchmark examples
    - _Requirements: 2.4_
  
  - [x] 21.2 Update Makefile with optimization build targets
    - Ensure Windows-specific code compiles correctly
    - Add benchmark target
    - _Requirements: 9.2, 9.3_
  
  - [x] 21.3 Add build tags to all Windows-specific files
    - Ensure `//go:build windows` tags are present
    - Verify generic fallbacks have `//go:build !windows`
    - _Requirements: 9.1_

- [x] 22. Final checkpoint - Run full test suite and benchmarks
  - Run all unit tests with race detection
  - Run all property-based tests
  - Run integration tests
  - Execute benchmarks and verify performance improvements
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- Build tags ensure Windows-specific code only compiles on Windows
- Integration tests measure actual performance improvements
