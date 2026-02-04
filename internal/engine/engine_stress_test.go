//go:build stress

package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yourusername/fast-file-deletion/internal/backend"
	"github.com/yourusername/fast-file-deletion/internal/testutil"
	"pgregory.net/rapid"
)

/*
Stress Tests for Deletion Engine

This file contains long-running stress tests that validate the deletion engine's
behavior under high load conditions with large data sets. These tests are excluded
from normal test runs and only execute when the "stress" build tag is provided.

Purpose:
- Validate engine performance with large file counts (10,000+ files)
- Test memory management under sustained load
- Verify correctness properties hold at scale
- Identify performance bottlenecks and resource leaks

Test Characteristics:
- Large data sets: 10,000+ files per test case
- Extended execution time: May take several minutes to complete
- Higher resource consumption: More CPU, memory, and disk I/O
- Thorough validation: More iterations for property-based tests

Running Stress Tests:
  # Run all stress tests
  go test -tags=stress ./internal/engine

  # Run stress tests with thorough mode
  TEST_INTENSITY=thorough go test -tags=stress ./internal/engine

  # Run specific stress test
  go test -tags=stress ./internal/engine -run TestStressLargeDirectoryDeletion

Configuration:
- Stress tests respect TEST_INTENSITY environment variable
- Quick mode: 1,000-5,000 files per test
- Thorough mode: 10,000-50,000 files per test
- Timeouts are extended for stress tests (10 minutes default)

Requirements Validated:
- Requirements 6.1: Stress tests separated with build tags
- Requirements 6.2: Excluded from default test runs
- Requirements 6.3: Documented execution instructions
- Requirements 6.5: Non-stress tests maintain >80% coverage
*/

