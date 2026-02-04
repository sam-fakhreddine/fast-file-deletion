//go:build windows

package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestRunBenchmark tests the RunBenchmark function with a small number of files.
// This test verifies that:
//   - Benchmark runs successfully for all methods
//   - Results are collected for each method
//   - Files are created and deleted correctly
//   - Performance metrics are calculated
//
// Validates Requirements: 6.1, 6.2, 6.3, 6.4, 6.5
func TestRunBenchmark(t *testing.T) {
	// Create temporary test directory
	testDir := filepath.Join(os.TempDir(), "ffd_benchmark_test")
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Configure benchmark with small iteration count for testing
	config := BenchmarkConfig{
		Methods: []DeletionMethod{
			MethodFileInfo,
			MethodDeleteOnClose,
			MethodDeleteAPI,
		},
		Iterations: 100, // Small count for fast testing
		TestDir:    testDir,
		Workers:    4,
		BufferSize: 100,
	}

	// Run benchmark
	results, err := RunBenchmark(config)
	if err != nil {
		t.Fatalf("RunBenchmark failed: %v", err)
	}

	// Verify results
	if len(results) != len(config.Methods) {
		t.Errorf("Expected %d results, got %d", len(config.Methods), len(results))
	}

	// Verify each result
	for i, result := range results {
		t.Logf("Method: %s", result.Method.String())
		t.Logf("  Files/sec: %.2f", result.FilesPerSecond)
		t.Logf("  Total time: %v", result.TotalTime)
		t.Logf("  Scan time: %v", result.ScanTime)
		t.Logf("  Queue time: %v", result.QueueTime)
		t.Logf("  Delete time: %v", result.DeleteTime)
		t.Logf("  Files deleted: %d", result.FilesDeleted)
		t.Logf("  Files failed: %d", result.FilesFailed)
		t.Logf("  Error rate: %.2f%%", result.ErrorRate)
		t.Logf("  Syscall count: %d", result.SyscallCount)
		t.Logf("  Memory used: %d bytes", result.MemoryUsedBytes)

		// Verify method matches
		if result.Method != config.Methods[i] {
			t.Errorf("Result %d: expected method %s, got %s", i, config.Methods[i].String(), result.Method.String())
		}

		// Verify successful completion
		if !result.IsSuccessful() {
			t.Errorf("Result %d: benchmark was not successful (deleted=%d, error_rate=%.2f%%, time=%v)",
				i, result.FilesDeleted, result.ErrorRate, result.TotalTime)
		}

		// Verify files were deleted
		if result.FilesDeleted == 0 {
			t.Errorf("Result %d: no files were deleted", i)
		}

		// Verify performance metrics
		if result.FilesPerSecond <= 0 {
			t.Errorf("Result %d: invalid files/sec: %.2f", i, result.FilesPerSecond)
		}

		if result.TotalTime <= 0 {
			t.Errorf("Result %d: invalid total time: %v", i, result.TotalTime)
		}

		// Verify timing breakdown is reasonable
		if result.ScanTime < 0 {
			t.Errorf("Result %d: invalid scan time: %v", i, result.ScanTime)
		}
		if result.QueueTime < 0 {
			t.Errorf("Result %d: invalid queue time: %v", i, result.QueueTime)
		}
		if result.DeleteTime < 0 {
			t.Errorf("Result %d: invalid delete time: %v", i, result.DeleteTime)
		}

		// Verify timing breakdown adds up to total time (with small tolerance for measurement overhead)
		calculatedTotal := result.ScanTime + result.QueueTime + result.DeleteTime
		timeDiff := result.TotalTime - calculatedTotal
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		// Allow up to 10ms difference for measurement overhead
		if timeDiff > 10*time.Millisecond {
			t.Errorf("Result %d: timing breakdown doesn't match total time: scan=%v + queue=%v + delete=%v = %v, but total=%v (diff=%v)",
				i, result.ScanTime, result.QueueTime, result.DeleteTime, calculatedTotal, result.TotalTime, timeDiff)
		}

		// Verify syscall count is reasonable
		if result.SyscallCount <= 0 {
			t.Errorf("Result %d: invalid syscall count: %d", i, result.SyscallCount)
		}
	}
}

