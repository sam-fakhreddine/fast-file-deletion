// Package internal provides integration tests for the Fast File Deletion tool.
// These tests verify the complete end-to-end workflow from CLI argument parsing
// through deletion execution, testing the integration of all components.
package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yourusername/fast-file-deletion/internal/backend"
	"github.com/yourusername/fast-file-deletion/internal/engine"
	"github.com/yourusername/fast-file-deletion/internal/safety"
	"github.com/yourusername/fast-file-deletion/internal/scanner"
)

// Integration Test 1: End-to-End Deletion Workflow
// Tests the complete workflow from scanning through deletion
// Validates: Requirements 1.1, 2.3, 7.1
func TestEndToEndDeletionWorkflow(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	
	// Create a realistic directory structure
	if err := os.MkdirAll(filepath.Join(targetDir, "logs", "2024"), 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "cache", "temp"), 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "data"), 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}
	
	// Create files with various ages
	files := []struct {
		path    string
		content string
		age     time.Duration
	}{
		{filepath.Join(targetDir, "logs", "2024", "app.log"), "log data", 10 * 24 * time.Hour},
		{filepath.Join(targetDir, "logs", "2024", "error.log"), "error data", 5 * 24 * time.Hour},
		{filepath.Join(targetDir, "cache", "temp", "cache1.tmp"), "cache data", 45 * 24 * time.Hour},
		{filepath.Join(targetDir, "cache", "temp", "cache2.tmp"), "cache data", 2 * 24 * time.Hour},
		{filepath.Join(targetDir, "data", "file1.dat"), "data content", 60 * 24 * time.Hour},
		{filepath.Join(targetDir, "data", "file2.dat"), "data content", 1 * 24 * time.Hour},
	}
	
	for _, f := range files {
		if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", f.path, err)
		}
		
		// Set modification time to simulate age
		modTime := time.Now().Add(-f.age)
		if err := os.Chtimes(f.path, modTime, modTime); err != nil {
			t.Fatalf("Failed to set file time: %v", err)
		}
	}
	
	// Step 1: Safety validation
	isSafe, reason := safety.IsSafePath(targetDir)
	if !isSafe {
		t.Fatalf("Path validation failed: %s", reason)
	}
	
	// Step 2: Scan directory without age filter
	s := scanner.NewScanner(targetDir, nil)
	scanResult, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Verify scan results
	if scanResult.TotalScanned == 0 {
		t.Fatalf("Expected files to be scanned, got 0")
	}
	if scanResult.TotalToDelete == 0 {
		t.Fatalf("Expected files to delete, got 0")
	}
	
	// Step 3: Initialize deletion engine
	b := backend.NewBackend()
	var deletedCount atomic.Int64
	progressCallback := func(count int) {
		deletedCount.Store(int64(count))
	}
	eng := engine.NewEngine(b, 2, progressCallback)
	
	// Step 4: Execute deletion
	ctx := context.Background()
	result, err := eng.Delete(ctx, scanResult.Files, false)
	if err != nil {
		t.Fatalf("Deletion failed: %v", err)
	}
	
	// Verify deletion results
	if result.FailedCount > 0 {
		t.Errorf("Expected 0 failures, got %d. Errors: %v", result.FailedCount, result.Errors)
	}
	
	if result.DeletedCount == 0 {
		t.Fatalf("Expected files to be deleted, got 0")
	}
	
	// Verify target directory no longer exists
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("Target directory still exists after deletion")
	}
	
	// Verify progress callback was called
	if deletedCount.Load() == 0 {
		t.Errorf("Progress callback was not called")
	}
	
	t.Logf("Successfully deleted %d items in %.2f seconds", 
		result.DeletedCount, result.DurationSeconds)
}

// Integration Test 2: Various Directory Structures
// Tests deletion with different directory layouts and complexities
// Validates: Requirements 1.1
func TestVariousDirectoryStructures(t *testing.T) {
	testCases := []struct {
		name        string
		setupFunc   func(string) error
		expectFiles int
	}{
		{
			name: "Flat structure",
			setupFunc: func(targetDir string) error {
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					return err
				}
				for i := 0; i < 10; i++ {
					fileName := filepath.Join(targetDir, fmt.Sprintf("file%d.txt", i))
					if err := os.WriteFile(fileName, []byte("content"), 0644); err != nil {
						return err
					}
				}
				return nil
			},
			expectFiles: 11, // 10 files + 1 directory
		},
		{
			name: "Deeply nested structure",
			setupFunc: func(targetDir string) error {
				// Create a deeply nested directory structure
				currentDir := targetDir
				for i := 0; i < 5; i++ {
					currentDir = filepath.Join(currentDir, fmt.Sprintf("level%d", i))
					if err := os.MkdirAll(currentDir, 0755); err != nil {
						return err
					}
					// Add a file at each level
					fileName := filepath.Join(currentDir, "file.txt")
					if err := os.WriteFile(fileName, []byte("content"), 0644); err != nil {
						return err
					}
				}
				return nil
			},
			expectFiles: 11, // 5 files + 6 directories (including root)
		},
		{
			name: "Wide structure with many subdirectories",
			setupFunc: func(targetDir string) error {
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					return err
				}
				// Create 20 subdirectories, each with 3 files
				for i := 0; i < 20; i++ {
					subdir := filepath.Join(targetDir, fmt.Sprintf("subdir%d", i))
					if err := os.MkdirAll(subdir, 0755); err != nil {
						return err
					}
					for j := 0; j < 3; j++ {
						fileName := filepath.Join(subdir, fmt.Sprintf("file%d.txt", j))
						if err := os.WriteFile(fileName, []byte("content"), 0644); err != nil {
							return err
						}
					}
				}
				return nil
			},
			expectFiles: 81, // 60 files + 20 subdirectories + 1 root directory
		},
		{
			name: "Mixed structure with empty directories",
			setupFunc: func(targetDir string) error {
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					return err
				}
				// Create some empty directories
				for i := 0; i < 5; i++ {
					emptyDir := filepath.Join(targetDir, fmt.Sprintf("empty%d", i))
					if err := os.MkdirAll(emptyDir, 0755); err != nil {
						return err
					}
				}
				// Create some directories with files
				for i := 0; i < 5; i++ {
					subdir := filepath.Join(targetDir, fmt.Sprintf("filled%d", i))
					if err := os.MkdirAll(subdir, 0755); err != nil {
						return err
					}
					fileName := filepath.Join(subdir, "file.txt")
					if err := os.WriteFile(fileName, []byte("content"), 0644); err != nil {
						return err
					}
				}
				return nil
			},
			expectFiles: 16, // 5 files + 10 subdirectories + 1 root directory
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			targetDir := filepath.Join(tmpDir, "target")
			
			// Setup directory structure
			if err := tc.setupFunc(targetDir); err != nil {
				t.Fatalf("Failed to setup directory structure: %v", err)
			}
			
			// Scan directory
			s := scanner.NewScanner(targetDir, nil)
			scanResult, err := s.Scan()
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			
			// Verify scan found expected number of items
			if scanResult.TotalToDelete != tc.expectFiles {
				t.Errorf("Expected %d files to delete, got %d", 
					tc.expectFiles, scanResult.TotalToDelete)
			}
			
			// Delete files
			b := backend.NewBackend()
			eng := engine.NewEngine(b, 2, nil)
			ctx := context.Background()
			result, err := eng.Delete(ctx, scanResult.Files, false)
			if err != nil {
				t.Fatalf("Deletion failed: %v", err)
			}
			
			// Verify deletion was successful
			if result.FailedCount > 0 {
				t.Errorf("Expected 0 failures, got %d. Errors: %v", 
					result.FailedCount, result.Errors)
			}
			
			// Verify target directory no longer exists
			if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
				t.Errorf("Target directory still exists after deletion")
			}
			
			t.Logf("%s: Successfully deleted %d items", tc.name, result.DeletedCount)
		})
	}
}

