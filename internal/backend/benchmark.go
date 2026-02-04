//go:build windows

package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// BenchmarkConfig specifies the configuration for benchmark runs.
// It defines which deletion methods to test and how many files to use for each test.
//
// Benchmarking allows comparison of different deletion methods to identify
// the fastest approach for specific scenarios (file size, count, hardware, etc.).
//
// Validates Requirements: 6.1
type BenchmarkConfig struct {
	// Methods specifies which deletion methods to benchmark.
	// If empty, all available methods will be tested.
	// Example: []DeletionMethod{MethodFileInfo, MethodDeleteOnClose, MethodNtAPI, MethodDeleteAPI}
	Methods []DeletionMethod

	// Iterations specifies how many files to delete per method test.
	// Higher values provide more accurate measurements but take longer.
	// Recommended: 1000-10000 for quick tests, 100000+ for thorough benchmarks.
	Iterations int

	// TestDir specifies the directory containing test files for benchmarking.
	// The directory should contain enough files for all iterations.
	// Files will be created if the directory doesn't have enough files.
	TestDir string

	// Workers specifies the number of concurrent workers to use during deletion.
	// If 0, defaults to NumCPU * 4 (same as production default).
	// This allows testing how different deletion methods scale with concurrency.
	Workers int

	// BufferSize specifies the work queue buffer size.
	// If 0, defaults to min(Iterations, 10000).
	// This allows testing how buffer size affects performance.
	BufferSize int
}

// BenchmarkResult contains the timing and performance metrics for a single
// deletion method benchmark run.
//
// Each result represents one complete test of a deletion method, including
// all timing information, throughput metrics, and resource usage.
//
// Validates Requirements: 6.2
type BenchmarkResult struct {
	// Method identifies which deletion method was used for this benchmark.
	Method DeletionMethod

	// FilesPerSecond is the throughput achieved (files deleted per second).
	// This is the primary performance metric for comparison.
	// Higher values indicate better performance.
	FilesPerSecond float64

	// TotalTime is the total duration of the deletion operation.
	// This includes all overhead (worker startup, queue management, etc.).
	TotalTime time.Duration

	// ScanTime is the time spent scanning and preparing files for deletion.
	// This is measured separately to isolate deletion performance.
	// Only populated if scan timing is tracked separately.
	ScanTime time.Duration

	// QueueTime is the time spent queuing files to workers.
	// This measures the overhead of distributing work to the worker pool.
	// Lower queue times indicate more efficient work distribution.
	QueueTime time.Duration

	// DeleteTime is the time spent actually deleting files.
	// This excludes scan time and other overhead.
	// This is the most accurate measure of deletion method performance.
	DeleteTime time.Duration

	// SyscallCount estimates the number of system calls made during deletion.
	// Different methods have different syscall counts:
	//   - FileDispositionInfoEx: ~2 syscalls per file (CreateFile + SetFileInformationByHandle)
	//   - FILE_FLAG_DELETE_ON_CLOSE: ~2 syscalls per file (CreateFile + CloseHandle)
	//   - NtDeleteFile: ~1 syscall per file (direct kernel call)
	//   - DeleteFile: ~1 syscall per file (Win32 API)
	// Lower syscall counts generally correlate with better performance.
	SyscallCount int

	// MemoryUsedBytes is the peak memory usage during the benchmark run.
	// This helps identify memory-efficient deletion methods.
	// Measured in bytes.
	MemoryUsedBytes int64

	// FilesDeleted is the actual number of files successfully deleted.
	// Should equal Iterations for a successful benchmark.
	FilesDeleted int

	// FilesFailed is the number of files that failed to delete.
	// Should be 0 for a successful benchmark.
	FilesFailed int

	// ErrorRate is the percentage of files that failed to delete.
	// Calculated as (FilesFailed / (FilesDeleted + FilesFailed)) * 100.
	// Should be 0.0 for a successful benchmark.
	ErrorRate float64

	// Stats contains detailed statistics about which deletion methods were
	// actually used during the benchmark. This is useful when benchmarking
	// MethodAuto to see which methods were selected by the fallback chain.
	Stats *DeletionStats
}