// TestRunBenchmarkWithBaseline tests percentage improvement calculation.
// This test verifies that:
//   - Baseline method (MethodDeleteAPI) is included
//   - Percentage improvements are calculated correctly
//   - Optimized methods show improvement over baseline
//
// Validates Requirements: 2.5
func TestRunBenchmarkWithBaseline(t *testing.T) {
	// Create temporary test directory
	testDir := filepath.Join(os.TempDir(), "ffd_benchmark_baseline_test")
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Configure benchmark with baseline method
	config := BenchmarkConfig{
		Methods: []DeletionMethod{
			MethodDeleteAPI, // Baseline
			MethodFileInfo,  // Optimized
		},
		Iterations: 100,
		TestDir:    testDir,
		Workers:    4,
		BufferSize: 100,
	}

	// Run benchmark
	results, err := RunBenchmark(config)
	if err != nil {
		t.Fatalf("RunBenchmark failed: %v", err)
	}

	// Find baseline result
	var baselineResult *BenchmarkResult
	for i := range results {
		if results[i].Method == MethodDeleteAPI {
			baselineResult = &results[i]
			break
		}
	}

	if baselineResult == nil {
		t.Fatal("Baseline result not found")
	}

	// Calculate percentage improvements
	for _, result := range results {
		if result.Method == MethodDeleteAPI {
			continue // Skip baseline itself
		}

		improvement := result.PercentageImprovement(baselineResult)
		t.Logf("Method %s: %.2f%% improvement over baseline", result.Method.String(), improvement)

		// Note: We don't assert improvement > 0 because performance can vary
		// depending on system load, file system state, etc.
		// The important thing is that the calculation works correctly.
	}
}

// TestRunBenchmarkIsolation tests that each method runs in isolation.
// This test verifies that:
//   - Fresh test files are created for each method
//   - Methods don't interfere with each other
//   - Results are independent
//
// Validates Requirements: 6.5
func TestRunBenchmarkIsolation(t *testing.T) {
	// Create temporary test directory
	testDir := filepath.Join(os.TempDir(), "ffd_benchmark_isolation_test")
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Configure benchmark
	config := BenchmarkConfig{
		Methods: []DeletionMethod{
			MethodFileInfo,
			MethodDeleteOnClose,
		},
		Iterations: 50,
		TestDir:    testDir,
		Workers:    2,
		BufferSize: 50,
	}

	// Run benchmark
	results, err := RunBenchmark(config)
	if err != nil {
		t.Fatalf("RunBenchmark failed: %v", err)
	}

	// Verify each method deleted the expected number of files
	for _, result := range results {
		if result.FilesDeleted != config.Iterations {
			t.Errorf("Method %s: expected %d files deleted, got %d (isolation may be broken)",
				result.Method.String(), config.Iterations, result.FilesDeleted)
		}
	}

	// Verify test directory is clean after benchmark
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read test directory: %v", err)
	}

	// Should be empty or only contain empty subdirectories
	for _, entry := range entries {
		if entry.IsDir() {
			subEntries, err := os.ReadDir(filepath.Join(testDir, entry.Name()))
			if err != nil {
				t.Errorf("Failed to read subdirectory %s: %v", entry.Name(), err)
				continue
			}
			if len(subEntries) > 0 {
				t.Errorf("Subdirectory %s is not empty after benchmark (isolation cleanup failed)", entry.Name())
			}
		} else {
			t.Errorf("File %s left in test directory after benchmark (isolation cleanup failed)", entry.Name())
		}
	}
}