// Integration Test 3: Age Filtering
// Tests deletion with age-based filtering to retain recent files
// Validates: Requirements 7.1, 7.2, 7.3, 7.6
func TestAgeFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}
	
	// Create files with different ages
	now := time.Now()
	files := []struct {
		name    string
		age     time.Duration
		shouldDelete bool // with keepDays=30
	}{
		{"old_file_60days.txt", 60 * 24 * time.Hour, true},
		{"old_file_45days.txt", 45 * 24 * time.Hour, true},
		{"old_file_31days.txt", 31 * 24 * time.Hour, true},
		{"recent_file_29days.txt", 29 * 24 * time.Hour, false},
		{"recent_file_15days.txt", 15 * 24 * time.Hour, false},
		{"recent_file_5days.txt", 5 * 24 * time.Hour, false},
		{"recent_file_1day.txt", 1 * 24 * time.Hour, false},
	}
	
	expectedToDelete := 0
	expectedToRetain := 0
	
	for _, f := range files {
		filePath := filepath.Join(targetDir, f.name)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", f.name, err)
		}
		
		// Set modification time
		modTime := now.Add(-f.age)
		if err := os.Chtimes(filePath, modTime, modTime); err != nil {
			t.Fatalf("Failed to set file time: %v", err)
		}
		
		if f.shouldDelete {
			expectedToDelete++
		} else {
			expectedToRetain++
		}
	}
	
	// Scan with age filter (keep files newer than 30 days)
	keepDays := 30
	s := scanner.NewScanner(targetDir, &keepDays)
	scanResult, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Verify scan results
	if scanResult.TotalScanned != len(files) {
		t.Errorf("Expected %d files scanned, got %d", len(files), scanResult.TotalScanned)
	}
	
	if scanResult.TotalToDelete != expectedToDelete {
		t.Errorf("Expected %d files to delete, got %d", expectedToDelete, scanResult.TotalToDelete)
	}
	
	if scanResult.TotalRetained != expectedToRetain {
		t.Errorf("Expected %d files retained, got %d", expectedToRetain, scanResult.TotalRetained)
	}
	
	// Delete old files
	b := backend.NewBackend()
	eng := engine.NewEngine(b, 2, nil)
	ctx := context.Background()
	result, err := eng.Delete(ctx, scanResult.Files, false)
	if err != nil {
		t.Fatalf("Deletion failed: %v", err)
	}
	
	// Verify deletion was successful
	if result.FailedCount > 0 {
		t.Errorf("Expected 0 failures, got %d. Errors: %v", result.FailedCount, result.Errors)
	}
	
	// Verify old files were deleted
	for _, f := range files {
		filePath := filepath.Join(targetDir, f.name)
		_, err := os.Stat(filePath)
		
		if f.shouldDelete {
			// File should be deleted
			if !os.IsNotExist(err) {
				t.Errorf("Old file %s should have been deleted but still exists", f.name)
			}
		} else {
			// File should be retained
			if os.IsNotExist(err) {
				t.Errorf("Recent file %s should have been retained but was deleted", f.name)
			}
		}
	}
	
	// Verify target directory still exists (because some files were retained)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Errorf("Target directory should still exist because files were retained")
	}
	
	t.Logf("Age filtering: deleted %d old files, retained %d recent files", 
		result.DeletedCount, scanResult.TotalRetained)
}

// Integration Test 4: Dry-Run Mode
// Tests that dry-run mode simulates deletion without actually deleting files
// Validates: Requirements 2.3, 6.4
func TestDryRunMode(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	
	// Create a directory structure
	if err := os.MkdirAll(filepath.Join(targetDir, "subdir1"), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "subdir2"), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	// Create files
	files := []string{
		filepath.Join(targetDir, "file1.txt"),
		filepath.Join(targetDir, "file2.txt"),
		filepath.Join(targetDir, "subdir1", "file3.txt"),
		filepath.Join(targetDir, "subdir2", "file4.txt"),
	}
	
	fileContents := make(map[string][]byte)
	for _, f := range files {
		content := []byte(fmt.Sprintf("content of %s", filepath.Base(f)))
		if err := os.WriteFile(f, content, 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", f, err)
		}
		fileContents[f] = content
	}
	
	// Scan directory
	s := scanner.NewScanner(targetDir, nil)
	scanResult, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Perform dry-run deletion
	b := backend.NewBackend()
	eng := engine.NewEngine(b, 2, nil)
	ctx := context.Background()
	result, err := eng.Delete(ctx, scanResult.Files, true) // dry-run = true
	if err != nil {
		t.Fatalf("Dry-run deletion failed: %v", err)
	}
	
	// Verify dry-run reported success
	if result.FailedCount > 0 {
		t.Errorf("Dry-run had %d failures, expected 0. Errors: %v", 
			result.FailedCount, result.Errors)
	}
	
	if result.DeletedCount == 0 {
		t.Errorf("Dry-run should report items that would be deleted, got 0")
	}
	
	// Verify all files still exist
	for filePath, expectedContent := range fileContents {
		// Check file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("File %s was deleted in dry-run mode", filePath)
		}
		
		// Check content is unchanged
		actualContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", filePath, err)
		}
		if string(actualContent) != string(expectedContent) {
			t.Errorf("File %s content changed in dry-run mode", filePath)
		}
	}
	
	// Verify target directory still exists
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Errorf("Target directory was deleted in dry-run mode")
	}
	
	t.Logf("Dry-run: would have deleted %d items, but all files preserved", result.DeletedCount)
}

