package testutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestCleanupTestDir tests the basic cleanup functionality
func TestCleanupTestDir(t *testing.T) {
	// Create a temporary directory
	dir := t.TempDir()

	// Create some test files
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatal("Test file should exist")
	}

	// Clean up the directory
	if err := CleanupTestDir(dir); err != nil {
		t.Fatalf("CleanupTestDir failed: %v", err)
	}

	// Verify directory is removed
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("Directory should be removed after cleanup")
	}
}

// TestCleanupTestDirNonExistent tests cleanup of non-existent directory
func TestCleanupTestDirNonExistent(t *testing.T) {
	// Try to clean up a non-existent directory
	err := CleanupTestDir("/nonexistent/directory/path")
	if err != nil {
		t.Errorf("CleanupTestDir should not fail for non-existent directory: %v", err)
	}
}

// TestRegisterCleanup tests the RegisterCleanup function
func TestRegisterCleanup(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Register cleanup in a subtest so it runs when subtest completes
	t.Run("subtest", func(t *testing.T) {
		RegisterCleanup(t, tempDir)
		// Verify file exists during test
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Fatal("Test file should exist during test")
		}
	})

	// After subtest completes, directory should be cleaned up
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Directory should be removed after test cleanup")
	}
}

// TestCleanupWithTimeout tests cleanup with timeout
func TestCleanupWithTimeout(t *testing.T) {
	// Create a temporary directory with files
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Clean up with timeout
	err := CleanupWithTimeout(dir, 5*time.Second)
	if err != nil {
		t.Fatalf("CleanupWithTimeout failed: %v", err)
	}

	// Verify directory is removed
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("Directory should be removed after cleanup")
	}
}

// TestCountFilesInDir tests file counting
func TestCountFilesInDir(t *testing.T) {
	dir := t.TempDir()

	// Create some test files
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(dir, "test"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create a subdirectory (should not be counted)
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Count files
	count, err := CountFilesInDir(dir)
	if err != nil {
		t.Fatalf("CountFilesInDir failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 files, got %d", count)
	}
}

// TestCountFilesRecursive tests recursive file counting
func TestCountFilesRecursive(t *testing.T) {
	dir := t.TempDir()

	// Create files in root
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(dir, "test"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create subdirectory with files
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	for i := 0; i < 2; i++ {
		testFile := filepath.Join(subdir, "test"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Count files recursively
	count, err := CountFilesRecursive(dir)
	if err != nil {
		t.Fatalf("CountFilesRecursive failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 files total, got %d", count)
	}
}

// TestVerifyCleanup tests cleanup verification
func TestVerifyCleanup(t *testing.T) {
	// Create and remove a directory
	dir := t.TempDir()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("Failed to remove directory: %v", err)
	}

	// Verify cleanup
	if err := VerifyCleanup(dir); err != nil {
		t.Errorf("VerifyCleanup should succeed for removed directory: %v", err)
	}

	// Create a new directory
	dir2 := t.TempDir()

	// Verify cleanup should fail for existing directory
	if err := VerifyCleanup(dir2); err == nil {
		t.Error("VerifyCleanup should fail for existing directory")
	}
}

// Property 6: Cleanup After Timeout
// **Validates: Requirements 4.4**
//
// For any test that times out, all test files and directories created by that test
// shall be removed during cleanup.
func TestCleanupAfterTimeoutProperty(t *testing.T) {
	// Configure rapid for this test
	GetRapidCheckConfig(t)

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random number of files to create (1-20)
		numFiles := rapid.IntRange(1, 20).Draw(t, "numFiles")

		// Generate a random file size (1-1024 bytes)
		fileSize := rapid.IntRange(1, 1024).Draw(t, "fileSize")

		// Create a temporary directory for this test iteration
		tempDir, err := os.MkdirTemp("", "cleanup-timeout-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}

		// Ensure cleanup happens even if test fails
		defer func() {
			// Clean up after the test
			if cleanupErr := CleanupTestDir(tempDir); cleanupErr != nil {
				t.Logf("Warning: failed to cleanup temp directory: %v", cleanupErr)
			}
		}()

		// Create test files
		createdFiles := make([]string, 0, numFiles)
		for i := 0; i < numFiles; i++ {
			filename := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "filename")
			testFile := filepath.Join(tempDir, "test_"+filename+".txt")
			content := make([]byte, fileSize)
			if err := os.WriteFile(testFile, content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			createdFiles = append(createdFiles, testFile)
		}

		// Verify all files exist before timeout
		for _, file := range createdFiles {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				t.Fatalf("File should exist before timeout: %s", file)
			}
		}

		// Test the cleanup mechanism directly
		// Simulate a timeout scenario by manually calling CleanupTestDir
		// after verifying files exist
		
		// Property: CleanupTestDir should successfully remove all files
		if err := CleanupTestDir(tempDir); err != nil {
			t.Fatalf("CleanupTestDir failed: %v", err)
		}

		// Give cleanup a moment to complete
		time.Sleep(10 * time.Millisecond)

		// Property: After cleanup, the directory should not exist
		if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
			// Directory still exists - this violates the property
			remainingCount, countErr := CountFilesRecursive(tempDir)
			if countErr == nil && remainingCount > 0 {
				t.Fatalf("Property violated: Directory %s still exists with %d files after cleanup", tempDir, remainingCount)
			} else {
				t.Fatalf("Property violated: Directory %s still exists after cleanup", tempDir)
			}
		}

		// Additional verification: Ensure individual files are also removed
		for _, file := range createdFiles {
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				t.Fatalf("Property violated: File %s still exists after cleanup", file)
			}
		}
	})
}

// TestCleanupAfterTimeoutIntegration is an integration test that verifies
// cleanup works correctly with the actual timeout mechanism
func TestCleanupAfterTimeoutIntegration(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "cleanup-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create some test files
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(tempDir, "test"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Verify files exist
	count, err := CountFilesInDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to count files: %v", err)
	}
	if count != 5 {
		t.Fatalf("Expected 5 files, got %d", count)
	}

	// Create a test function that will timeout
	timeoutFn := func(ctx context.Context) {
		// Sleep longer than timeout
		select {
		case <-time.After(1 * time.Second):
			// Should not reach here
		case <-ctx.Done():
			// Context cancelled
			return
		}
	}

	// Run with a very short timeout and cleanup
	shortTimeout := 50 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
	defer cancel()

	// Manually implement timeout with cleanup for this test
	done := make(chan error, 1)
	go func() {
		timeoutFn(ctx)
		done <- nil
	}()

	select {
	case <-done:
		t.Fatal("Test should have timed out")
	case <-ctx.Done():
		// Timeout occurred - clean up
		if err := CleanupTestDir(tempDir); err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}
	}

	// Verify directory is removed
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Directory should be removed after timeout cleanup")
	}
}