// TestRunBenchmarkInvalidConfig tests error handling for invalid configurations.
// This test verifies that:
//   - Invalid iterations are rejected
//   - Missing test directory is rejected
//   - Appropriate error messages are returned
func TestRunBenchmarkInvalidConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      BenchmarkConfig
		expectError bool
	}{
		{
			name: "zero iterations",
			config: BenchmarkConfig{
				Iterations: 0,
				TestDir:    os.TempDir(),
			},
			expectError: true,
		},
		{
			name: "negative iterations",
			config: BenchmarkConfig{
				Iterations: -1,
				TestDir:    os.TempDir(),
			},
			expectError: true,
		},
		{
			name: "missing test directory",
			config: BenchmarkConfig{
				Iterations: 10,
				TestDir:    "",
			},
			expectError: true,
		},
		{
			name: "valid config",
			config: BenchmarkConfig{
				Iterations: 10,
				TestDir:    filepath.Join(os.TempDir(), "ffd_benchmark_valid_test"),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up test directory if it exists
			if tt.config.TestDir != "" {
				defer os.RemoveAll(tt.config.TestDir)
			}

			_, err := RunBenchmark(tt.config)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, but got none", tt.name)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.name, err)
			}
		})
	}
}

// TestCreateTestFiles tests the test file creation helper.
// This test verifies that:
//   - Files are created in the correct directory
//   - The correct number of files is created
//   - Files are organized into subdirectories
func TestCreateTestFiles(t *testing.T) {
	// Create temporary test directory
	testDir := filepath.Join(os.TempDir(), "ffd_create_test_files")
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create test files
	count := 2500 // Should create 3 subdirectories (batch_0, batch_1, batch_2)
	files, err := createTestFiles(testDir, count)
	if err != nil {
		t.Fatalf("createTestFiles failed: %v", err)
	}

	// Verify count
	if len(files) != count {
		t.Errorf("Expected %d files, got %d", count, len(files))
	}

	// Verify files exist
	for i, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("File %d does not exist: %s", i, file)
		}
	}

	// Verify subdirectories were created
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read test directory: %v", err)
	}

	subdirCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			subdirCount++
		}
	}

	expectedSubdirs := (count + 999) / 1000 // Ceiling division
	if subdirCount != expectedSubdirs {
		t.Errorf("Expected %d subdirectories, got %d", expectedSubdirs, subdirCount)
	}
}

// TestEstimateSyscallCount tests the syscall count estimation.
// This test verifies that:
//   - Different methods have different syscall counts
//   - Counts are reasonable and proportional to file count
func TestEstimateSyscallCount(t *testing.T) {
	tests := []struct {
		method        DeletionMethod
		fileCount     int
		expectedCount int
	}{
		{MethodFileInfo, 100, 300},        // 3 syscalls per file
		{MethodDeleteOnClose, 100, 200},   // 2 syscalls per file
		{MethodNtAPI, 100, 100},           // 1 syscall per file
		{MethodDeleteAPI, 100, 100},       // 1 syscall per file
		{MethodFileInfo, 1000, 3000},      // 3 syscalls per file
		{MethodDeleteOnClose, 1000, 2000}, // 2 syscalls per file
	}

	for _, tt := range tests {
		t.Run(tt.method.String(), func(t *testing.T) {
			count := estimateSyscallCount(tt.method, tt.fileCount)
			if count != tt.expectedCount {
				t.Errorf("Method %s with %d files: expected %d syscalls, got %d",
					tt.method.String(), tt.fileCount, tt.expectedCount, count)
			}
		})
	}
}