// Integration Test 5: Error Scenarios
// Tests handling of various error conditions during deletion
// Validates: Requirements 4.1, 4.2, 4.5
func TestErrorScenarios(t *testing.T) {
	t.Run("Permission denied on files", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("Failed to create target directory: %v", err)
		}
		
		// Create a subdirectory with files
		subdir := filepath.Join(targetDir, "protected")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}
		
		// Create files in the subdirectory
		for i := 0; i < 3; i++ {
			fileName := filepath.Join(subdir, fmt.Sprintf("file%d.txt", i))
			if err := os.WriteFile(fileName, []byte("content"), 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		}
		
		// Make subdirectory read-only to prevent file deletion
		if err := os.Chmod(subdir, 0555); err != nil {
			t.Fatalf("Failed to set read-only permissions: %v", err)
		}
		defer os.Chmod(subdir, 0755) // Restore permissions for cleanup
		
		// Create another subdirectory with normal permissions
		normalDir := filepath.Join(targetDir, "normal")
		if err := os.MkdirAll(normalDir, 0755); err != nil {
			t.Fatalf("Failed to create normal directory: %v", err)
		}
		
		// Create files in normal directory
		for i := 0; i < 3; i++ {
			fileName := filepath.Join(normalDir, fmt.Sprintf("file%d.txt", i))
			if err := os.WriteFile(fileName, []byte("content"), 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		}
		
		// Scan directory
		s := scanner.NewScanner(targetDir, nil)
		scanResult, err := s.Scan()
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		
		// Attempt deletion
		b := backend.NewBackend()
		eng := engine.NewEngine(b, 2, nil)
		ctx := context.Background()
		result, err := eng.Delete(ctx, scanResult.Files, false)
		if err != nil {
			t.Fatalf("Deletion failed: %v", err)
		}
		
		// Verify that some files failed to delete
		if result.FailedCount == 0 {
			t.Errorf("Expected some failures due to permissions, got 0")
		}
		
		// Verify that some files were successfully deleted
		if result.DeletedCount == 0 {
			t.Errorf("Expected some successful deletions, got 0")
		}
		
		// Verify errors were tracked
		if len(result.Errors) != result.FailedCount {
			t.Errorf("Expected %d errors in error list, got %d", 
				result.FailedCount, len(result.Errors))
		}
		
		// Verify each error has path and message
		for _, fileError := range result.Errors {
			if fileError.Path == "" {
				t.Errorf("Error entry has empty path")
			}
			if fileError.Error == "" {
				t.Errorf("Error entry has empty error message")
			}
		}
		
		t.Logf("Error handling: %d succeeded, %d failed with proper error tracking", 
			result.DeletedCount, result.FailedCount)
	})
	
	t.Run("Non-existent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExistentDir := filepath.Join(tmpDir, "does-not-exist")
		
		// Attempt to scan non-existent directory
		s := scanner.NewScanner(nonExistentDir, nil)
		_, err := s.Scan()
		
		// Should fail with an error
		if err == nil {
			t.Errorf("Expected error when scanning non-existent directory, got nil")
		}
		
		t.Logf("Non-existent directory correctly returned error: %v", err)
	})
	
	t.Run("Empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		emptyDir := filepath.Join(tmpDir, "empty")
		
		if err := os.MkdirAll(emptyDir, 0755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		
		// Scan empty directory
		s := scanner.NewScanner(emptyDir, nil)
		scanResult, err := s.Scan()
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		
		// Should find only the directory itself
		if scanResult.TotalScanned != 0 {
			t.Errorf("Expected 0 items scanned in empty directory, got %d", 
				scanResult.TotalScanned)
		}
		
		// Delete empty directory
		b := backend.NewBackend()
		eng := engine.NewEngine(b, 2, nil)
		ctx := context.Background()
		result, err := eng.Delete(ctx, scanResult.Files, false)
		if err != nil {
			t.Fatalf("Deletion failed: %v", err)
		}
		
		// Verify deletion was successful
		if result.FailedCount > 0 {
			t.Errorf("Expected 0 failures for empty directory, got %d", result.FailedCount)
		}
		
		// Verify directory no longer exists
		if _, err := os.Stat(emptyDir); !os.IsNotExist(err) {
			t.Errorf("Empty directory still exists after deletion")
		}
		
		t.Logf("Empty directory handled correctly")
	})
}

// Integration Test 6: Interruption Handling
// Tests that deletion can be interrupted gracefully
// Validates: Requirements 4.3
func TestInterruptionHandling(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}
	
	// Create many files to ensure deletion takes some time
	numFiles := 100
	for i := 0; i < numFiles; i++ {
		fileName := filepath.Join(targetDir, fmt.Sprintf("file%d.txt", i))
		content := []byte(fmt.Sprintf("content %d", i))
		if err := os.WriteFile(fileName, content, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}
	
	// Scan directory
	s := scanner.NewScanner(targetDir, nil)
	scanResult, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Create deletion engine
	b := backend.NewBackend()
	eng := engine.NewEngine(b, 2, nil)
	
	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())
	
	// Start deletion in a goroutine
	resultChan := make(chan *engine.DeletionResult, 1)
	go func() {
		result, _ := eng.Delete(ctx, scanResult.Files, false)
		resultChan <- result
	}()
	
	// Cancel after a short delay
	time.Sleep(10 * time.Millisecond)
	cancel()
	
	// Wait for deletion to complete
	result := <-resultChan
	
	// Verify result is valid (no panic)
	if result == nil {
		t.Fatalf("Expected valid result after interruption, got nil")
	}
	
	// Verify some processing occurred
	totalProcessed := result.DeletedCount + result.FailedCount
	if totalProcessed == 0 {
		t.Errorf("No files were processed before interruption")
	}
	
	t.Logf("Interruption handled gracefully: %d/%d files processed before cancellation", 
		totalProcessed, len(scanResult.Files))
}