// PercentageImprovement calculates the percentage improvement of this result
// compared to a baseline result.
//
// The improvement is calculated based on FilesPerSecond:
//   improvement = ((current - baseline) / baseline) * 100
//
// Positive values indicate this result is faster than the baseline.
// Negative values indicate this result is slower than the baseline.
//
// Example:
//   baseline: 700 files/sec
//   current:  1400 files/sec
//   improvement: ((1400 - 700) / 700) * 100 = 100% (2x faster)
//
// Validates Requirements: 2.5
func (r *BenchmarkResult) PercentageImprovement(baseline *BenchmarkResult) float64 {
	if baseline == nil || baseline.FilesPerSecond == 0 {
		return 0.0
	}

	return ((r.FilesPerSecond - baseline.FilesPerSecond) / baseline.FilesPerSecond) * 100.0
}

// IsSuccessful returns true if the benchmark completed successfully.
// A successful benchmark has:
//   - FilesDeleted > 0
//   - ErrorRate < 5% (allowing for some transient failures)
//   - TotalTime > 0 (indicating the benchmark actually ran)
func (r *BenchmarkResult) IsSuccessful() bool {
	return r.FilesDeleted > 0 && r.ErrorRate < 5.0 && r.TotalTime > 0
}

// RunBenchmark executes comparative benchmarks for different deletion methods.
// This function creates test files, executes deletions with each method in isolation,
// measures performance metrics, and reports results with percentage improvements.
//
// The benchmarking process:
//   1. For each deletion method in config.Methods:
//      a. Create fresh test files (config.Iterations count)
//      b. Configure backend for the specific method
//      c. Measure deletion performance (files/sec, time, memory)
//      d. Collect statistics and clean up
//   2. Calculate percentage improvements relative to baseline (MethodDeleteAPI)
//   3. Return results for all methods
//
// Isolation is ensured by:
//   - Creating fresh test files for each method
//   - Running each method in a separate iteration
//   - Measuring memory before and after each run
//   - Cleaning up between runs
//
// Parameters:
//   - config: Benchmark configuration specifying methods, iterations, test directory, etc.
//
// Returns:
//   - []BenchmarkResult: Results for each method tested
//   - error: If benchmark setup or execution fails
//
// Example usage:
//
//	config := BenchmarkConfig{
//	    Methods:    []DeletionMethod{MethodFileInfo, MethodDeleteOnClose, MethodNtAPI, MethodDeleteAPI},
//	    Iterations: 10000,
//	    TestDir:    "C:\\temp\\benchmark",
//	    Workers:    16,
//	    BufferSize: 10000,
//	}
//
//	results, err := RunBenchmark(config)
//	if err != nil {
//	    log.Fatalf("Benchmark failed: %v", err)
//	}
//
//	// Find baseline for comparison
//	var baseline *BenchmarkResult
//	for i := range results {
//	    if results[i].Method == MethodDeleteAPI {
//	        baseline = &results[i]
//	        break
//	    }
//	}
//
//	// Print results with percentage improvements and timing breakdowns
//	fmt.Println("Benchmark Results:")
//	fmt.Println("==================")
//	for _, result := range results {
//	    fmt.Printf("\nMethod: %s\n", result.Method.String())
//	    fmt.Printf("  Files/sec:     %.2f\n", result.FilesPerSecond)
//	    fmt.Printf("  Total time:    %v\n", result.TotalTime)
//	    fmt.Printf("  Scan time:     %v (%.1f%%)\n", result.ScanTime, 
//	        float64(result.ScanTime)/float64(result.TotalTime)*100)
//	    fmt.Printf("  Queue time:    %v (%.1f%%)\n", result.QueueTime,
//	        float64(result.QueueTime)/float64(result.TotalTime)*100)
//	    fmt.Printf("  Delete time:   %v (%.1f%%)\n", result.DeleteTime,
//	        float64(result.DeleteTime)/float64(result.TotalTime)*100)
//	    fmt.Printf("  Files deleted: %d\n", result.FilesDeleted)
//	    fmt.Printf("  Files failed:  %d\n", result.FilesFailed)
//	    fmt.Printf("  Error rate:    %.2f%%\n", result.ErrorRate)
//	    fmt.Printf("  Syscalls:      %d\n", result.SyscallCount)
//	    fmt.Printf("  Memory used:   %.2f MB\n", float64(result.MemoryUsedBytes)/(1024*1024))
//
//	    if baseline != nil && result.Method != MethodDeleteAPI {
//	        improvement := result.PercentageImprovement(baseline)
//	        fmt.Printf("  Improvement:   %.2f%% over baseline\n", improvement)
//	    }
//	}
//
// Validates Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 2.4, 2.5
func RunBenchmark(config BenchmarkConfig) ([]BenchmarkResult, error) {
	// Validate configuration
	if config.Iterations <= 0 {
		return nil, fmt.Errorf("iterations must be positive, got %d", config.Iterations)
	}
	if config.TestDir == "" {
		return nil, fmt.Errorf("test directory must be specified")
	}

	// Default to all methods if none specified
	methods := config.Methods
	if len(methods) == 0 {
		methods = []DeletionMethod{
			MethodFileInfo,
			MethodDeleteOnClose,
			MethodNtAPI,
			MethodDeleteAPI,
		}
	}

	// Default worker count if not specified
	workers := config.Workers
	if workers == 0 {
		workers = runtime.NumCPU() * 4
	}

	// Default buffer size if not specified
	bufferSize := config.BufferSize
	if bufferSize == 0 {
		bufferSize = min(config.Iterations, 10000)
	}

	results := make([]BenchmarkResult, 0, len(methods))

	// Run benchmark for each method
	for _, method := range methods {
		result, err := runSingleMethodBenchmark(method, config, workers, bufferSize)
		if err != nil {
			// Log error but continue with other methods
			// This allows partial benchmark results even if one method fails
			result = BenchmarkResult{
				Method:       method,
				FilesFailed:  config.Iterations,
				ErrorRate:    100.0,
				TotalTime:    0,
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// runSingleMethodBenchmark executes a benchmark for a single deletion method.
// This function handles the complete lifecycle:
//   1. Create test files
//   2. Measure memory before deletion
//   3. Execute deletion with timing
//   4. Measure memory after deletion
//   5. Calculate metrics and return results
//
// Parameters:
//   - method: The deletion method to benchmark
//   - config: Benchmark configuration
//   - workers: Number of concurrent workers
//   - bufferSize: Work queue buffer size
//
// Returns:
//   - BenchmarkResult: Performance metrics for this method
//   - error: If benchmark execution fails
func runSingleMethodBenchmark(method DeletionMethod, config BenchmarkConfig, workers int, bufferSize int) (BenchmarkResult, error) {
	// Create test directory for this method
	methodTestDir := filepath.Join(config.TestDir, fmt.Sprintf("bench_%s_%d", method.String(), time.Now().UnixNano()))
	err := os.MkdirAll(methodTestDir, 0755)
	if err != nil {
		return BenchmarkResult{}, fmt.Errorf("failed to create test directory: %w", err)
	}
	defer os.RemoveAll(methodTestDir) // Clean up after benchmark

	// Phase 1: Scan - Create test files
	scanStartTime := time.Now()
	testFiles, err := createTestFiles(methodTestDir, config.Iterations)
	if err != nil {
		return BenchmarkResult{}, fmt.Errorf("failed to create test files: %w", err)
	}
	scanTime := time.Since(scanStartTime)

	// Measure memory before deletion
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	// Create backend configured for this method
	backend := NewWindowsAdvancedBackend()
	backend.SetDeletionMethod(method)

	// Create work channel with configured buffer size
	workChan := make(chan string, bufferSize)

	// Track deletion statistics
	var deletedCount atomic.Int64
	var failedCount atomic.Int64

	// Start workers
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case path, ok := <-workChan:
					if !ok {
						return
					}

					// Delete the file
					err := backend.DeleteFile(path)
					if err != nil {
						failedCount.Add(1)
					} else {
						deletedCount.Add(1)
					}
				}
			}
		}()
	}

	// Phase 2: Queue - Send files to workers
	queueStartTime := time.Now()
	for _, file := range testFiles {
		workChan <- file
	}
	close(workChan)
	queueTime := time.Since(queueStartTime)

	// Phase 3: Delete - Wait for all workers to complete
	deleteStartTime := time.Now()
	wg.Wait()
	deleteTime := time.Since(deleteStartTime)

	// Calculate total time
	totalTime := scanTime + queueTime + deleteTime

	// Measure memory after deletion
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	// Calculate memory used (peak allocation during benchmark)
	memoryUsed := int64(memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc)

	// Get deletion statistics
	stats := backend.GetDeletionStats()

	// Calculate metrics
	deleted := int(deletedCount.Load())
	failed := int(failedCount.Load())
	filesPerSecond := float64(deleted) / totalTime.Seconds()
	errorRate := 0.0
	if deleted+failed > 0 {
		errorRate = (float64(failed) / float64(deleted+failed)) * 100.0
	}

	// Estimate syscall count based on method
	syscallCount := estimateSyscallCount(method, deleted)

	return BenchmarkResult{
		Method:          method,
		FilesPerSecond:  filesPerSecond,
		TotalTime:       totalTime,
		ScanTime:        scanTime,
		QueueTime:       queueTime,
		DeleteTime:      deleteTime,
		SyscallCount:    syscallCount,
		MemoryUsedBytes: memoryUsed,
		FilesDeleted:    deleted,
		FilesFailed:     failed,
		ErrorRate:       errorRate,
		Stats:           stats,
	}, nil
}

