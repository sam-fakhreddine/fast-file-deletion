package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"pgregory.net/rapid"
)

// Feature: fast-file-deletion, Property 15: Platform Backend Selection
// For any operating system, the deletion engine should automatically select
// the appropriate backend (Windows-optimized or generic) based on the detected platform.
// Validates: Requirements 8.4
func TestPlatformBackendSelection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a backend using the factory function
		backend := NewBackend()

		// Verify that a backend was created
		if backend == nil {
			rt.Fatalf("NewBackend() returned nil")
		}

		// Create a temporary directory for testing
		tempDir := os.TempDir()
		testFile := filepath.Join(tempDir, fmt.Sprintf("test_%d.txt", rapid.Uint64().Draw(rt, "filenum")))

		// Create a test file
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			rt.Fatalf("Failed to create test file: %v", err)
		}
		// Ensure cleanup
		defer os.Remove(testFile)

		// Verify the backend can delete the file
		err = backend.DeleteFile(testFile)
		if err != nil {
			rt.Fatalf("Backend failed to delete file: %v", err)
		}

		// Verify the file was actually deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			rt.Fatalf("File still exists after deletion")
		}

		// Create a test directory
		testDir := filepath.Join(tempDir, fmt.Sprintf("testdir_%d", rapid.Uint64().Draw(rt, "dirnum")))
		err = os.Mkdir(testDir, 0755)
		if err != nil {
			rt.Fatalf("Failed to create test directory: %v", err)
		}
		// Ensure cleanup
		defer os.Remove(testDir)

		// Verify the backend can delete the directory
		err = backend.DeleteDirectory(testDir)
		if err != nil {
			rt.Fatalf("Backend failed to delete directory: %v", err)
		}

		// Verify the directory was actually deleted
		if _, err := os.Stat(testDir); !os.IsNotExist(err) {
			rt.Fatalf("Directory still exists after deletion")
		}

		// Verify the backend type matches the platform
		// We check this by verifying the backend name contains the expected string
		backendType := fmt.Sprintf("%T", backend)
		switch runtime.GOOS {
		case "windows":
			// On Windows, we should get WindowsBackend
			if backendType != "*backend.WindowsBackend" {
				rt.Fatalf("Expected WindowsBackend on Windows, got %s", backendType)
			}
		default:
			// On other platforms, we should get GenericBackend
			if backendType != "*backend.GenericBackend" {
				rt.Fatalf("Expected GenericBackend on %s, got %s", runtime.GOOS, backendType)
			}
		}
	})
}

// TestBackendInterfaceCompliance verifies that the backend implementations
// properly implement the Backend interface.
func TestBackendInterfaceCompliance(t *testing.T) {
	// Test that the current platform's backend implements Backend interface
	var _ Backend = NewBackend()
}

// TestNewBackendReturnsNonNil verifies that NewBackend always returns a valid backend.
func TestNewBackendReturnsNonNil(t *testing.T) {
	backend := NewBackend()
	if backend == nil {
		t.Fatal("NewBackend() returned nil")
	}
}

// TestDeleteFile_Success tests successful file deletion on the current platform's backend.
// Requirements: 4.1, 8.2, 8.3
func TestDeleteFile_Success(t *testing.T) {
	backend := NewBackend()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_delete_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// Verify file exists
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		t.Fatal("Temp file was not created")
	}

	// Delete the file using backend
	err = backend.DeleteFile(tempPath)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Verify file no longer exists
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("File still exists after deletion")
	}
}

// TestDeleteFile_NonExistent tests deletion of a non-existent file.
// Requirements: 4.1, 4.2
func TestDeleteFile_NonExistent(t *testing.T) {
	backend := NewBackend()

	// Try to delete a file that doesn't exist
	nonExistentPath := filepath.Join(os.TempDir(), "nonexistent_file_12345.txt")
	err := backend.DeleteFile(nonExistentPath)

	// Should return an error
	if err == nil {
		t.Error("Expected error when deleting non-existent file, got nil")
	}
}

// TestDeleteDirectory_Success tests successful directory deletion.
// Requirements: 4.1, 8.2, 8.3
func TestDeleteDirectory_Success(t *testing.T) {
	backend := NewBackend()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "test_delete_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Fatal("Temp directory was not created")
	}

	// Delete the directory using backend
	err = backend.DeleteDirectory(tempDir)
	if err != nil {
		t.Fatalf("DeleteDirectory failed: %v", err)
	}

	// Verify directory no longer exists
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Directory still exists after deletion")
	}
}

