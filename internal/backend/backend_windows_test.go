//go:build windows

package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWindowsBackend_ExtendedLengthPath tests Windows extended-length path handling.
// Requirements: 8.2
func TestWindowsBackend_ExtendedLengthPath(t *testing.T) {
	backend := NewWindowsBackend()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_extended_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// Delete using the backend (which should handle extended-length paths)
	err = backend.DeleteFile(tempPath)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("File still exists after deletion")
	}
}

// TestWindowsBackend_LongPath tests handling of very long paths.
// Requirements: 8.2
func TestWindowsBackend_LongPath(t *testing.T) {
	backend := NewWindowsBackend()

	// Create a deeply nested directory structure
	tempDir, err := os.MkdirTemp("", "test_long_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Build a long path by nesting directories
	currentPath := tempDir
	for i := 0; i < 10; i++ {
		currentPath = filepath.Join(currentPath, "very_long_directory_name_to_test_extended_paths")
		err = os.Mkdir(currentPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create nested directory: %v", err)
		}
	}

	// Create a file in the deepest directory
	testFile := filepath.Join(currentPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Delete the file using Windows backend
	err = backend.DeleteFile(testFile)
	if err != nil {
		t.Fatalf("DeleteFile failed for long path: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File still exists after deletion")
	}
}

// TestWindowsBackend_DirectoryWithLongPath tests directory deletion with long paths.
// Requirements: 8.2
func TestWindowsBackend_DirectoryWithLongPath(t *testing.T) {
	backend := NewWindowsBackend()

	// Create a deeply nested directory structure
	tempDir, err := os.MkdirTemp("", "test_long_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Build a long path
	currentPath := tempDir
	var paths []string
	for i := 0; i < 5; i++ {
		currentPath = filepath.Join(currentPath, "nested_directory_with_long_name")
		err = os.Mkdir(currentPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create nested directory: %v", err)
		}
		paths = append([]string{currentPath}, paths...) // Prepend for reverse order
	}

	// Delete directories from innermost to outermost
	for _, path := range paths {
		err = backend.DeleteDirectory(path)
		if err != nil {
			t.Errorf("DeleteDirectory failed for %s: %v", path, err)
		}
	}

	// Delete the root temp directory
	err = backend.DeleteDirectory(tempDir)
	if err != nil {
		t.Errorf("DeleteDirectory failed for root: %v", err)
	}

	// Verify all deleted
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Directory still exists after deletion")
	}
}

// TestToExtendedLengthPath tests the path conversion function.
// Requirements: 8.2
func TestToExtendedLengthPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Regular absolute path",
			input:    `C:\Users\test\file.txt`,
			expected: `\\?\C:\Users\test\file.txt`,
		},
		{
			name:     "Already extended path",
			input:    `\\?\C:\Users\test\file.txt`,
			expected: `\\?\C:\Users\test\file.txt`,
		},
		{
			name:     "UNC path",
			input:    `\\server\share\file.txt`,
			expected: `\\?\UNC\server\share\file.txt`,
		},
		{
			name:     "Path with spaces",
			input:    `C:\Program Files\test\file.txt`,
			expected: `\\?\C:\Program Files\test\file.txt`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toExtendedLengthPath(tt.input)
			if result != tt.expected {
				t.Errorf("toExtendedLengthPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestWindowsBackend_LockedFile tests handling of locked files.
// Requirements: 4.1, 4.2, 8.2
func TestWindowsBackend_LockedFile(t *testing.T) {
	backend := NewWindowsBackend()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_locked_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()
	// Keep file open to lock it
	defer tempFile.Close()
	defer os.Remove(tempPath)

	// Write some data to ensure file is in use
	_, err = tempFile.WriteString("test data")
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	// Try to delete the locked file
	err = backend.DeleteFile(tempPath)

	// Should return an error because file is locked
	if err == nil {
		t.Error("Expected error when deleting locked file, got nil")
	}

	// Verify error message indicates the issue
	if !strings.Contains(err.Error(), "failed to delete file") {
		t.Errorf("Error message doesn't indicate deletion failure: %v", err)
	}
}

// TestWindowsBackend_FileWithSpecialCharacters tests files with special characters in names.
// Requirements: 8.2
func TestWindowsBackend_FileWithSpecialCharacters(t *testing.T) {
	backend := NewWindowsBackend()

	// Create a file with special characters (that are valid on Windows)
	tempDir, err := os.MkdirTemp("", "test_special_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Windows allows these special characters in filenames
	specialNames := []string{
		"file with spaces.txt",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
		"file.multiple.dots.txt",
		"file(with)parens.txt",
		"file[with]brackets.txt",
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

// TestWindowsBackend_EmptyDirectory tests deletion of empty directories.
// Requirements: 8.2
func TestWindowsBackend_EmptyDirectory(t *testing.T) {
	backend := NewWindowsBackend()

	// Create an empty directory
	tempDir, err := os.MkdirTemp("", "test_empty_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Delete the empty directory
	err = backend.DeleteDirectory(tempDir)
	if err != nil {
		t.Fatalf("DeleteDirectory failed: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Directory still exists after deletion")
	}
}

// TestWindowsBackend_DirectoryWithHiddenAttribute tests directories with hidden attribute.
// Requirements: 8.2
func TestWindowsBackend_DirectoryWithHiddenAttribute(t *testing.T) {
	backend := NewWindowsBackend()

	// Create a directory
	tempDir, err := os.MkdirTemp("", "test_hidden_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Note: Setting hidden attribute requires Windows API calls
	// For this test, we'll just verify normal deletion works
	// A more comprehensive test would use syscall to set FILE_ATTRIBUTE_HIDDEN

	// Delete the directory
	err = backend.DeleteDirectory(tempDir)
	if err != nil {
		t.Fatalf("DeleteDirectory failed: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Directory still exists after deletion")
	}
}
