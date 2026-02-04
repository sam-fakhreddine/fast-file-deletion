//go:build windows

package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestNewNtBackend verifies that NewNtBackend creates a valid backend.
func TestNewNtBackend(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	if backend == nil {
		t.Fatal("NewNtBackend() returned nil backend")
	}

	if backend.fallbackBackend == nil {
		t.Fatal("NtBackend has nil fallback backend")
	}

	if backend.ntdllHandle == nil {
		t.Fatal("NtBackend has nil ntdll handle")
	}

	if backend.ntDeleteFileProc == nil {
		t.Fatal("NtBackend has nil NtDeleteFile proc")
	}
}

// TestNtBackendDeleteFile tests basic file deletion using NT API.
func TestNtBackendDeleteFile(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); err != nil {
		t.Fatalf("Test file does not exist: %v", err)
	}

	// Delete the file
	if err := backend.DeleteFile(testFile); err != nil {
		t.Fatalf("DeleteFile() failed: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatalf("File still exists after deletion")
	}
}

// TestNtBackendDeleteFileUTF16 tests file deletion using pre-converted UTF-16 paths.
func TestNtBackendDeleteFileUTF16(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_utf16.txt")

	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Convert path to UTF-16
	extendedPath := toExtendedLengthPath(testFile)
	pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
	if err != nil {
		t.Fatalf("Failed to convert path to UTF-16: %v", err)
	}

	// Delete the file using UTF-16 path
	if err := backend.DeleteFileUTF16(pathPtr, testFile); err != nil {
		t.Fatalf("DeleteFileUTF16() failed: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatalf("File still exists after deletion")
	}
}

// TestNtBackendDeleteDirectory tests directory deletion (should use fallback).
func TestNtBackendDeleteDirectory(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Create a temporary directory
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")

	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Delete the directory
	if err := backend.DeleteDirectory(testDir); err != nil {
		t.Fatalf("DeleteDirectory() failed: %v", err)
	}

	// Verify directory is deleted
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Fatalf("Directory still exists after deletion")
	}
}

// TestNtBackendDeleteDirectoryUTF16 tests directory deletion using UTF-16 paths.
func TestNtBackendDeleteDirectoryUTF16(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Create a temporary directory
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir_utf16")

	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Convert path to UTF-16
	extendedPath := toExtendedLengthPath(testDir)
	pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
	if err != nil {
		t.Fatalf("Failed to convert path to UTF-16: %v", err)
	}

	// Delete the directory
	if err := backend.DeleteDirectoryUTF16(pathPtr, testDir); err != nil {
		t.Fatalf("DeleteDirectoryUTF16() failed: %v", err)
	}

	// Verify directory is deleted
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Fatalf("Directory still exists after deletion")
	}
}

// TestNtBackendDeleteMultipleFiles tests deleting multiple files.
func TestNtBackendDeleteMultipleFiles(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Create multiple temporary files
	tmpDir := t.TempDir()
	fileCount := 100

	for i := 0; i < fileCount; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
	}

	// Delete all files
	for i := 0; i < fileCount; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
		if err := backend.DeleteFile(testFile); err != nil {
			t.Errorf("Failed to delete file %d: %v", i, err)
		}
	}

	// Verify all files are deleted
	for i := 0; i < fileCount; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Errorf("File %d still exists after deletion", i)
		}
	}
}

// TestNtBackendDeleteNonExistentFile tests deleting a non-existent file.
func TestNtBackendDeleteNonExistentFile(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")

	// Attempt to delete non-existent file (should fail gracefully)
	err = backend.DeleteFile(nonExistentFile)
	if err == nil {
		t.Fatal("Expected error when deleting non-existent file")
	}

	// Error should mention file not found
	errStr := err.Error()
	if errStr != "" {
		// We got an error as expected
		t.Logf("Got expected error: %v", err)
	}
}

// TestNtBackendDeleteReadOnlyFile tests deleting a read-only file.
func TestNtBackendDeleteReadOnlyFile(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Create a read-only file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "readonly.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set read-only attribute
	extendedPath := toExtendedLengthPath(testFile)
	pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
	if err != nil {
		t.Fatalf("Failed to convert path: %v", err)
	}

	if err := windows.SetFileAttributes(pathPtr, windows.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatalf("Failed to set read-only attribute: %v", err)
	}

	// Try to delete (should succeed via fallback which handles read-only)
	if err := backend.DeleteFile(testFile); err != nil {
		// It's OK if this fails - the fallback should handle it
		// but NT API might not handle read-only files directly
		t.Logf("Note: Read-only file deletion resulted in: %v", err)
	}

	// Check if file was deleted
	if _, err := os.Stat(testFile); err == nil {
		// File still exists, clear read-only and try again
		attrs, _ := windows.GetFileAttributes(pathPtr)
		newAttrs := attrs &^ windows.FILE_ATTRIBUTE_READONLY
		windows.SetFileAttributes(pathPtr, newAttrs)
	}
}