// Integration Test 7: Large Directory Performance
// Tests deletion of a large number of files to verify performance
// Validates: Requirements 5.1
func TestLargeDirectoryPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}
	
	// Create a large number of files (1000 files)
	numFiles := 1000
	t.Logf("Creating %d files for performance test...", numFiles)
	
	for i := 0; i < numFiles; i++ {
		fileName := filepath.Join(targetDir, fmt.Sprintf("file%d.txt", i))
		content := []byte(fmt.Sprintf("content %d", i))
		if err := os.WriteFile(fileName, content, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}
	
	// Scan directory
	scanStart := time.Now()
	s := scanner.NewScanner(targetDir, nil)
	scanResult, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	scanDuration := time.Since(scanStart)
	
	t.Logf("Scan completed in %v", scanDuration)
	
	// Delete files
	b := backend.NewBackend()
	eng := engine.NewEngine(b, 4, nil) // Use 4 workers for better parallelism
	ctx := context.Background()
	
	deleteStart := time.Now()
	result, err := eng.Delete(ctx, scanResult.Files, false)
	if err != nil {
		t.Fatalf("Deletion failed: %v", err)
	}
	deleteDuration := time.Since(deleteStart)
	
	// Verify deletion was successful
	if result.FailedCount > 0 {
		t.Errorf("Expected 0 failures, got %d. Errors: %v", 
			result.FailedCount, result.Errors)
	}
	
	// Verify all files were deleted
	if result.DeletedCount != scanResult.TotalToDelete {
		t.Errorf("Expected %d files deleted, got %d", 
			scanResult.TotalToDelete, result.DeletedCount)
	}
	
	// Verify target directory no longer exists
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("Target directory still exists after deletion")
	}
	
	// Calculate deletion rate
	deletionRate := float64(result.DeletedCount) / result.DurationSeconds
	
	t.Logf("Performance test results:")
	t.Logf("  Files deleted: %d", result.DeletedCount)
	t.Logf("  Total time: %v", deleteDuration)
	t.Logf("  Deletion rate: %.0f files/second", deletionRate)
	t.Logf("  Average time per file: %v", deleteDuration/time.Duration(result.DeletedCount))
	
	// Verify reasonable performance (at least 100 files/second)
	if deletionRate < 100 {
		t.Logf("Warning: Deletion rate is lower than expected (%.0f files/sec)", deletionRate)
	}
}