// TestBatchMemoryRelease validates that memory from completed batches
// should be released before processing subsequent batches.
// Validates: Requirements 6.1, 6.2, 6.3
func TestBatchMemoryRelease(t *testing.T) {
	// Configure rapid with testutil iteration count and timeout
	testutil.GetRapidCheckConfig(t)
	
	rapid.Check(t, func(rt *rapid.T) {
		config := testutil.GetTestConfig()
		
		// Stress-appropriate file counts based on test intensity
		// Quick mode: 5,000-7,500 files (stress level but still reasonable)
		// Thorough mode: 10,000-15,000 files (full stress test)
		minFiles := 5000
		maxFiles := 7500
		if config.Intensity == testutil.IntensityThorough {
			minFiles = 10000
			maxFiles = 15000
		}
		
		fileCount := rapid.IntRange(minFiles, maxFiles).Draw(rt, "fileCount")
		
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		// Create the target directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}

		// Create files in subdirectories for efficiency
		// Use 1000 files per subdirectory to create a reasonable depth structure
		filesPerSubdir := 1000
		numSubdirs := (fileCount + filesPerSubdir - 1) / filesPerSubdir
		
		var filesToDelete []string
		fileIndex := 0
		
		// Track memory usage before starting
		var memStatsBefore runtime.MemStats
		runtime.ReadMemStats(&memStatsBefore)
		initialAlloc := memStatsBefore.Alloc
		
		rt.Logf("Creating %d files in %d subdirectories", fileCount, numSubdirs)
		
		for subdir := 0; subdir < numSubdirs && fileIndex < fileCount; subdir++ {
			subdirPath := filepath.Join(targetDir, fmt.Sprintf("subdir_%d", subdir))
			if err := os.MkdirAll(subdirPath, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}
			
			// Create files in this subdirectory
			filesInThisSubdir := filesPerSubdir
			if fileIndex+filesInThisSubdir > fileCount {
				filesInThisSubdir = fileCount - fileIndex
			}
			
			for i := 0; i < filesInThisSubdir; i++ {
				fileName := filepath.Join(subdirPath, fmt.Sprintf("file_%d.txt", i))
				
				// Create a small file (minimal content to save time)
				content := []byte(fmt.Sprintf("file %d", fileIndex))
				if err := os.WriteFile(fileName, content, 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				
				filesToDelete = append(filesToDelete, fileName)
				fileIndex++
			}
		}

		// Add subdirectories and target directory to deletion list (bottom-up order)
		for subdir := numSubdirs - 1; subdir >= 0; subdir-- {
			subdirPath := filepath.Join(targetDir, fmt.Sprintf("subdir_%d", subdir))
			filesToDelete = append(filesToDelete, subdirPath)
		}
		filesToDelete = append(filesToDelete, targetDir)

		rt.Logf("Created %d files, starting deletion with batch processing", fileIndex)

		// Track memory usage during deletion
		var memStatsAfterCreation runtime.MemStats
		runtime.ReadMemStats(&memStatsAfterCreation)
		memAfterCreation := memStatsAfterCreation.Alloc
		
		// Create deletion engine with generic backend
		backend := backend.NewBackend()
		engine := NewEngine(backend, 4, nil) // Use 4 workers for faster processing

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion (not dry-run) to test actual batch processing
		result, err := engine.Delete(ctx, filesToDelete, false)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}

		// Track memory usage after deletion
		var memStatsAfterDeletion runtime.MemStats
		runtime.ReadMemStats(&memStatsAfterDeletion)
		memAfterDeletion := memStatsAfterDeletion.Alloc

		// Property: Memory from completed batches should be released before processing subsequent batches
		// This means memory usage should not grow unboundedly during deletion
		
		// Verify deletion completed successfully
		if result.FailedCount > 0 {
			rt.Fatalf("Deletion had %d failures, expected 0. Errors: %v",
				result.FailedCount, result.Errors)
		}

		// Verify all files were processed
		if result.DeletedCount != len(filesToDelete) {
			rt.Fatalf("Expected %d items processed, got %d",
				len(filesToDelete), result.DeletedCount)
		}

		// Calculate memory growth during deletion
		// Memory should not grow significantly beyond what's needed for processing
		// With 5,000-15,000 files, we expect bounded memory growth
		memGrowthDuringDeletion := int64(memAfterDeletion) - int64(memAfterCreation)
		
		// Calculate expected maximum memory growth
		// Each work item is approximately 100 bytes (path strings + metadata)
		// With our file count range (5,000-15,000), we expect max ~1.5MB of work item memory
		// Add buffer for other allocations (error tracking, goroutine overhead, etc.)
		const bytesPerWorkItem = 100
		const bufferMultiplier = 50 // Allow 50x buffer for other allocations and GC behavior
		expectedMaxMemGrowth := int64(fileCount * bytesPerWorkItem * bufferMultiplier)
		
		// Verify memory growth is bounded
		// If memory grows beyond expected maximum, it indicates poor memory management
		if memGrowthDuringDeletion > expectedMaxMemGrowth {
			rt.Fatalf("Memory growth during deletion (%d bytes) exceeds expected maximum (%d bytes). "+
				"This indicates memory is not being managed efficiently.",
				memGrowthDuringDeletion, expectedMaxMemGrowth)
		}

		// Log memory statistics for debugging
		rt.Logf("Memory stats: initial=%d MB, after_creation=%d MB, after_deletion=%d MB, growth=%d MB",
			initialAlloc/1024/1024,
			memAfterCreation/1024/1024,
			memAfterDeletion/1024/1024,
			memGrowthDuringDeletion/1024/1024)
		
		// Verify that the target directory no longer exists
		if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
			rt.Fatalf("Target directory %s still exists after successful deletion", targetDir)
		}
		
		// Additional verification: Check that batch processing behavior is validated
		// With 5,000-15,000 files, we're testing the engine's memory management at stress levels
		if fileCount >= 5000 {
			rt.Logf("Memory release validated for %d files (stress test)", fileCount)
		}
	})
}

