//go:build !windows

package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenericBackend_BasicFileOperation tests basic file deletion on non-Windows platforms.
// Requirements: 8.3
func TestGenericBackend_BasicFileOperation(t *testing.T) {
	backend := NewGenericBackend()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_generic_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// Delete using the backend
	err = backend.DeleteFile(tempPath)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("File still exists after deletion")
	}
}

// TestGenericBackend_BasicDirectoryOperation tests basic directory deletion.
// Requirements: 8.3
func TestGenericBackend_BasicDirectoryOperation(t *testing.T) {
	backend := NewGenericBackend()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "test_generic_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Delete using the backend
	err = backend.DeleteDirectory(tempDir)
	if err != nil {
		t.Fatalf("DeleteDirectory failed: %v", err)
	}

	// Verify directory was deleted
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Directory still exists after deletion")
	}
}

// TestGenericBackend_SymlinkHandling tests handling of symbolic links.
// Requirements: 8.3
func TestGenericBackend_SymlinkHandling(t *testing.T) {
	backend := NewGenericBackend()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_symlink_target_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	targetPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(targetPath)

	// Create a symlink to the file
	symlinkPath := filepath.Join(os.TempDir(), "test_symlink.txt")
	err = os.Symlink(targetPath, symlinkPath)
	if err != nil {
		t.Skipf("Skipping symlink test: %v", err)
	}
	defer os.Remove(symlinkPath)

	// Delete the symlink (not the target)
	err = backend.DeleteFile(symlinkPath)
	if err != nil {
		t.Fatalf("DeleteFile failed for symlink: %v", err)
	}

	// Verify symlink was deleted
	if _, err := os.Stat(symlinkPath); !os.IsNotExist(err) {
		t.Error("Symlink still exists after deletion")
	}

	// Verify target file still exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Error("Target file was incorrectly deleted")
	}
}

// TestGenericBackend_FileWithUnixPermissions tests files with various Unix permissions.
// Requirements: 4.1, 4.2, 8.3
func TestGenericBackend_FileWithUnixPermissions(t *testing.T) {
	backend := NewGenericBackend()

	permissions := []os.FileMode{
		0644, // rw-r--r--
		0755, // rwxr-xr-x
		0600, // rw-------
		0444, // r--r--r--
	}

	for _, perm := range permissions {
		// Create a temporary file with specific permissions
		tempFile, err := os.CreateTemp("", "test_perm_*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tempPath := tempFile.Name()
		tempFile.Close()

		err = os.Chmod(tempPath, perm)
		if err != nil {
			t.Fatalf("Failed to set permissions: %v", err)
		}

		// Delete using backend
		err = backend.DeleteFile(tempPath)
		if err != nil {
			t.Errorf("DeleteFile failed for permissions %o: %v", perm, err)
		}

		// Verify deleted
		if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
			t.Errorf("File with permissions %o still exists after deletion", perm)
		}
	}
}

// TestGenericBackend_DirectoryPermissions tests directory deletion with various permissions.
// Requirements: 4.1, 4.2, 8.3
func TestGenericBackend_DirectoryPermissions(t *testing.T) {
	backend := NewGenericBackend()

	permissions := []os.FileMode{
		0755, // rwxr-xr-x
		0700, // rwx------
		0555, // r-xr-xr-x
	}

	for _, perm := range permissions {
		// Create a temporary directory with specific permissions
		tempDir, err := os.MkdirTemp("", "test_dir_perm_*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}

		err = os.Chmod(tempDir, perm)
		if err != nil {
			t.Fatalf("Failed to set directory permissions: %v", err)
		}

		// Delete using backend
		err = backend.DeleteDirectory(tempDir)
		if err != nil {
			t.Errorf("DeleteDirectory failed for permissions %o: %v", perm, err)
		}

		// Verify deleted
		if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
			t.Errorf("Directory with permissions %o still exists after deletion", perm)
		}
	}
}