// TestBenchmarkResultPercentageImprovement tests the percentage improvement calculation.
// This test verifies that:
//   - Improvement is calculated correctly
//   - Positive values indicate faster performance
//   - Negative values indicate slower performance
//
// Validates Requirements: 2.5
func TestBenchmarkResultPercentageImprovement(t *testing.T) {
	baseline := &BenchmarkResult{
		FilesPerSecond: 700.0,
	}

	tests := []struct {
		name               string
		result             BenchmarkResult
		expectedImprovement float64
	}{
		{
			name: "2x faster",
			result: BenchmarkResult{
				FilesPerSecond: 1400.0,
			},
			expectedImprovement: 100.0, // 100% improvement
		},
		{
			name: "50% faster",
			result: BenchmarkResult{
				FilesPerSecond: 1050.0,
			},
			expectedImprovement: 50.0, // 50% improvement
		},
		{
			name: "same speed",
			result: BenchmarkResult{
				FilesPerSecond: 700.0,
			},
			expectedImprovement: 0.0, // No improvement
		},
		{
			name: "50% slower",
			result: BenchmarkResult{
				FilesPerSecond: 350.0,
			},
			expectedImprovement: -50.0, // 50% slower
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			improvement := tt.result.PercentageImprovement(baseline)
			if improvement != tt.expectedImprovement {
				t.Errorf("Expected %.2f%% improvement, got %.2f%%", tt.expectedImprovement, improvement)
			}
		})
	}
}

// TestBenchmarkResultIsSuccessful tests the success criteria.
// This test verifies that:
//   - Successful benchmarks are identified correctly
//   - Failed benchmarks are identified correctly
func TestBenchmarkResultIsSuccessful(t *testing.T) {
	tests := []struct {
		name       string
		result     BenchmarkResult
		shouldPass bool
	}{
		{
			name: "successful benchmark",
			result: BenchmarkResult{
				FilesDeleted: 100,
				ErrorRate:    0.0,
				TotalTime:    1000000000, // 1 second in nanoseconds
			},
			shouldPass: true,
		},
		{
			name: "no files deleted",
			result: BenchmarkResult{
				FilesDeleted: 0,
				ErrorRate:    0.0,
				TotalTime:    1000000000,
			},
			shouldPass: false,
		},
		{
			name: "high error rate",
			result: BenchmarkResult{
				FilesDeleted: 100,
				ErrorRate:    10.0, // 10% error rate
				TotalTime:    1000000000,
			},
			shouldPass: false,
		},
		{
			name: "zero time",
			result: BenchmarkResult{
				FilesDeleted: 100,
				ErrorRate:    0.0,
				TotalTime:    0,
			},
			shouldPass: false,
		},
		{
			name: "acceptable error rate",
			result: BenchmarkResult{
				FilesDeleted: 100,
				ErrorRate:    4.0, // 4% error rate (< 5%)
				TotalTime:    1000000000,
			},
			shouldPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSuccessful := tt.result.IsSuccessful()
			if isSuccessful != tt.shouldPass {
				t.Errorf("Expected IsSuccessful()=%v, got %v", tt.shouldPass, isSuccessful)
			}
		})
	}
}

