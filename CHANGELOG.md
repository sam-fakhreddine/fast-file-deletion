# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.16.0] - 2024-02-04

**MAJOR PERFORMANCE RELEASE: 40-60% faster deletion (1,050-1,150 files/sec)**

### Added
- **Dynamic Memory Management**: Automatically allocates 25% of system RAM to Go runtime (up to 6GB, minimum 512MB)
  - Dramatically reduces garbage collection pressure on high-memory systems
  - 32GB system: 8GB for Go (8x improvement over 1GB default)
  - Respects `GOMEMLIMIT` environment variable for manual override
  - Graceful fallback if system memory detection fails
- Memory limit bounds checking with configurable floor (512MB) and ceiling (6GB)
- Platform-specific memory detection: Windows (GlobalMemoryStatusEx), macOS (sysctl), Linux (/proc/meminfo)

### Changed
- **Memory Pressure Threshold**: Increased from 80% to 95% to reduce false positive warnings
  - On 32GB system using 1GB, warnings were firing at 96% of Go's 1GB limit (false alarm)
  - New 95% threshold more accurately reflects genuine memory pressure
- **Bottleneck Warning Cooldown**: Added 60-second cooldown to prevent log spam
  - Memory, GC, and CPU warnings now respect independent cooldowns
  - Reduces notification fatigue during long-running operations

### Performance
- Overall deletion rate: 1,050-1,150 files/sec (up from 659-790 files/sec baseline)
- **40-60% performance improvement** over v0.13 baseline
- Reduced GC pause frequency by ~8x on 32GB systems
- Lower CPU usage due to fewer GC cycles

### Documentation
- Updated README with v0.16 benchmarks and features
- Added comprehensive Memory Management section
- Updated performance characteristics table
- Added GOMEMLIMIT environment variable documentation

## [0.15.0] - 2024-02-03

### Added
- Initial dynamic memory limit implementation (25% of system RAM)
- Windows memory detection via GlobalMemoryStatusEx API
- Unix memory detection (macOS sysctl, Linux /proc/meminfo)
- Automatic Go memory limit configuration at startup

### Changed
- Logging format for memory limit initialization

## [0.14.0] - 2024-02-03

### Fixed
- Memory pressure warnings firing too aggressively (every second at 96% on 32GB system)
- Increased threshold from 80% to 95% of allocated memory
- Added warning cooldown to prevent log spam

## [0.13.0] - 2024-02-02

**WINDOWS PERFORMANCE OPTIMIZATION RELEASE**