// TestDeleteDirectory_NonEmpty tests deletion of a non-empty directory.
// Requirements: 4.1, 4.2
func TestDeleteDirectory_NonEmpty(t *testing.T) {
	backend := NewBackend()

	// Create a temporary directory with a file in it
	tempDir, err := os.MkdirTemp("", "test_delete_nonempty_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup in case test fails

	// Create a file inside the directory
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to delete the non-empty directory
	err = backend.DeleteDirectory(tempDir)

	// Should return an error (directory not empty)
	if err == nil {
		t.Error("Expected error when deleting non-empty directory, got nil")
	}

	// Cleanup
	os.RemoveAll(tempDir)
}

// TestDeleteDirectory_NonExistent tests deletion of a non-existent directory.
// Requirements: 4.1, 4.2
func TestDeleteDirectory_NonExistent(t *testing.T) {
	backend := NewBackend()

	// Try to delete a directory that doesn't exist
	nonExistentPath := filepath.Join(os.TempDir(), "nonexistent_dir_12345")
	err := backend.DeleteDirectory(nonExistentPath)

	// Should return an error
	if err == nil {
		t.Error("Expected error when deleting non-existent directory, got nil")
	}
}

// TestDeleteFile_ReadOnlyPermissions tests deletion of a read-only file.
// This tests error handling for permission issues.
// Requirements: 4.1, 4.2
func TestDeleteFile_ReadOnlyPermissions(t *testing.T) {
	// Skip on Windows as permission handling is different
	if runtime.GOOS == "windows" {
		t.Skip("Skipping read-only test on Windows (different permission model)")
	}

	backend := NewBackend()

	// Create a temporary directory we can control
	tempDir, err := os.MkdirTemp("", "test_perm_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file in our controlled directory
	tempPath := filepath.Join(tempDir, "readonly.txt")
	err = os.WriteFile(tempPath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make the file read-only
	err = os.Chmod(tempPath, 0444)
	if err != nil {
		t.Fatalf("Failed to change file permissions: %v", err)
	}

	// Make parent directory read-only to prevent deletion
	err = os.Chmod(tempDir, 0555) // Read and execute only
	if err != nil {
		t.Skipf("Cannot change directory permissions (may require elevated privileges): %v", err)
	}
	defer os.Chmod(tempDir, 0755) // Restore permissions

	// Try to delete the file
	err = backend.DeleteFile(tempPath)

	// Should return an error due to permissions
	if err == nil {
		t.Error("Expected error when deleting file without write permissions, got nil")
	}

	// Restore permissions for cleanup
	os.Chmod(tempDir, 0755)
}

// TestDeleteMultipleFiles tests deleting multiple files in sequence.
// Requirements: 4.1, 8.2, 8.3
func TestDeleteMultipleFiles(t *testing.T) {
	backend := NewBackend()

	// Create multiple temporary files
	var tempFiles []string
	for i := 0; i < 5; i++ {
		tempFile, err := os.CreateTemp("", fmt.Sprintf("test_multi_%d_*.txt", i))
		if err != nil {
			t.Fatalf("Failed to create temp file %d: %v", i, err)
		}
		tempPath := tempFile.Name()
		tempFile.Close()
		tempFiles = append(tempFiles, tempPath)
	}

	// Delete all files
	for i, path := range tempFiles {
		err := backend.DeleteFile(path)
		if err != nil {
			t.Errorf("Failed to delete file %d (%s): %v", i, path, err)
		}

		// Verify file was deleted
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("File %d still exists after deletion", i)
		}
	}
}

// TestDeleteNestedDirectories tests deleting nested empty directories.
// Requirements: 4.1, 8.2, 8.3
func TestDeleteNestedDirectories(t *testing.T) {
	backend := NewBackend()

	// Create nested directories
	tempDir, err := os.MkdirTemp("", "test_nested_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup in case test fails

	subDir1 := filepath.Join(tempDir, "sub1")
	subDir2 := filepath.Join(subDir1, "sub2")
	err = os.MkdirAll(subDir2, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	// Delete from innermost to outermost
	err = backend.DeleteDirectory(subDir2)
	if err != nil {
		t.Errorf("Failed to delete innermost directory: %v", err)
	}

	err = backend.DeleteDirectory(subDir1)
	if err != nil {
		t.Errorf("Failed to delete middle directory: %v", err)
	}

	err = backend.DeleteDirectory(tempDir)
	if err != nil {
		t.Errorf("Failed to delete outermost directory: %v", err)
	}

	// Verify all directories were deleted
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Root directory still exists after deletion")
	}
}