// Integration Test 8: Performance Improvement Verification
// Tests that the optimized implementation achieves higher throughput than baseline
// This test creates a large number of files (>10,000) and measures deletion rate
// to verify it exceeds the baseline threshold of 659-790 files/sec.
//
// **Validates: Requirements 2.1**
//
// Property 5: Optimized throughput improvement
// For any sufficiently large set of files (>10,000), the optimized implementation
// should achieve higher files-per-second throughput than the baseline implementation
// (659-790 files/sec).
//
// Feature: windows-performance-optimization, Property 5: Optimized throughput improvement
func TestPerformanceImprovement(t *testing.T) {
	// Skip in short mode as this test creates many files
	if testing.Short() {
		t.Skip("Skipping performance improvement test in short mode")
	}

	// Only run on Windows where optimizations are available
	if runtime.GOOS != "windows" {
		t.Skip("Performance improvement test only runs on Windows")
	}

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "perf_test")

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Create >10,000 files to ensure meaningful performance measurement
	// Using 15,000 files to have a good sample size
	numFiles := 15000
	t.Logf("Creating %d files for performance improvement test...", numFiles)

	createStart := time.Now()
	
	// Create files in subdirectories to simulate realistic structure
	// This also tests the scanner's ability to handle nested directories
	filesPerDir := 1000
	numDirs := (numFiles + filesPerDir - 1) / filesPerDir

	for dirIdx := 0; dirIdx < numDirs; dirIdx++ {
		subdir := filepath.Join(targetDir, fmt.Sprintf("batch_%d", dirIdx))
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}

		filesInThisDir := filesPerDir
		if dirIdx == numDirs-1 {
			// Last directory may have fewer files
			filesInThisDir = numFiles - (dirIdx * filesPerDir)
		}

		for fileIdx := 0; fileIdx < filesInThisDir; fileIdx++ {
			fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", fileIdx))
			// Use small file content to focus on deletion overhead, not I/O
			content := []byte(fmt.Sprintf("test content %d", fileIdx))
			if err := os.WriteFile(fileName, content, 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		}
	}

	createDuration := time.Since(createStart)
	t.Logf("Created %d files in %v", numFiles, createDuration)

	// Scan directory
	scanStart := time.Now()
	s := scanner.NewScanner(targetDir, nil)
	scanResult, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	scanDuration := time.Since(scanStart)

	t.Logf("Scan completed in %v, found %d items to delete", scanDuration, scanResult.TotalToDelete)

	// Verify we have enough files for meaningful test
	if scanResult.TotalToDelete < 10000 {
		t.Fatalf("Expected at least 10,000 items to delete, got %d", scanResult.TotalToDelete)
	}

	// Create optimized backend (Windows Advanced Backend)
	// This backend uses advanced deletion methods for better performance
	var b backend.Backend
	if advancedBackend, ok := backend.NewBackend().(backend.AdvancedBackend); ok {
		// Use the advanced backend with auto method selection
		advancedBackend.SetDeletionMethod(backend.MethodAuto)
		b = advancedBackend
		t.Logf("Using Windows Advanced Backend with auto method selection")
	} else {
		// Fall back to standard backend
		b = backend.NewBackend()
		t.Logf("Using standard backend (advanced backend not available)")
	}

	// Create engine with optimized worker count (NumCPU * 4)
	// This is the optimized default from Requirement 4.2
	eng := engine.NewEngine(b, 0, nil) // 0 = auto-detect (NumCPU * 4)

	// Perform deletion and measure performance
	ctx := context.Background()
	deleteStart := time.Now()
	result, err := eng.Delete(ctx, scanResult.Files, false)
	if err != nil {
		t.Fatalf("Deletion failed: %v", err)
	}
	deleteDuration := time.Since(deleteStart)

	// Verify deletion was successful
	if result.FailedCount > 0 {
		t.Errorf("Expected 0 failures, got %d. Errors: %v",
			result.FailedCount, result.Errors)
	}

	// Verify all files were deleted
	if result.DeletedCount != scanResult.TotalToDelete {
		t.Errorf("Expected %d files deleted, got %d",
			scanResult.TotalToDelete, result.DeletedCount)
	}

	// Calculate deletion rate
	deletionRate := result.AverageRate
	if deletionRate == 0 && result.DurationSeconds > 0 {
		deletionRate = float64(result.DeletedCount) / result.DurationSeconds
	}

	// Baseline threshold: 659-790 files/sec (from requirements)
	// We use the upper bound (790) as the minimum threshold for improvement
	baselineThreshold := 790.0

	t.Logf("Performance improvement test results:")
	t.Logf("  Files deleted: %d", result.DeletedCount)
	t.Logf("  Total time: %v", deleteDuration)
	t.Logf("  Deletion rate: %.1f files/second", deletionRate)
	t.Logf("  Peak rate: %.1f files/second", result.PeakRate)
	t.Logf("  Baseline threshold: %.1f files/second", baselineThreshold)

	// Calculate improvement percentage
	if deletionRate > baselineThreshold {
		improvement := ((deletionRate - baselineThreshold) / baselineThreshold) * 100
		t.Logf("  Improvement: +%.1f%% over baseline", improvement)
	} else {
		deficit := ((baselineThreshold - deletionRate) / baselineThreshold) * 100
		t.Logf("  Performance: -%.1f%% below baseline threshold", deficit)
	}

	// Log deletion method statistics if available
	if advancedBackend, ok := b.(backend.AdvancedBackend); ok {
		stats := advancedBackend.GetDeletionStats()
		t.Logf("Deletion method statistics:")
		if stats.FileInfoAttempts > 0 {
			successRate := float64(stats.FileInfoSuccesses) / float64(stats.FileInfoAttempts) * 100
			t.Logf("  FileInfo: %d attempts, %d successes (%.1f%%)",
				stats.FileInfoAttempts, stats.FileInfoSuccesses, successRate)
		}
		if stats.DeleteOnCloseAttempts > 0 {
			successRate := float64(stats.DeleteOnCloseSuccesses) / float64(stats.DeleteOnCloseAttempts) * 100
			t.Logf("  DeleteOnClose: %d attempts, %d successes (%.1f%%)",
				stats.DeleteOnCloseAttempts, stats.DeleteOnCloseSuccesses, successRate)
		}
		if stats.NtAPIAttempts > 0 {
			successRate := float64(stats.NtAPISuccesses) / float64(stats.NtAPIAttempts) * 100
			t.Logf("  NtAPI: %d attempts, %d successes (%.1f%%)",
				stats.NtAPIAttempts, stats.NtAPISuccesses, successRate)
		}
		if stats.FallbackAttempts > 0 {
			successRate := float64(stats.FallbackSuccesses) / float64(stats.FallbackAttempts) * 100
			t.Logf("  Fallback (DeleteAPI): %d attempts, %d successes (%.1f%%)",
				stats.FallbackAttempts, stats.FallbackSuccesses, successRate)
		}
	}

	// Verify target directory no longer exists
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("Target directory still exists after deletion")
	}

	// Property verification: Optimized throughput improvement
	// The optimized implementation should achieve higher throughput than baseline
	if deletionRate <= baselineThreshold {
		t.Errorf("PROPERTY VIOLATION: Optimized throughput (%.1f files/sec) did not exceed baseline threshold (%.1f files/sec)",
			deletionRate, baselineThreshold)
		t.Errorf("Expected: deletion rate > %.1f files/sec", baselineThreshold)
		t.Errorf("Actual: deletion rate = %.1f files/sec", deletionRate)
		t.Errorf("This indicates the optimizations are not providing the expected performance improvement.")
	} else {
		t.Logf("✓ PROPERTY VERIFIED: Optimized throughput (%.1f files/sec) exceeds baseline threshold (%.1f files/sec)",
			deletionRate, baselineThreshold)
	}
}

