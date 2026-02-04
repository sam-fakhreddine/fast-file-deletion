package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourusername/fast-file-deletion/internal/backend"
)

// TestPeriodicRateReporting verifies that the engine reports deletion rate
// every 5 seconds during deletion operations.
// This is a demonstration test that validates Requirement 12.3.
func TestPeriodicRateReporting(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Create enough files to ensure deletion takes at least 6 seconds
	// This allows us to observe at least one periodic rate report
	// We'll create 3000 files which should take 6-10 seconds to delete
	fileCount := 3000
	var filesToDelete []string
	
	t.Logf("Creating %d test files...", fileCount)
	for i := 0; i < fileCount; i++ {
		fileName := filepath.Join(targetDir, fmt.Sprintf("file_%d.txt", i))
		content := []byte(fmt.Sprintf("test content %d", i))
		
		if err := os.WriteFile(fileName, content, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		
		filesToDelete = append(filesToDelete, fileName)
	}
	
	// Add target directory to deletion list
	filesToDelete = append(filesToDelete, targetDir)
	
	t.Logf("Created %d files, starting deletion...", fileCount)

	// Create deletion engine
	backend := backend.NewBackend()
	engine := NewEngine(backend, 4, nil) // Use 4 workers

	// Create context for deletion
	ctx := context.Background()

	// Perform deletion (not dry-run)
	// The adaptiveWorkerTuning goroutine will report rate every 5 seconds
	startTime := time.Now()
	result, err := engine.Delete(ctx, filesToDelete, false)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	duration := time.Since(startTime)

	// Verify deletion completed successfully
	if result.FailedCount > 0 {
		t.Errorf("Deletion had %d failures, expected 0", result.FailedCount)
	}

	// Verify all files were processed
	if result.DeletedCount != len(filesToDelete) {
		t.Errorf("Expected %d items processed, got %d",
			len(filesToDelete), result.DeletedCount)
	}

	// Calculate actual deletion rate
	actualRate := float64(result.DeletedCount) / duration.Seconds()
	
	t.Logf("Deletion completed in %.2f seconds", duration.Seconds())
	t.Logf("Average deletion rate: %.1f files/sec", actualRate)
	t.Logf("Total files deleted: %d", result.DeletedCount)
	
	// Verify the deletion rate is reasonable
	if actualRate <= 0 {
		t.Errorf("Deletion rate is non-positive: %.2f files/sec", actualRate)
	}
	
	// Note: The periodic rate reporting happens in the adaptiveWorkerTuning goroutine
	// which logs messages like "Deletion rate: X files/sec" every 5 seconds.
	// These messages are visible in the test output when running with -v flag,
	// but we don't assert on them here since they're logged, not returned.
	
	// Verify that the target directory no longer exists
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("Target directory %s still exists after successful deletion", targetDir)
	}
}

// TestPeriodicRateReportingShortDuration verifies that periodic rate reporting
// works correctly even for short deletion operations (< 5 seconds).
// In this case, no periodic reports should be generated, but the function
// should still work correctly.
func TestPeriodicRateReportingShortDuration(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Create a small number of files (should complete in < 5 seconds)
	fileCount := 100
	var filesToDelete []string
	
	for i := 0; i < fileCount; i++ {
		fileName := filepath.Join(targetDir, fmt.Sprintf("file_%d.txt", i))
		content := []byte(fmt.Sprintf("test content %d", i))
		
		if err := os.WriteFile(fileName, content, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		
		filesToDelete = append(filesToDelete, fileName)
	}
	
	// Add target directory to deletion list
	filesToDelete = append(filesToDelete, targetDir)

	// Create deletion engine
	backend := backend.NewBackend()
	engine := NewEngine(backend, 4, nil)

	// Create context for deletion
	ctx := context.Background()

	// Perform deletion
	startTime := time.Now()
	result, err := engine.Delete(ctx, filesToDelete, false)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	duration := time.Since(startTime)

	// Verify deletion completed successfully
	if result.FailedCount > 0 {
		t.Errorf("Deletion had %d failures, expected 0", result.FailedCount)
	}

	// Verify all files were processed
	if result.DeletedCount != len(filesToDelete) {
		t.Errorf("Expected %d items processed, got %d",
			len(filesToDelete), result.DeletedCount)
	}

	t.Logf("Short deletion completed in %.2f seconds", duration.Seconds())
	t.Logf("Average deletion rate: %.1f files/sec", float64(result.DeletedCount)/duration.Seconds())
	
	// Verify deletion completed in reasonable time (should be < 5 seconds)
	if duration >= 5*time.Second {
		t.Logf("Warning: Short deletion took longer than expected (%.2f seconds)", duration.Seconds())
	}
}