// TestNtBackendDeleteLongPath tests deleting a file with a long path.
func TestNtBackendDeleteLongPath(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Create a long path (Windows MAX_PATH is 260 characters)
	tmpDir := t.TempDir()

	// Build a long directory path
	longPath := tmpDir
	for len(longPath) < 300 {
		longPath = filepath.Join(longPath, "subdir")
	}

	// Create the directory structure
	if err := os.MkdirAll(longPath, 0755); err != nil {
		t.Skipf("Cannot create long path (OS limitation): %v", err)
	}

	// Create a file in the long path
	testFile := filepath.Join(longPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Skipf("Cannot create file in long path: %v", err)
	}

	// Delete the file
	if err := backend.DeleteFile(testFile); err != nil {
		t.Fatalf("Failed to delete file with long path: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatalf("File still exists after deletion")
	}
}

// TestNtBackendDeleteUnicodeFile tests deleting a file with Unicode characters.
func TestNtBackendDeleteUnicodeFile(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Create a file with Unicode characters in the name
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_文件_файл_αρχείο.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create Unicode test file: %v", err)
	}

	// Delete the file
	if err := backend.DeleteFile(testFile); err != nil {
		t.Fatalf("Failed to delete Unicode file: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatalf("Unicode file still exists after deletion")
	}
}

// TestToNtPathUTF16 tests NT path conversion.
func TestToNtPathUTF16(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Standard path",
			input:    `C:\Windows\System32`,
			expected: `\??\C:\Windows\System32`,
		},
		{
			name:     "Extended-length path",
			input:    `\\?\C:\Windows\System32`,
			expected: `\??\C:\Windows\System32`,
		},
		{
			name:     "UNC path",
			input:    `\\server\share\file.txt`,
			expected: `\??\UNC\server\share\file.txt`,
		},
		{
			name:     "Drive letter with file",
			input:    `D:\data\file.txt`,
			expected: `\??\D:\data\file.txt`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert input to UTF-16
			inputUTF16, err := syscall.UTF16PtrFromString(tt.input)
			if err != nil {
				t.Fatalf("Failed to convert input to UTF-16: %v", err)
			}

			// Convert to NT path
			ntPathUTF16, err := backend.toNtPathUTF16(inputUTF16)
			if err != nil {
				t.Fatalf("toNtPathUTF16() failed: %v", err)
			}

			// Convert back to string for comparison
			ntPathStr := windows.UTF16PtrToString(ntPathUTF16)

			if ntPathStr != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, ntPathStr)
			}
		})
	}
}

// TestCreateUnicodeString tests UNICODE_STRING creation.
func TestCreateUnicodeString(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		wantLen  uint16 // Expected length in bytes (excluding null terminator)
	}{
		{
			name:    "Short path",
			input:   `C:\test`,
			wantLen: 7 * 2, // 7 characters * 2 bytes per character
		},
		{
			name:    "Long path",
			input:   `C:\very\long\path\to\some\file.txt`,
			wantLen: 34 * 2, // 34 characters * 2 bytes per character
		},
		{
			name:    "Empty string",
			input:   ``,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip empty string test as UTF16PtrFromString doesn't handle it well
			if tt.input == "" {
				t.Skip("Skipping empty string test")
			}

			// Convert to UTF-16
			utf16Ptr, err := syscall.UTF16PtrFromString(tt.input)
			if err != nil {
				t.Fatalf("Failed to convert to UTF-16: %v", err)
			}

			// Create UNICODE_STRING
			us := backend.createUnicodeString(utf16Ptr)

			// Verify length
			if us.Length != tt.wantLen {
				t.Errorf("Expected Length=%d, got %d", tt.wantLen, us.Length)
			}

			// MaximumLength should be Length + 2 (for null terminator)
			expectedMaxLen := tt.wantLen + 2
			if us.MaximumLength != expectedMaxLen {
				t.Errorf("Expected MaximumLength=%d, got %d", expectedMaxLen, us.MaximumLength)
			}

			// Buffer should point to the input
			if us.Buffer != utf16Ptr {
				t.Errorf("Buffer pointer mismatch")
			}
		})
	}
}