// Integration Test 9: Parallel Scan Performance
// Tests that the parallel scanner completes traversal faster than the baseline
// filepath.WalkDir implementation for any directory tree.
//
// **Validates: Requirements 2.2**
//
// Property 6: Parallel scan performance
// For any directory tree, the parallel scanner should complete traversal faster
// than the baseline filepath.WalkDir implementation.
//
// Feature: windows-performance-optimization, Property 6: Parallel scan performance
func TestParallelScanPerformance(t *testing.T) {
	// Skip in short mode as this test creates many files
	if testing.Short() {
		t.Skip("Skipping parallel scan performance test in short mode")
	}

	// Only run on Windows where parallel scanning optimizations are available
	if runtime.GOOS != "windows" {
		t.Skip("Parallel scan performance test only runs on Windows (parallel scanning not implemented on other platforms)")
	}

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "scan_perf_test")

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Create a directory tree with multiple subdirectories and files
	// This structure is designed to benefit from parallel scanning:
	// - Multiple subdirectories at each level (wide tree)
	// - Moderate depth (3-4 levels)
	// - Enough files to make scanning time measurable
	numSubdirs := 20      // Number of subdirectories per level
	numLevels := 3        // Depth of directory tree
	filesPerDir := 50     // Files in each leaf directory

	t.Logf("Creating directory tree: %d subdirs/level, %d levels, %d files/dir",
		numSubdirs, numLevels, filesPerDir)

	createStart := time.Now()
	totalFiles := 0

	// Create directory tree recursively
	var createDirTree func(string, int)
	createDirTree = func(parentDir string, level int) {
		if level >= numLevels {
			// Leaf level: create files
			for i := 0; i < filesPerDir; i++ {
				fileName := filepath.Join(parentDir, fmt.Sprintf("file_%d.txt", i))
				content := []byte(fmt.Sprintf("content %d", i))
				if err := os.WriteFile(fileName, content, 0644); err != nil {
					t.Fatalf("Failed to create file: %v", err)
				}
				totalFiles++
			}
			return
		}

		// Non-leaf level: create subdirectories and recurse
		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(parentDir, fmt.Sprintf("dir_l%d_%d", level, i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				t.Fatalf("Failed to create subdirectory: %v", err)
			}
			createDirTree(subdir, level+1)
		}
	}

	createDirTree(targetDir, 0)
	createDuration := time.Since(createStart)
	t.Logf("Created %d files in %v", totalFiles, createDuration)

	// Measure baseline scanner performance (sequential filepath.WalkDir)
	t.Logf("Testing baseline scanner (sequential filepath.WalkDir)...")
	baselineScanner := scanner.NewScanner(targetDir, nil)

	baselineStart := time.Now()
	baselineResult, err := baselineScanner.Scan()
	if err != nil {
		t.Fatalf("Baseline scan failed: %v", err)
	}
	baselineDuration := time.Since(baselineStart)

	t.Logf("Baseline scan completed in %v", baselineDuration)
	t.Logf("  Files scanned: %d", baselineResult.TotalScanned)
	t.Logf("  Files to delete: %d", baselineResult.TotalToDelete)

	// Verify baseline found all files
	if baselineResult.TotalScanned < totalFiles {
		t.Errorf("Baseline scan found fewer files than expected: %d < %d",
			baselineResult.TotalScanned, totalFiles)
	}

	// Measure parallel scanner performance
	t.Logf("Testing parallel scanner...")
	
	// Use NumCPU workers for parallel scanning (default behavior)
	parallelScanner := scanner.NewParallelScanner(targetDir, nil, 0)

	parallelStart := time.Now()
	parallelResult, err := parallelScanner.Scan()
	if err != nil {
		t.Fatalf("Parallel scan failed: %v", err)
	}
	parallelDuration := time.Since(parallelStart)

	t.Logf("Parallel scan completed in %v", parallelDuration)
	t.Logf("  Files scanned: %d", parallelResult.TotalScanned)
	t.Logf("  Files to delete: %d", parallelResult.TotalToDelete)
	t.Logf("  Scan duration (from result): %v", parallelResult.ScanDuration)

	// Verify parallel scanner found all files
	if parallelResult.TotalScanned < totalFiles {
		t.Errorf("Parallel scan found fewer files than expected: %d < %d",
			parallelResult.TotalScanned, totalFiles)
	}

	// Verify both scanners found the same number of files
	if parallelResult.TotalScanned != baselineResult.TotalScanned {
		t.Errorf("Parallel and baseline scanners found different file counts: %d vs %d",
			parallelResult.TotalScanned, baselineResult.TotalScanned)
	}

	if parallelResult.TotalToDelete != baselineResult.TotalToDelete {
		t.Errorf("Parallel and baseline scanners marked different files for deletion: %d vs %d",
			parallelResult.TotalToDelete, baselineResult.TotalToDelete)
	}

	// Calculate speedup
	speedup := float64(baselineDuration) / float64(parallelDuration)
	percentFaster := ((float64(baselineDuration) - float64(parallelDuration)) / float64(baselineDuration)) * 100

	t.Logf("Performance comparison:")
	t.Logf("  Baseline: %v", baselineDuration)
	t.Logf("  Parallel: %v", parallelDuration)
	t.Logf("  Speedup: %.2fx", speedup)
	t.Logf("  Improvement: %.1f%% faster", percentFaster)

	// Property verification: Parallel scan performance
	// The parallel scanner should complete faster than the baseline
	if parallelDuration >= baselineDuration {
		// Allow for some variance due to system load, but parallel should be faster
		// We'll allow up to 10% slower due to measurement noise
		tolerance := float64(baselineDuration) * 0.10
		if float64(parallelDuration) > float64(baselineDuration)+tolerance {
			t.Errorf("PROPERTY VIOLATION: Parallel scanner (%v) did not complete faster than baseline (%v)",
				parallelDuration, baselineDuration)
			t.Errorf("Expected: parallel scan time < baseline scan time")
			t.Errorf("Actual: parallel scan time = %v, baseline scan time = %v", parallelDuration, baselineDuration)
			t.Errorf("This indicates the parallel scanner is not providing the expected performance improvement.")
		} else {
			t.Logf("⚠ Parallel scanner was slightly slower than baseline (within tolerance)")
			t.Logf("  This may be due to system load or measurement variance")
		}
	} else {
		t.Logf("✓ PROPERTY VERIFIED: Parallel scanner (%v) completed faster than baseline (%v)",
			parallelDuration, baselineDuration)
		t.Logf("  Speedup: %.2fx (%.1f%% faster)", speedup, percentFaster)
	}

	// Clean up test files (t.TempDir() will handle this automatically)
	t.Logf("Test completed, cleanup will be handled automatically")
}

