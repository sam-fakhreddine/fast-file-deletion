// Package internal provides integration tests for the Fast File Deletion tool.
// These tests verify the complete end-to-end workflow from CLI argument parsing
// through deletion execution, testing the integration of all components.
package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	deletedCount := 0
	progressCallback := func(count int) {
		deletedCount = count
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
	if deletedCount == 0 {
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

// Integration Test 8: Combined Age Filtering and Dry-Run
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