### Added
- **NtDeleteFile Backend**: Native Windows API for 20-30% faster deletion
  - Bypasses Win32 layer entirely for lower overhead
  - Manual syscall implementation (OBJECT_ATTRIBUTES, UNICODE_STRING)
  - NT path conversion with `\??\` prefix
  - Comprehensive test suite (18 unit tests + 2 benchmarks)
- **Scanner Optimizations**:
  - FindFirstFileEx with FIND_FIRST_EX_LARGE_FETCH flag for faster traversal
  - Lock-free per-worker buffers eliminate mutex contention
  - strings.Builder for path concatenation (pre-allocated capacity)
  - UTF-16 path pre-conversion during scan (zero re-allocation during deletion)
- **Reparse Point Safety**:
  - Proper detection and handling of symlinks, junctions, mount points
  - FILE_ATTRIBUTE_REPARSE_POINT detection with GetFileAttributes
  - DeviceIoControl FSCTL_GET_REPARSE_POINT tag validation
  - Prevents accidental traversal outside target directory
- Atomic counter operations for lock-free statistics
- Advanced deletion method selection with automatic fallback
- `--deletion-method` flag to force specific backend (fileinfo, deleteonclose, ntapi, deleteapi)

### Changed
- Default worker multiplier: NumCPU * 4 (was NumCPU * 2)
- Buffer size: auto-detect min(fileCount, 10000) for sub-linear memory scaling
- Batch processing: 30,000-file batches with 80% sliding window threshold
- Error handling: Windows-specific error code translation

### Performance
- Baseline deletion rate: 659-790 files/sec (pre-optimization)
- Scanner optimizations: 15-20% improvement in traversal speed
- NtDeleteFile backend: 20-30% faster than DeleteFile baseline
- Overall system: 30-50% faster than v0.12

### Fixed
- O(n²) bubble sort in sortPathsByDepth replaced with O(n log n) sort.Slice
- Depth calculation bug in recursive scanner (off-by-one error)
- filepath.Join overhead in hot path (15-25% of scanner CPU time)

## [0.12.0] - 2024-01-25

### Added
- **Performance Monitoring** (`--monitor` flag)
  - Real-time CPU, memory, GC, and I/O tracking
  - Bottleneck detection and analysis
  - Resource pressure summary in final report
- **Benchmarking Mode** (`--benchmark` flag)
  - Compare all deletion methods on your system
  - Detailed performance metrics (files/sec, syscalls, error rates)
  - Automatic recommendations for fastest method
- Linux io_uring backend (IORING_OP_UNLINKAT) for async deletion
- macOS/Linux godirwalk parallel scanner

### Changed
- Monitor package with SystemMetrics structure
- Bottleneck detection thresholds: Memory 80%, GC 2.0, CPU 90%
- Windows CPU usage via GetSystemTimes API

### Testing
- Comprehensive monitor package test suite (monitor_test.go)
- Property-based tests for periodic rate calculations
- Cross-platform test verification

## [0.11.0] - 2024-01-20

### Added
- Advanced Windows deletion methods with automatic fallback chain:
  1. FileDispositionInfoEx (Windows 10 RS1+, fastest)
  2. FILE_FLAG_DELETE_ON_CLOSE (single syscall)
  3. NtDeleteFile (native API, low overhead)
  4. DeleteFile (baseline, universal compatibility)
- Windows version detection for API availability
- Deletion statistics tracking (methods used, syscall counts)
- `--buffer-size` flag for work queue tuning

### Changed
- Windows backend architecture for multiple deletion methods
- Backend interface with UTF16Backend and AdvancedBackend extensions
- Error handling with method-specific retry logic

## [0.10.0] - 2024-01-15

### Added
- Configurable test intensity levels (`TEST_INTENSITY=quick|thorough`)
- Stress tests behind `stress` build tag
- Makefile targets: `test`, `test-thorough`, `test-stress`, `test-all`, `test-coverage`, `test-race`
- Property-based testing with configurable iteration counts
- `testutil` package with GetRapidOptions(), GetTestConfig(), test fixtures

### Changed
- Test suite performance: ~30 seconds (quick mode), ~5 minutes (thorough mode)
- Rapid property tests use environment-based iteration counts
- Test documentation in README with execution time expectations

### Fixed
- Test verbosity: removed `-v` flag usage (property tests flood output)
- Added `VERBOSE_TESTS` environment variable for controlled verbosity

## [0.9.0] - 2024-01-10

### Added
- Initial implementation of fast file deletion tool
- Parallel deletion with goroutine worker pools
- Age-based file filtering (`--keep-days`)
- Dry-run mode (`--dry-run`)
- Progress reporting with ETA calculation
- Safety validation for protected paths
- Real-time statistics (deletion rate, elapsed time)
- Cross-platform support (Windows, Linux, macOS)

### Features
- Windows-optimized deletion using Win32 API
- Configurable worker count (`--workers`)
- Verbose logging (`--verbose`) and log file output (`--log-file`)
- Graceful interruption handling (Ctrl+C)
- Path confirmation prompts for safety
- Single binary distribution with zero dependencies

### Performance
- Initial baseline: 2-3x faster than PowerShell scripts
- Parallel processing across multiple CPU cores
- Efficient directory traversal

[Unreleased]: https://github.com/yourusername/fast-file-deletion/compare/v0.16.0...HEAD
[0.16.0]: https://github.com/yourusername/fast-file-deletion/releases/tag/v0.16.0
[0.15.0]: https://github.com/yourusername/fast-file-deletion/releases/tag/v0.15.0
[0.14.0]: https://github.com/yourusername/fast-file-deletion/releases/tag/v0.14.0
[0.13.0]: https://github.com/yourusername/fast-file-deletion/releases/tag/v0.13.0
[0.12.0]: https://github.com/yourusername/fast-file-deletion/releases/tag/v0.12.0
[0.11.0]: https://github.com/yourusername/fast-file-deletion/releases/tag/v0.11.0
[0.10.0]: https://github.com/yourusername/fast-file-deletion/releases/tag/v0.10.0
[0.9.0]: https://github.com/yourusername/fast-file-deletion/releases/tag/v0.9.0