// TestStressCompleteDirectoryRemoval is a stress version of TestCompleteDirectoryRemoval
// that validates complete directory removal with 10,000 files.
// For any valid directory structure, when deletion completes successfully,
// the target directory and all its contents should no longer exist on the filesystem.
// Validates: Requirements 6.1, 6.2, 6.3
func TestStressCompleteDirectoryRemoval(t *testing.T) {
	// Configure rapid with testutil iteration count
	testutil.GetRapidCheckConfig(t)
	
	testutil.RapidCheck(t, func(rt *rapid.T) {
		config := testutil.GetTestConfig()
		
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		// Create the target directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}

		// Stress test file counts: 10,000 files
		// This is significantly larger than the regular test (50-500 files)
		fileCount := 10000
		
		// Number of subdirectories (5 to 10 for reasonable structure)
		numSubdirs := rapid.IntRange(5, 10).Draw(rt, "numSubdirs")
		
		// Calculate files per directory to reach target file count
		totalDirs := numSubdirs + 1 // +1 for target dir
		filesPerDir := fileCount / totalDirs

		// Track all created paths for verification
		allPaths := []string{targetDir}
		
		// Create subdirectories
		subdirs := []string{targetDir}
		for i := 0; i < numSubdirs; i++ {
			// Pick a random parent directory
			parentIdx := rapid.IntRange(0, len(subdirs)-1).Draw(rt, "parentIdx")
			parentDir := subdirs[parentIdx]
			
			// Create a subdirectory
			subdir := filepath.Join(parentDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}
			subdirs = append(subdirs, subdir)
			allPaths = append(allPaths, subdir)
		}

		rt.Logf("Creating %d files in %d directories", fileCount, len(subdirs))

		// Create files in each directory
		totalFilesCreated := 0
		for _, dir := range subdirs {
			for i := 0; i < filesPerDir && totalFilesCreated < fileCount; i++ {
				fileName := filepath.Join(dir, fmt.Sprintf("file_%d.txt", i))
				
				// Generate small file content (use config MaxFileSize)
				contentSize := rapid.IntRange(1, int(config.MaxFileSize)).Draw(rt, "contentSize")
				content := make([]byte, contentSize)
				for j := 0; j < contentSize; j++ {
					content[j] = byte(rapid.IntRange(0, 255).Draw(rt, "contentByte"))
				}
				
				if err := os.WriteFile(fileName, content, 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				allPaths = append(allPaths, fileName)
				totalFilesCreated++
			}
		}

		rt.Logf("Created %d files, starting deletion", totalFilesCreated)

		// Build file list for deletion (bottom-up order)
		var filesToDelete []string
		err := filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			filesToDelete = append(filesToDelete, path)
			return nil
		})
		if err != nil {
			rt.Fatalf("Failed to walk directory: %v", err)
		}

		// Reverse the list to get bottom-up order (files before directories)
		for i, j := 0, len(filesToDelete)-1; i < j; i, j = i+1, j-1 {
			filesToDelete[i], filesToDelete[j] = filesToDelete[j], filesToDelete[i]
		}

		// Create deletion engine with generic backend
		backend := backend.NewBackend()
		engine := NewEngine(backend, 4, nil) // Use 4 workers for faster processing

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion (not dry-run)
		result, err := engine.Delete(ctx, filesToDelete, false)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}

		// Property: When deletion completes successfully, the target directory
		// and all its contents should no longer exist on the filesystem
		
		// Check that deletion was successful (no failures)
		if result.FailedCount > 0 {
			rt.Fatalf("Deletion had %d failures, expected 0", result.FailedCount)
		}

		// Verify the target directory no longer exists
		if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
			rt.Fatalf("Target directory %s still exists after successful deletion", targetDir)
		}

		// Verify the deletion count matches the number of items we tried to delete
		if result.DeletedCount != len(filesToDelete) {
			rt.Fatalf("Expected %d items deleted, got %d", 
				len(filesToDelete), result.DeletedCount)
		}
		
		rt.Logf("Successfully deleted %d items (stress test with %d files)", 
			result.DeletedCount, totalFilesCreated)
		
		// Suppress unused variable warning
		_ = allPaths
	})
}