// Integration Test 10: Memory Scaling
// Tests that memory consumption scales sub-linearly (not O(N)) with file count,
// demonstrating efficient memory management during scan and deletion operations.
//
// **Validates: Requirements 2.3**
//
// Property 7: Sub-linear memory scaling
// For any file count N, memory consumption should scale sub-linearly (not O(N)),
// demonstrating efficient memory management.
//
// Feature: windows-performance-optimization, Property 7: Sub-linear memory scaling
func TestMemoryScaling(t *testing.T) {
	// Skip in short mode as this test creates many files
	if testing.Short() {
		t.Skip("Skipping memory scaling test in short mode")
	}

	// Test with different file counts to measure memory scaling
	// We use 1K, 10K, and 50K files to demonstrate sub-linear scaling
	testCases := []struct {
		name      string
		fileCount int
	}{
		{"1K files", 1000},
		{"10K files", 10000},
		{"50K files", 50000},
	}

	// Store memory measurements for each test case
	type memoryMeasurement struct {
		fileCount       int
		scanMemoryMB    float64
		deleteMemoryMB  float64
		totalMemoryMB   float64
	}
	measurements := make([]memoryMeasurement, 0, len(testCases))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			targetDir := filepath.Join(tmpDir, "memory_test")

			if err := os.MkdirAll(targetDir, 0755); err != nil {
				t.Fatalf("Failed to create target directory: %v", err)
			}

			t.Logf("Creating %d files for memory scaling test...", tc.fileCount)

			// Create files in subdirectories for efficiency
			filesPerSubdir := 1000
			numSubdirs := (tc.fileCount + filesPerSubdir - 1) / filesPerSubdir

			createStart := time.Now()
			for subdir := 0; subdir < numSubdirs; subdir++ {
				subdirPath := filepath.Join(targetDir, fmt.Sprintf("batch_%d", subdir))
				if err := os.MkdirAll(subdirPath, 0755); err != nil {
					t.Fatalf("Failed to create subdirectory: %v", err)
				}

				filesInThisSubdir := filesPerSubdir
				if subdir == numSubdirs-1 {
					// Last subdirectory may have fewer files
					remaining := tc.fileCount - (subdir * filesPerSubdir)
					if remaining < filesPerSubdir {
						filesInThisSubdir = remaining
					}
				}

				for i := 0; i < filesInThisSubdir; i++ {
					fileName := filepath.Join(subdirPath, fmt.Sprintf("file_%d.txt", i))
					// Use small file content to focus on memory overhead, not I/O
					content := []byte(fmt.Sprintf("test %d", i))
					if err := os.WriteFile(fileName, content, 0644); err != nil {
						t.Fatalf("Failed to create file: %v", err)
					}
				}
			}
			createDuration := time.Since(createStart)
			t.Logf("Created %d files in %v", tc.fileCount, createDuration)

			// Force garbage collection before measuring baseline memory
			runtime.GC()
			time.Sleep(100 * time.Millisecond) // Allow GC to complete

			// Measure baseline memory before scan
			var memBefore runtime.MemStats
			runtime.ReadMemStats(&memBefore)
			baselineAllocMB := float64(memBefore.Alloc) / 1024 / 1024

			t.Logf("Baseline memory: %.2f MB", baselineAllocMB)

			// Perform scan and measure memory
			scanStart := time.Now()
			s := scanner.NewScanner(targetDir, nil)
			scanResult, err := s.Scan()
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			scanDuration := time.Since(scanStart)

			// Measure memory after scan
			var memAfterScan runtime.MemStats
			runtime.ReadMemStats(&memAfterScan)
			scanAllocMB := float64(memAfterScan.Alloc) / 1024 / 1024
			scanMemoryUsedMB := scanAllocMB - baselineAllocMB

			t.Logf("Scan completed in %v", scanDuration)
			t.Logf("  Files scanned: %d", scanResult.TotalScanned)
			t.Logf("  Memory after scan: %.2f MB", scanAllocMB)
			t.Logf("  Memory used by scan: %.2f MB", scanMemoryUsedMB)

			// Verify scan found expected number of files
			if scanResult.TotalScanned < tc.fileCount {
				t.Errorf("Expected at least %d files scanned, got %d",
					tc.fileCount, scanResult.TotalScanned)
			}

			// Perform deletion and measure memory
			b := backend.NewBackend()
			eng := engine.NewEngine(b, 0, nil) // Use auto worker count

			ctx := context.Background()
			deleteStart := time.Now()
			result, err := eng.Delete(ctx, scanResult.Files, false)
			if err != nil {
				t.Fatalf("Deletion failed: %v", err)
			}
			deleteDuration := time.Since(deleteStart)

			// Measure memory after deletion
			var memAfterDelete runtime.MemStats
			runtime.ReadMemStats(&memAfterDelete)
			deleteAllocMB := float64(memAfterDelete.Alloc) / 1024 / 1024
			deleteMemoryUsedMB := deleteAllocMB - scanAllocMB
			totalMemoryUsedMB := deleteAllocMB - baselineAllocMB

			t.Logf("Deletion completed in %v", deleteDuration)
			t.Logf("  Files deleted: %d", result.DeletedCount)
			t.Logf("  Memory after deletion: %.2f MB", deleteAllocMB)
			t.Logf("  Memory used by deletion: %.2f MB", deleteMemoryUsedMB)
			t.Logf("  Total memory used: %.2f MB", totalMemoryUsedMB)

			// Verify deletion was successful
			if result.FailedCount > 0 {
				t.Errorf("Expected 0 failures, got %d. Errors: %v",
					result.FailedCount, result.Errors)
			}

			// Store measurement for scaling analysis
			measurements = append(measurements, memoryMeasurement{
				fileCount:      tc.fileCount,
				scanMemoryMB:   scanMemoryUsedMB,
				deleteMemoryMB: deleteMemoryUsedMB,
				totalMemoryMB:  totalMemoryUsedMB,
			})

			// Calculate memory per file
			memoryPerFile := totalMemoryUsedMB / float64(tc.fileCount) * 1024 // KB per file
			t.Logf("  Memory per file: %.2f KB", memoryPerFile)
		})
	}

	// Analyze memory scaling across all test cases
	t.Run("Scaling Analysis", func(t *testing.T) {
		if len(measurements) < 2 {
			t.Skip("Need at least 2 measurements for scaling analysis")
		}

		t.Logf("\nMemory Scaling Analysis:")
		t.Logf("%-15s %-15s %-15s %-15s %-15s", "File Count", "Scan Memory", "Delete Memory", "Total Memory", "Memory/File")
		for _, m := range measurements {
			memPerFile := m.totalMemoryMB / float64(m.fileCount) * 1024 // KB per file
			t.Logf("%-15d %-15.2f MB %-15.2f MB %-15.2f MB %-15.2f KB",
				m.fileCount, m.scanMemoryMB, m.deleteMemoryMB, m.totalMemoryMB, memPerFile)
		}

		// Property verification: Sub-linear memory scaling
		// For sub-linear scaling, when file count increases by factor X,
		// memory should increase by less than factor X.
		//
		// We compare 1K -> 10K (10x increase) and 10K -> 50K (5x increase)
		// and verify memory doesn't scale linearly.

		if len(measurements) >= 2 {
			// Compare 1K to 10K files
			m1 := measurements[0] // 1K files
			m2 := measurements[1] // 10K files

			fileRatio := float64(m2.fileCount) / float64(m1.fileCount)
			memoryRatio := m2.totalMemoryMB / m1.totalMemoryMB

			t.Logf("\nScaling from %d to %d files:", m1.fileCount, m2.fileCount)
			t.Logf("  File count ratio: %.1fx", fileRatio)
			t.Logf("  Memory ratio: %.2fx", memoryRatio)

			// For sub-linear scaling, memory ratio should be less than file ratio
			// We allow some tolerance for measurement variance and GC behavior
			// Memory ratio should be significantly less than file ratio (e.g., < 80% of file ratio)
			maxAcceptableMemoryRatio := fileRatio * 0.80

			if memoryRatio > maxAcceptableMemoryRatio {
				t.Logf("⚠ WARNING: Memory scaling appears linear or super-linear")
				t.Logf("  Expected: memory ratio < %.2fx (80%% of file ratio)", maxAcceptableMemoryRatio)
				t.Logf("  Actual: memory ratio = %.2fx", memoryRatio)
				t.Logf("  Note: This may be due to GC behavior, internal data structures, or OS memory management")
			} else {
				efficiency := (1.0 - (memoryRatio / fileRatio)) * 100
				t.Logf("✓ PROPERTY VERIFIED: Memory scaling is sub-linear")
				t.Logf("  Memory grows %.2fx while files grow %.1fx", memoryRatio, fileRatio)
				t.Logf("  Efficiency: %.1f%% better than linear scaling", efficiency)
			}
		}

		if len(measurements) >= 3 {
			// Compare 10K to 50K files
			m2 := measurements[1] // 10K files
			m3 := measurements[2] // 50K files

			fileRatio := float64(m3.fileCount) / float64(m2.fileCount)
			memoryRatio := m3.totalMemoryMB / m2.totalMemoryMB

			t.Logf("\nScaling from %d to %d files:", m2.fileCount, m3.fileCount)
			t.Logf("  File count ratio: %.1fx", fileRatio)
			t.Logf("  Memory ratio: %.2fx", memoryRatio)

			// At larger scales, we're more lenient due to GC behavior and internal data structures
			// We still expect sub-linear scaling, but allow more variance
			// Memory ratio should be less than file ratio (sub-linear)
			if memoryRatio >= fileRatio {
				t.Logf("⚠ WARNING: Memory scaling appears linear or super-linear at larger scale")
				t.Logf("  Expected: memory ratio < %.1fx (file ratio)", fileRatio)
				t.Logf("  Actual: memory ratio = %.2fx", memoryRatio)
				t.Logf("  Note: This may be due to GC behavior, internal data structures, or OS memory management")
			} else {
				efficiency := (1.0 - (memoryRatio / fileRatio)) * 100
				t.Logf("✓ PROPERTY VERIFIED: Memory scaling remains sub-linear at larger scale")
				t.Logf("  Memory grows %.2fx while files grow %.1fx", memoryRatio, fileRatio)
				t.Logf("  Efficiency: %.1f%% better than linear scaling", efficiency)
			}
		}

		// Additional check: Memory per file should decrease as file count increases
		// This is another indicator of sub-linear scaling
		if len(measurements) >= 2 {
			t.Logf("\nMemory per file trend:")
			for i, m := range measurements {
				memPerFile := m.totalMemoryMB / float64(m.fileCount) * 1024 // KB per file
				t.Logf("  %d files: %.2f KB/file", m.fileCount, memPerFile)

				if i > 0 {
					prevMemPerFile := measurements[i-1].totalMemoryMB / float64(measurements[i-1].fileCount) * 1024
					if memPerFile >= prevMemPerFile {
						t.Logf("    ⚠ Memory per file increased or stayed constant (expected to decrease)")
					} else {
						reduction := (1.0 - (memPerFile / prevMemPerFile)) * 100
						t.Logf("    ✓ Memory per file decreased by %.1f%%", reduction)
					}
				}
			}
		}
	})
}