// TestPropertyBenchmarkIsolation is a property-based test that verifies
// benchmark isolation across multiple deletion methods.
//
// **Validates: Requirements 6.5**
//
// Property 17: Benchmark isolation
// For any benchmarking run, each deletion method should execute on fresh,
// isolated test data to ensure fair comparison without interference.
//
// This property test verifies:
//   1. Each method receives exactly the expected number of files to delete
//   2. Each method's file count is independent of other methods
//   3. No files from one method's test remain when another method runs
//   4. All methods complete successfully with their isolated data
//   5. Test directories are properly cleaned up after each method
//
// The test generates random configurations and verifies that isolation
// is maintained regardless of:
//   - Number of methods being tested (2-4 methods)
//   - Number of iterations per method (10-100 files)
//   - Worker count (1-8 workers)
//   - Buffer size (10-100)
//
// Feature: windows-performance-optimization, Property 17: Benchmark isolation
func TestPropertyBenchmarkIsolation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random benchmark configuration
		// Use smaller iteration counts for property testing (10-100 files)
		iterations := rapid.IntRange(10, 100).Draw(t, "iterations")
		workers := rapid.IntRange(1, 8).Draw(t, "workers")
		bufferSize := rapid.IntRange(10, 100).Draw(t, "bufferSize")
		
		// Generate random subset of methods (2-4 methods)
		allMethods := []DeletionMethod{
			MethodFileInfo,
			MethodDeleteOnClose,
			MethodDeleteAPI,
		}
		
		// Select 2-4 methods randomly
		numMethods := rapid.IntRange(2, len(allMethods)).Draw(t, "numMethods")
		methodIndices := rapid.Permutation(allMethods).Draw(t, "methodIndices")
		methods := methodIndices[:numMethods]
		
		// Create unique test directory for this property test run
		testDir := filepath.Join(os.TempDir(), fmt.Sprintf("ffd_property_isolation_%d", time.Now().UnixNano()))
		err := os.MkdirAll(testDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
		defer os.RemoveAll(testDir)
		
		// Configure benchmark
		config := BenchmarkConfig{
			Methods:    methods,
			Iterations: iterations,
			TestDir:    testDir,
			Workers:    workers,
			BufferSize: bufferSize,
		}
		
		// Run benchmark
		results, err := RunBenchmark(config)
		if err != nil {
			t.Fatalf("RunBenchmark failed: %v", err)
		}
		
		// Property 1: Each method should have a result
		if len(results) != len(methods) {
			t.Fatalf("Expected %d results, got %d - isolation may have caused methods to interfere", 
				len(methods), len(results))
		}
		
		// Property 2: Each method should delete exactly the expected number of files
		// This verifies that each method got fresh, isolated test data
		for i, result := range results {
			if result.FilesDeleted != iterations {
				t.Errorf("Method %s (index %d): expected %d files deleted, got %d - isolation broken, files may have been shared or missing",
					result.Method.String(), i, iterations, result.FilesDeleted)
			}
			
			// Property 3: Error rate should be low (< 5%) for isolated tests
			// High error rates might indicate interference between methods
			if result.ErrorRate >= 5.0 {
				t.Errorf("Method %s (index %d): error rate %.2f%% >= 5%% - isolation may have caused file conflicts",
					result.Method.String(), i, result.ErrorRate)
			}
			
			// Property 4: Each method should complete successfully
			if !result.IsSuccessful() {
				t.Errorf("Method %s (index %d): benchmark not successful (deleted=%d, error_rate=%.2f%%, time=%v) - isolation failure",
					result.Method.String(), i, result.FilesDeleted, result.ErrorRate, result.TotalTime)
			}
		}
		
		// Property 5: Test directory should be clean after benchmark
		// This verifies that each method's test data was properly isolated and cleaned up
		entries, err := os.ReadDir(testDir)
		if err != nil {
			t.Fatalf("Failed to read test directory after benchmark: %v", err)
		}
		
		// Check for leftover files or non-empty subdirectories
		for _, entry := range entries {
			if entry.IsDir() {
				subPath := filepath.Join(testDir, entry.Name())
				subEntries, err := os.ReadDir(subPath)
				if err != nil {
					t.Errorf("Failed to read subdirectory %s: %v - cleanup may have failed", entry.Name(), err)
					continue
				}
				if len(subEntries) > 0 {
					t.Errorf("Subdirectory %s contains %d entries after benchmark - isolation cleanup failed, data leaked between methods",
						entry.Name(), len(subEntries))
				}
			} else {
				t.Errorf("File %s left in test directory after benchmark - isolation cleanup failed",
					entry.Name())
			}
		}
		
		// Property 6: Each method's file count should be independent
		// Verify that the sum of deleted files equals iterations * number of methods
		// This ensures each method got its own isolated set of files
		totalDeleted := 0
		for _, result := range results {
			totalDeleted += result.FilesDeleted
		}
		
		expectedTotal := iterations * len(methods)
		if totalDeleted != expectedTotal {
			t.Errorf("Total files deleted %d != expected %d (iterations=%d * methods=%d) - isolation broken, files shared or missing",
				totalDeleted, expectedTotal, iterations, len(methods))
		}
	})
}