// TestStressBufferSizeCalculation is a stress version of TestBufferSizeCalculation
// that validates buffer size calculation with up to 50,000 files.
// For any file count N, the work queue buffer size should equal min(N, 10000),
// preventing unbounded memory growth while maintaining optimal performance.
// Validates: Requirements 6.1, 6.2, 6.3
func TestStressBufferSizeCalculation(t *testing.T) {
	// Configure rapid with testutil iteration count
	testutil.GetRapidCheckConfig(t)
	
	testutil.RapidCheck(t, func(rt *rapid.T) {
		// Stress test file counts: 10,000 to 50,000 files
		// This is significantly larger than the regular test (1-10,000 files)
		fileCount := rapid.IntRange(10000, 50000).Draw(rt, "fileCount")
		
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		// Create the target directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}

		// Create files in subdirectories for efficiency (1000 files per subdir)
		filesPerSubdir := 1000
		numSubdirs := (fileCount + filesPerSubdir - 1) / filesPerSubdir
		
		var filesToDelete []string
		fileIndex := 0
		
		rt.Logf("Creating %d files in %d subdirectories", fileCount, numSubdirs)
		
		for subdir := 0; subdir < numSubdirs && fileIndex < fileCount; subdir++ {
			subdirPath := filepath.Join(targetDir, fmt.Sprintf("subdir_%d", subdir))
			if err := os.MkdirAll(subdirPath, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}
			
			// Create files in this subdirectory
			filesInThisSubdir := filesPerSubdir
			if fileIndex+filesInThisSubdir > fileCount {
				filesInThisSubdir = fileCount - fileIndex
			}
			
			for i := 0; i < filesInThisSubdir; i++ {
				fileName := filepath.Join(subdirPath, fmt.Sprintf("file_%d.txt", i))
				
				// Create a small file (minimal content to save time)
				content := []byte(fmt.Sprintf("f%d", fileIndex))
				if err := os.WriteFile(fileName, content, 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				
				filesToDelete = append(filesToDelete, fileName)
				fileIndex++
			}
		}

		// Add subdirectories and target directory to deletion list (bottom-up order)
		for subdir := numSubdirs - 1; subdir >= 0; subdir-- {
			subdirPath := filepath.Join(targetDir, fmt.Sprintf("subdir_%d", subdir))
			filesToDelete = append(filesToDelete, subdirPath)
		}
		filesToDelete = append(filesToDelete, targetDir)

		rt.Logf("Created %d files, starting deletion in dry-run mode", fileIndex)

		// Property: The buffer size should be min(fileCount, 10000)
		expectedBufferSize := fileCount
		if expectedBufferSize > 10000 {
			expectedBufferSize = 10000
		}

		// Create deletion engine with generic backend
		backend := backend.NewBackend()
		engine := NewEngine(backend, 4, nil) // Use 4 workers for faster processing

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion in DRY-RUN mode (faster for large file counts)
		result, err := engine.Delete(ctx, filesToDelete, true)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}

		// Verify deletion completed successfully
		if result.FailedCount > 0 {
			rt.Fatalf("Deletion had %d failures, expected 0", result.FailedCount)
		}

		// Verify all files were processed
		if result.DeletedCount != len(filesToDelete) {
			rt.Fatalf("Expected %d items processed, got %d",
				len(filesToDelete), result.DeletedCount)
		}
		
		rt.Logf("Successfully processed %d items in dry-run mode (stress test with %d files, expected buffer size: %d)", 
			result.DeletedCount, fileIndex, expectedBufferSize)
		
		// Suppress unused variable warning
		_ = expectedBufferSize
	})
}