// createTestFiles creates test files for benchmarking.
// Each file is created with a small amount of data to simulate real files.
//
// Parameters:
//   - dir: Directory to create files in
//   - count: Number of files to create
//
// Returns:
//   - []string: Paths to created files
//   - error: If file creation fails
func createTestFiles(dir string, count int) ([]string, error) {
	files := make([]string, 0, count)

	// Create files in batches to avoid overwhelming the filesystem
	batchSize := 1000
	for i := 0; i < count; i++ {
		// Create subdirectories to avoid too many files in one directory
		subdir := filepath.Join(dir, fmt.Sprintf("batch_%d", i/batchSize))
		if i%batchSize == 0 {
			err := os.MkdirAll(subdir, 0755)
			if err != nil {
				return nil, fmt.Errorf("failed to create subdirectory: %w", err)
			}
		}

		// Create test file
		filename := filepath.Join(subdir, fmt.Sprintf("test_%d.txt", i))
		err := os.WriteFile(filename, []byte("test data for benchmarking"), 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create test file %s: %w", filename, err)
		}

		files = append(files, filename)
	}

	return files, nil
}

// estimateSyscallCount estimates the number of system calls for a deletion method.
// Different methods have different syscall counts:
//   - FileDispositionInfoEx: ~2 syscalls per file (CreateFile + SetFileInformationByHandle)
//   - FILE_FLAG_DELETE_ON_CLOSE: ~2 syscalls per file (CreateFile + CloseHandle)
//   - NtDeleteFile: ~1 syscall per file (direct kernel call)
//   - DeleteFile: ~1 syscall per file (Win32 API)
//
// Parameters:
//   - method: The deletion method used
//   - fileCount: Number of files deleted
//
// Returns:
//   - int: Estimated syscall count
func estimateSyscallCount(method DeletionMethod, fileCount int) int {
	switch method {
	case MethodFileInfo:
		// CreateFile + SetFileInformationByHandle + CloseHandle = 3 syscalls
		return fileCount * 3
	case MethodDeleteOnClose:
		// CreateFile + CloseHandle = 2 syscalls
		return fileCount * 2
	case MethodNtAPI:
		// NtDeleteFile = 1 syscall
		return fileCount * 1
	case MethodDeleteAPI:
		// DeleteFile = 1 syscall
		return fileCount * 1
	default:
		return fileCount * 2 // Default estimate
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