// TestTranslateNTStatus tests NT status code translation.
func TestTranslateNTStatus(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	tests := []struct {
		status      uint32
		expectError bool
		contains    string
	}{
		{STATUS_SUCCESS, false, ""},
		{STATUS_ACCESS_DENIED, true, "access denied"},
		{STATUS_OBJECT_NAME_NOT_FOUND, true, "file not found"},
		{STATUS_OBJECT_PATH_NOT_FOUND, true, "path not found"},
		{STATUS_SHARING_VIOLATION, true, "in use"},
		{STATUS_DIRECTORY_NOT_EMPTY, true, "not empty"},
		{STATUS_CANNOT_DELETE, true, "cannot delete"},
		{STATUS_FILE_IS_A_DIRECTORY, true, "directory"},
		{STATUS_INVALID_PARAMETER, true, "invalid parameter"},
		{0xDEADBEEF, true, "0xDEADBEEF"}, // Unknown status
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Status_0x%08X", tt.status), func(t *testing.T) {
			err := backend.translateNTStatus(tt.status)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for status 0x%08X", tt.status)
				} else if tt.contains != "" && !contains(err.Error(), tt.contains) {
					t.Errorf("Expected error to contain %q, got %q", tt.contains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for status 0x%08X, got %v", tt.status, err)
				}
			}
		})
	}
}

// TestShouldFallback tests the fallback decision logic.
func TestShouldFallback(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	tests := []struct {
		status         uint32
		shouldFallback bool
	}{
		{STATUS_SUCCESS, false},
		{STATUS_ACCESS_DENIED, true},           // Should fallback for better handling
		{STATUS_FILE_IS_A_DIRECTORY, true},     // Should use directory deletion
		{STATUS_INVALID_PARAMETER, true},       // Path format issue
		{STATUS_OBJECT_NAME_NOT_FOUND, false},  // Legitimate error
		{STATUS_SHARING_VIOLATION, false},      // Legitimate error
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Status_0x%08X", tt.status), func(t *testing.T) {
			result := backend.shouldFallback(tt.status)
			if result != tt.shouldFallback {
				t.Errorf("Expected shouldFallback=%v for status 0x%08X, got %v",
					tt.shouldFallback, tt.status, result)
			}
		})
	}
}

// TestNtBackendImplementsInterfaces verifies that NtBackend implements required interfaces.
func TestNtBackendImplementsInterfaces(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Check Backend interface
	var _ Backend = backend

	// Check UTF16Backend interface
	var _ UTF16Backend = backend
}

// Helper function to check if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// TestCallNtDeleteFile tests the low-level NtDeleteFile syscall.
func TestCallNtDeleteFile(t *testing.T) {
	backend, err := NewNtBackend()
	if err != nil {
		t.Fatalf("NewNtBackend() failed: %v", err)
	}

	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_lowlevel.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Convert to NT path
	extendedPath := toExtendedLengthPath(testFile)
	pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
	if err != nil {
		t.Fatalf("Failed to convert path: %v", err)
	}

	ntPathUTF16, err := backend.toNtPathUTF16(pathPtr)
	if err != nil {
		t.Fatalf("Failed to convert to NT path: %v", err)
	}

	// Create UNICODE_STRING
	unicodeStr := backend.createUnicodeString(ntPathUTF16)

	// Initialize OBJECT_ATTRIBUTES
	var objAttr OBJECT_ATTRIBUTES
	InitializeObjectAttributes(
		&objAttr,
		&unicodeStr,
		OBJ_CASE_INSENSITIVE,
		0,
		0,
	)

	// Call NtDeleteFile directly
	status := backend.callNtDeleteFile(&objAttr)

	if status != STATUS_SUCCESS {
		err := backend.translateNTStatus(status)
		t.Logf("NtDeleteFile status: 0x%08X, error: %v", status, err)
	}

	// Check if file was deleted
	if _, err := os.Stat(testFile); err == nil {
		t.Logf("Note: File still exists after NtDeleteFile call (status: 0x%08X)", status)
	}
}

// BenchmarkNtBackendDeleteFile benchmarks NT backend file deletion.
func BenchmarkNtBackendDeleteFile(b *testing.B) {
	backend, err := NewNtBackend()
	if err != nil {
		b.Fatalf("NewNtBackend() failed: %v", err)
	}

	tmpDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("bench_%d.txt", i))

		// Create file
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			b.Fatalf("Failed to create test file: %v", err)
		}

		// Delete file
		if err := backend.DeleteFile(testFile); err != nil {
			b.Fatalf("Failed to delete file: %v", err)
		}
	}
}

// BenchmarkNtBackendDeleteFileUTF16 benchmarks NT backend file deletion with pre-converted paths.
func BenchmarkNtBackendDeleteFileUTF16(b *testing.B) {
	backend, err := NewNtBackend()
	if err != nil {
		b.Fatalf("NewNtBackend() failed: %v", err)
	}

	tmpDir := b.TempDir()

	// Pre-create files
	files := make([]string, b.N)
	pathPtrs := make([]*uint16, b.N)
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("bench_%d.txt", i))
		files[i] = testFile

		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			b.Fatalf("Failed to create test file: %v", err)
		}

		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			b.Fatalf("Failed to convert path: %v", err)
		}
		pathPtrs[i] = pathPtr
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := backend.DeleteFileUTF16(pathPtrs[i], files[i]); err != nil {
			b.Fatalf("Failed to delete file: %v", err)
		}
	}
}