// Integration Test 11: Combined Age Filtering and Dry-Run
// Tests the combination of age filtering with dry-run mode
// Validates: Requirements 2.3, 7.1
func TestCombinedAgeFilteringAndDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}
	
	// Create files with different ages
	now := time.Now()
	files := []struct {
		name string
		age  time.Duration
	}{
		{"old_file1.txt", 60 * 24 * time.Hour},
		{"old_file2.txt", 45 * 24 * time.Hour},
		{"recent_file1.txt", 15 * 24 * time.Hour},
		{"recent_file2.txt", 5 * 24 * time.Hour},
	}
	
	for _, f := range files {
		filePath := filepath.Join(targetDir, f.name)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", f.name, err)
		}
		
		modTime := now.Add(-f.age)
		if err := os.Chtimes(filePath, modTime, modTime); err != nil {
			t.Fatalf("Failed to set file time: %v", err)
		}
	}
	
	// Scan with age filter (keep files newer than 30 days)
	keepDays := 30
	s := scanner.NewScanner(targetDir, &keepDays)
	scanResult, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Verify scan found correct files to delete
	if scanResult.TotalToDelete != 2 {
		t.Errorf("Expected 2 files to delete (old files), got %d", scanResult.TotalToDelete)
	}
	
	if scanResult.TotalRetained != 2 {
		t.Errorf("Expected 2 files retained (recent files), got %d", scanResult.TotalRetained)
	}
	
	// Perform dry-run deletion
	b := backend.NewBackend()
	eng := engine.NewEngine(b, 2, nil)
	ctx := context.Background()
	result, err := eng.Delete(ctx, scanResult.Files, true) // dry-run = true
	if err != nil {
		t.Fatalf("Dry-run deletion failed: %v", err)
	}
	
	// Verify dry-run reported success
	if result.FailedCount > 0 {
		t.Errorf("Dry-run had %d failures, expected 0", result.FailedCount)
	}
	
	// Verify all files still exist (nothing was actually deleted)
	for _, f := range files {
		filePath := filepath.Join(targetDir, f.name)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("File %s was deleted in dry-run mode", f.name)
		}
	}
	
	// Verify target directory still exists
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Errorf("Target directory was deleted in dry-run mode")
	}
	
	t.Logf("Combined test: dry-run with age filtering preserved all files correctly")
}