// TestGenericBackend_LargeFile tests deletion of a large file.
// Requirements: 8.3
func TestGenericBackend_LargeFile(t *testing.T) {
	backend := NewGenericBackend()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_large_*.dat")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()

	// Write 10MB of data
	data := make([]byte, 10*1024*1024)
	_, err = tempFile.Write(data)
	if err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		t.Fatalf("Failed to write large file: %v", err)
	}
	tempFile.Close()

	// Delete using backend
	err = backend.DeleteFile(tempPath)
	if err != nil {
		os.Remove(tempPath)
		t.Fatalf("DeleteFile failed for large file: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("Large file still exists after deletion")
	}
}

// TestGenericBackend_FileInUse tests deletion of a file that's currently open.
// Requirements: 4.1, 4.2, 8.3
func TestGenericBackend_FileInUse(t *testing.T) {
	backend := NewGenericBackend()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_inuse_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()
	defer tempFile.Close()
	defer os.Remove(tempPath)

	// Write some data
	_, err = tempFile.WriteString("test data")
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	// On Unix-like systems, we can delete files that are open
	// The file will be removed from the directory but the inode remains until closed
	err = backend.DeleteFile(tempPath)
	if err != nil {
		t.Fatalf("DeleteFile failed for open file: %v", err)
	}

	// On Unix, the file should appear deleted even though it's still open
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("File still appears in directory after deletion")
	}
}

// TestGenericBackend_DeepNestedStructure tests deletion in deeply nested directories.
// Requirements: 8.3
func TestGenericBackend_DeepNestedStructure(t *testing.T) {
	backend := NewGenericBackend()

	// Create a deeply nested directory structure
	tempDir, err := os.MkdirTemp("", "test_deep_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Build a deep path
	currentPath := tempDir
	var paths []string
	for i := 0; i < 20; i++ {
		currentPath = filepath.Join(currentPath, "nested")
		err = os.Mkdir(currentPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create nested directory: %v", err)
		}
		paths = append([]string{currentPath}, paths...) // Prepend for reverse order
	}

	// Create a file in the deepest directory
	testFile := filepath.Join(currentPath, "deep_file.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Delete the file
	err = backend.DeleteFile(testFile)
	if err != nil {
		t.Fatalf("DeleteFile failed in deep structure: %v", err)
	}

	// Delete directories from innermost to outermost
	for _, path := range paths {
		err = backend.DeleteDirectory(path)
		if err != nil {
			t.Errorf("DeleteDirectory failed for %s: %v", path, err)
		}
	}

	// Delete root
	err = backend.DeleteDirectory(tempDir)
	if err != nil {
		t.Errorf("DeleteDirectory failed for root: %v", err)
	}

	// Verify all deleted
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Directory structure still exists after deletion")
	}
}

// TestGenericBackend_SpecialCharactersInPath tests paths with special characters.
// Requirements: 8.3
func TestGenericBackend_SpecialCharactersInPath(t *testing.T) {
	backend := NewGenericBackend()

	// Create a directory for testing
	tempDir, err := os.MkdirTemp("", "test_special_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Unix allows most characters except / and null
	specialNames := []string{
		"file with spaces.txt",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
		"file.multiple.dots.txt",
		"file(with)parens.txt",
		"file[with]brackets.txt",
		"file{with}braces.txt",
		"file@with@at.txt",
		"file#with#hash.txt",
		"file$with$dollar.txt",
	}

	for _, name := range specialNames {
		testFile := filepath.Join(tempDir, name)
		err = os.WriteFile(testFile, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", name, err)
		}

		// Delete using backend
		err = backend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed for %s: %v", name, err)
		}

		// Verify deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Errorf("File %s still exists after deletion", name)
		}
	}
}

// TestGenericBackend_ErrorMessages tests that error messages are informative.
// Requirements: 4.1, 4.2, 8.3
func TestGenericBackend_ErrorMessages(t *testing.T) {
	backend := NewGenericBackend()

	// Try to delete non-existent file
	err := backend.DeleteFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}

	// Verify error message contains useful information
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to delete file") {
		t.Errorf("Error message should indicate deletion failure: %v", err)
	}

	// Try to delete non-existent directory
	err = backend.DeleteDirectory("/nonexistent/path/dir")
	if err == nil {
		t.Fatal("Expected error for non-existent directory")
	}

	// Verify error message contains useful information
	errMsg = err.Error()
	if !strings.Contains(errMsg, "failed to delete directory") {
		t.Errorf("Error message should indicate deletion failure: %v", err)
	}
}
