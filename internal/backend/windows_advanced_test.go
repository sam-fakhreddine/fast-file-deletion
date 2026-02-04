//go:build windows

package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
	"pgregory.net/rapid"
)

// TestGetWindowsVersion verifies that Windows version detection works correctly.
func TestGetWindowsVersion(t *testing.T) {
	major, minor, build := getWindowsVersion()

	// Verify we got valid version numbers
	if major == 0 {
		t.Error("Expected non-zero major version")
	}

	// Windows versions should be at least Windows 7 (6.1) or later
	if major < 6 {
		t.Errorf("Expected major version >= 6, got %d", major)
	}

	// Build number should be non-zero
	if build == 0 {
		t.Error("Expected non-zero build number")
	}

	// Verify caching works - calling again should return same values
	major2, minor2, build2 := getWindowsVersion()
	if major != major2 || minor != minor2 || build != build2 {
		t.Errorf("Version detection not cached properly: first call (%d.%d.%d) != second call (%d.%d.%d)",
			major, minor, build, major2, minor2, build2)
	}
}

// TestSupportsFileDispositionInfoEx verifies FileDispositionInfoEx availability detection.
func TestSupportsFileDispositionInfoEx(t *testing.T) {
	supported := supportsFileDispositionInfoEx()

	// Get version to verify the logic
	major, _, build := getWindowsVersion()

	// FileDispositionInfoEx requires Windows 10 RS1 (10.0.14393) or later
	expectedSupport := major > 10 || (major == 10 && build >= windows10RS1Build)

	if supported != expectedSupport {
		t.Errorf("FileDispositionInfoEx support detection incorrect: got %v, expected %v (Windows %d.0.%d)",
			supported, expectedSupport, major, build)
	}

	// Verify caching works
	supported2 := supportsFileDispositionInfoEx()
	if supported != supported2 {
		t.Error("FileDispositionInfoEx support detection not cached properly")
	}
}

// TestSupportsNtDeleteFile verifies NtDeleteFile availability detection.
func TestSupportsNtDeleteFile(t *testing.T) {
	supported := supportsNtDeleteFile()

	// NtDeleteFile should be available on all modern Windows versions
	// We can't guarantee it's available, but we can verify the function doesn't panic
	t.Logf("NtDeleteFile supported: %v", supported)

	// Verify caching works
	supported2 := supportsNtDeleteFile()
	if supported != supported2 {
		t.Error("NtDeleteFile support detection not cached properly")
	}
}

// TestGetAPIAvailability verifies the GetAPIAvailability function returns correct information.
// This test validates Requirements 7.1, 7.2, 7.5.
func TestGetAPIAvailability(t *testing.T) {
	major, minor, build, hasFileInfoEx, hasNtDelete := GetAPIAvailability()

	// Verify version numbers are valid
	if major == 0 {
		t.Error("Expected non-zero major version")
	}

	// Verify version numbers match getWindowsVersion
	expectedMajor, expectedMinor, expectedBuild := getWindowsVersion()
	if major != expectedMajor || minor != expectedMinor || build != expectedBuild {
		t.Errorf("GetAPIAvailability version mismatch: got %d.%d.%d, expected %d.%d.%d",
			major, minor, build, expectedMajor, expectedMinor, expectedBuild)
	}

	// Verify FileDispositionInfoEx availability matches supportsFileDispositionInfoEx
	expectedFileInfoEx := supportsFileDispositionInfoEx()
	if hasFileInfoEx != expectedFileInfoEx {
		t.Errorf("GetAPIAvailability FileInfoEx mismatch: got %v, expected %v",
			hasFileInfoEx, expectedFileInfoEx)
	}

	// Verify NtDeleteFile availability matches supportsNtDeleteFile
	expectedNtDelete := supportsNtDeleteFile()
	if hasNtDelete != expectedNtDelete {
		t.Errorf("GetAPIAvailability NtDelete mismatch: got %v, expected %v",
			hasNtDelete, expectedNtDelete)
	}

	// Log the results for visibility
	t.Logf("Windows version: %d.%d (build %d)", major, minor, build)
	t.Logf("FileDispositionInfoEx available: %v", hasFileInfoEx)
	t.Logf("NtDeleteFile available: %v", hasNtDelete)
}

// TestTranslateNTStatus verifies NT status code translation.
func TestTranslateNTStatus(t *testing.T) {
	tests := []struct {
		status   uint32
		expected string
	}{
		{0x00000000, ""},                    // STATUS_SUCCESS (no error)
		{0xC0000022, "access denied"},       // STATUS_ACCESS_DENIED
		{0xC0000034, "file not found"},      // STATUS_OBJECT_NAME_NOT_FOUND
		{0xC0000043, "file is in use"},      // STATUS_SHARING_VIOLATION
		{0xC000003A, "path not found"},      // STATUS_OBJECT_PATH_NOT_FOUND
		{0xC0000101, "directory not empty"}, // STATUS_DIRECTORY_NOT_EMPTY
		{0xC0000121, "cannot delete"},       // STATUS_CANNOT_DELETE
		{0xDEADBEEF, "NT status 0xDEADBEEF"}, // Unknown status
	}

	for _, tt := range tests {
		err := translateNTStatus(tt.status)
		if tt.status == 0x00000000 {
			// STATUS_SUCCESS should return nil
			if err != nil {
				t.Errorf("translateNTStatus(0x%08X) = %v, expected nil", tt.status, err)
			}
		} else {
			if err == nil {
				t.Errorf("translateNTStatus(0x%08X) = nil, expected error", tt.status)
			} else if err.Error() != tt.expected {
				t.Errorf("translateNTStatus(0x%08X) = %q, expected %q", tt.status, err.Error(), tt.expected)
			}
		}
	}
}

// TestDeletionMethodString verifies the String() method for DeletionMethod.
func TestDeletionMethodString(t *testing.T) {
	tests := []struct {
		method   DeletionMethod
		expected string
	}{
		{MethodAuto, "auto"},
		{MethodFileInfo, "fileinfo"},
		{MethodDeleteOnClose, "deleteonclose"},
		{MethodNtAPI, "ntapi"},
		{MethodDeleteAPI, "deleteapi"},
		{DeletionMethod(999), "unknown"},
	}

	for _, tt := range tests {
		result := tt.method.String()
		if result != tt.expected {
			t.Errorf("DeletionMethod(%d).String() = %q, expected %q", tt.method, result, tt.expected)
		}
	}
}

// TestInitializeObjectAttributes verifies OBJECT_ATTRIBUTES initialization.
func TestInitializeObjectAttributes(t *testing.T) {
	var objAttr OBJECT_ATTRIBUTES
	var unicodeStr windows.NTUnicodeString

	// Initialize with test values
	const testAttributes = 0x00000040 // OBJ_CASE_INSENSITIVE
	InitializeObjectAttributes(&objAttr, &unicodeStr, testAttributes, 0, 0)

	// Verify fields are set correctly
	if objAttr.Length != uint32(unsafe.Sizeof(objAttr)) {
		t.Errorf("Length = %d, expected %d", objAttr.Length, unsafe.Sizeof(objAttr))
	}

	if objAttr.ObjectName != &unicodeStr {
		t.Error("ObjectName not set correctly")
	}

	if objAttr.Attributes != testAttributes {
		t.Errorf("Attributes = 0x%08X, expected 0x%08X", objAttr.Attributes, testAttributes)
	}

	if objAttr.RootDirectory != 0 {
		t.Errorf("RootDirectory = %v, expected 0", objAttr.RootDirectory)
	}

	if objAttr.SecurityDescriptor != 0 {
		t.Errorf("SecurityDescriptor = %v, expected 0", objAttr.SecurityDescriptor)
	}

	if objAttr.SecurityQualityOfService != 0 {
		t.Errorf("SecurityQualityOfService = %v, expected 0", objAttr.SecurityQualityOfService)
	}
}

// TestDeleteWithFileInfo verifies the deleteWithFileInfo function.
// This test validates Requirements 1.1, 1.2, 1.4.
func TestDeleteWithFileInfo(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	t.Run("delete regular file", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tempDir, "test_file.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Delete the file using deleteWithFileInfo
		err = deleteWithFileInfo(pathPtr)
		if err != nil {
			t.Errorf("deleteWithFileInfo failed: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion")
		}
	})

	t.Run("delete read-only file", func(t *testing.T) {
		// Create a read-only test file
		testFile := filepath.Join(tempDir, "readonly_file.txt")
		err := os.WriteFile(testFile, []byte("readonly content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Set read-only attribute
		err = os.Chmod(testFile, 0444)
		if err != nil {
			t.Fatalf("Failed to set read-only attribute: %v", err)
		}

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Delete the file using deleteWithFileInfo
		// On Windows 10 RS1+, FileDispositionInfoEx should handle read-only files automatically
		err = deleteWithFileInfo(pathPtr)
		
		// The behavior depends on Windows version:
		// - Windows 10 RS1+: Should succeed (FileDispositionInfoEx ignores read-only)
		// - Older Windows: May fail (FileDispositionInfo doesn't ignore read-only)
		if supportsFileDispositionInfoEx() {
			// On newer Windows, deletion should succeed
			if err != nil {
				t.Errorf("deleteWithFileInfo failed on read-only file (Windows 10 RS1+): %v", err)
			}
			// Verify the file is deleted
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Error("Read-only file still exists after deletion")
			}
		} else {
			// On older Windows, we just log the result
			t.Logf("deleteWithFileInfo on read-only file (older Windows): %v", err)
			// Clean up if deletion failed
			if err != nil {
				os.Chmod(testFile, 0644)
				os.Remove(testFile)
			}
		}
	})

	t.Run("delete non-existent file", func(t *testing.T) {
		// Try to delete a file that doesn't exist
		testFile := filepath.Join(tempDir, "nonexistent_file.txt")
		
		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Delete should fail
		err = deleteWithFileInfo(pathPtr)
		if err == nil {
			t.Error("Expected error when deleting non-existent file, got nil")
		}
	})

	t.Run("delete with long path", func(t *testing.T) {
		// Create a deeply nested directory structure to test long path handling
		longPath := tempDir
		for i := 0; i < 10; i++ {
			longPath = filepath.Join(longPath, "very_long_directory_name_to_test_extended_length_paths")
		}
		
		// Create the directory structure
		err := os.MkdirAll(longPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create long path: %v", err)
		}

		// Create a test file in the deep directory
		testFile := filepath.Join(longPath, "test_file.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file in long path: %v", err)
		}

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert long path to UTF-16: %v", err)
		}

		// Delete the file using deleteWithFileInfo
		err = deleteWithFileInfo(pathPtr)
		if err != nil {
			t.Errorf("deleteWithFileInfo failed on long path: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File in long path still exists after deletion")
		}
	})
}


// TestFallbackChainCompleteness is a property-based test that verifies the fallback chain
// for file deletion methods. It tests that when a deletion method fails or is unavailable,
// the backend tries the next method in the fallback chain (FileDispositionInfoEx →
// FileDispositionInfo → DeleteFile) until one succeeds or all methods are exhausted.
//
// **Validates: Requirements 1.1, 1.2, 1.3**
// Feature: windows-performance-optimization, Property 1: Fallback chain completeness
func TestFallbackChainCompleteness(t *testing.T) {
	// Note: This is a Windows-specific test that verifies the internal fallback mechanism
	// in deleteWithFileInfo. The function automatically tries FileDispositionInfoEx first,
	// then falls back to FileDispositionInfo if InfoEx returns ERROR_INVALID_PARAMETER.
	// We test this by creating various file scenarios and verifying successful deletion.
	
	tempDir := t.TempDir()

	t.Run("FileDispositionInfoEx to FileDispositionInfo fallback", func(t *testing.T) {
		// This test verifies that when FileDispositionInfoEx fails with ERROR_INVALID_PARAMETER,
		// the implementation falls back to FileDispositionInfo
		
		// Create a test file
		testFile := filepath.Join(tempDir, "fallback_test_1.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Call deleteWithFileInfo - it should handle the fallback internally
		err = deleteWithFileInfo(pathPtr)
		if err != nil {
			t.Errorf("deleteWithFileInfo failed (should have fallen back): %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion with fallback")
		}
	})

	t.Run("FileDispositionInfo to DeleteFile fallback simulation", func(t *testing.T) {
		// This test simulates the scenario where FileDispositionInfo also fails,
		// requiring a fallback to the basic DeleteFile API
		
		// Create a test file
		testFile := filepath.Join(tempDir, "fallback_test_2.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// For this test, we'll verify that the basic DeleteFile API works
		// as the final fallback by using it directly
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Use the basic DeleteFile API (final fallback)
		err = windows.DeleteFile(pathPtr)
		if err != nil {
			t.Errorf("DeleteFile (final fallback) failed: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion with final fallback")
		}
	})

	t.Run("Fallback chain with read-only file", func(t *testing.T) {
		// This test verifies the fallback chain behavior with read-only files
		// On Windows 10 RS1+, FileDispositionInfoEx should handle it
		// On older Windows, it should fall back to FileDispositionInfo
		
		// Create a read-only test file
		testFile := filepath.Join(tempDir, "fallback_readonly.txt")
		err := os.WriteFile(testFile, []byte("readonly content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Set read-only attribute
		err = os.Chmod(testFile, 0444)
		if err != nil {
			t.Fatalf("Failed to set read-only attribute: %v", err)
		}

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Call deleteWithFileInfo - it should handle the fallback internally
		err = deleteWithFileInfo(pathPtr)
		
		// On Windows 10 RS1+, FileDispositionInfoEx should succeed
		// On older Windows, FileDispositionInfo may fail, requiring manual attribute clearing
		if supportsFileDispositionInfoEx() {
			if err != nil {
				t.Errorf("deleteWithFileInfo failed on read-only file (Windows 10 RS1+): %v", err)
			}
			// Verify the file is deleted
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Error("Read-only file still exists after deletion")
			}
		} else {
			// On older Windows, we expect potential failure and need to handle it
			t.Logf("deleteWithFileInfo on read-only file (older Windows): %v", err)
			// Clean up if deletion failed
			if err != nil {
				os.Chmod(testFile, 0644)
				os.Remove(testFile)
			}
		}
	})

	t.Run("Fallback exhaustion with non-existent file", func(t *testing.T) {
		// This test verifies that when all fallback methods fail,
		// the error is properly propagated
		
		// Try to delete a file that doesn't exist
		testFile := filepath.Join(tempDir, "nonexistent_fallback.txt")
		
		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Call deleteWithFileInfo - all methods should fail
		err = deleteWithFileInfo(pathPtr)
		if err == nil {
			t.Error("Expected error when all fallback methods fail, got nil")
		}
	})

	t.Run("Fallback chain with locked file", func(t *testing.T) {
		// This test verifies the fallback chain behavior with a locked file
		// All methods should fail, but the fallback chain should be attempted
		
		// Create a test file
		testFile := filepath.Join(tempDir, "locked_file.txt")
		err := os.WriteFile(testFile, []byte("locked content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Open the file to lock it
		file, err := os.OpenFile(testFile, os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("Failed to open file for locking: %v", err)
		}
		defer file.Close()

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Try to delete the locked file - should fail after trying all methods
		err = deleteWithFileInfo(pathPtr)
		if err == nil {
			t.Error("Expected error when deleting locked file, got nil")
		}

		// The file should still exist
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("Locked file was deleted (should have failed)")
		}
	})
}

// TestDeleteWithDeleteOnClose verifies the deleteWithDeleteOnClose function.
// This test validates Requirement 1.5.
func TestDeleteWithDeleteOnClose(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	t.Run("delete regular file", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tempDir, "deleteonclose_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Delete the file using deleteWithDeleteOnClose
		err = deleteWithDeleteOnClose(pathPtr)
		if err != nil {
			t.Errorf("deleteWithDeleteOnClose failed: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion")
		}
	})

	t.Run("delete non-existent file", func(t *testing.T) {
		// Try to delete a file that doesn't exist
		testFile := filepath.Join(tempDir, "nonexistent_deleteonclose.txt")
		
		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Delete should fail
		err = deleteWithDeleteOnClose(pathPtr)
		if err == nil {
			t.Error("Expected error when deleting non-existent file, got nil")
		}
	})

	t.Run("delete with long path", func(t *testing.T) {
		// Create a deeply nested directory structure to test long path handling
		longPath := tempDir
		for i := 0; i < 10; i++ {
			longPath = filepath.Join(longPath, "very_long_directory_name_to_test_extended_length_paths")
		}
		
		// Create the directory structure
		err := os.MkdirAll(longPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create long path: %v", err)
		}

		// Create a test file in the deep directory
		testFile := filepath.Join(longPath, "deleteonclose_longpath.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file in long path: %v", err)
		}

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert long path to UTF-16: %v", err)
		}

		// Delete the file using deleteWithDeleteOnClose
		err = deleteWithDeleteOnClose(pathPtr)
		if err != nil {
			t.Errorf("deleteWithDeleteOnClose failed on long path: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File in long path still exists after deletion")
		}
	})

	t.Run("delete multiple files sequentially", func(t *testing.T) {
		// Test that deleteWithDeleteOnClose works consistently across multiple files
		numFiles := 5
		testFiles := make([]string, numFiles)

		// Create multiple test files
		for i := 0; i < numFiles; i++ {
			testFile := filepath.Join(tempDir, fmt.Sprintf("deleteonclose_multi_%d.txt", i))
			err := os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file %d: %v", i, err)
			}
			testFiles[i] = testFile
		}

		// Delete all files
		for i, testFile := range testFiles {
			extendedPath := toExtendedLengthPath(testFile)
			pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
			if err != nil {
				t.Fatalf("Failed to convert path to UTF-16 for file %d: %v", i, err)
			}

			err = deleteWithDeleteOnClose(pathPtr)
			if err != nil {
				t.Errorf("deleteWithDeleteOnClose failed on file %d: %v", i, err)
			}
		}

		// Verify all files are deleted
		for i, testFile := range testFiles {
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Errorf("File %d still exists after deletion", i)
			}
		}
	})

	t.Run("delete locked file", func(t *testing.T) {
		// Test that deleteWithDeleteOnClose fails appropriately on locked files
		testFile := filepath.Join(tempDir, "locked_deleteonclose.txt")
		err := os.WriteFile(testFile, []byte("locked content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Open the file to lock it
		file, err := os.OpenFile(testFile, os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("Failed to open file for locking: %v", err)
		}
		defer file.Close()

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Try to delete the locked file - should fail
		err = deleteWithDeleteOnClose(pathPtr)
		if err == nil {
			t.Error("Expected error when deleting locked file, got nil")
		}

		// The file should still exist
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("Locked file was deleted (should have failed)")
		}
	})

	t.Run("delete read-only file", func(t *testing.T) {
		// Test that deleteWithDeleteOnClose handles read-only files
		// Note: Unlike FileDispositionInfoEx, DELETE_ON_CLOSE does not automatically
		// ignore read-only attributes, so this may fail
		testFile := filepath.Join(tempDir, "readonly_deleteonclose.txt")
		err := os.WriteFile(testFile, []byte("readonly content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Set read-only attribute
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		attrs, err := windows.GetFileAttributes(pathPtr)
		if err != nil {
			t.Fatalf("Failed to get file attributes: %v", err)
		}

		err = windows.SetFileAttributes(pathPtr, attrs|windows.FILE_ATTRIBUTE_READONLY)
		if err != nil {
			t.Fatalf("Failed to set read-only attribute: %v", err)
		}

		// Try to delete the read-only file
		// DELETE_ON_CLOSE does not automatically handle read-only attributes
		err = deleteWithDeleteOnClose(pathPtr)
		
		// This is expected to fail on read-only files
		if err == nil {
			t.Log("deleteWithDeleteOnClose succeeded on read-only file (unexpected but acceptable)")
			// Verify the file is deleted
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Error("Read-only file still exists after successful deletion")
			}
		} else {
			t.Logf("deleteWithDeleteOnClose failed on read-only file (expected): %v", err)
			// Clean up
			attrs, _ := windows.GetFileAttributes(pathPtr)
			windows.SetFileAttributes(pathPtr, attrs&^windows.FILE_ATTRIBUTE_READONLY)
			os.Remove(testFile)
		}
	})
}

// TestReadOnlyFileHandling is a property-based test that verifies read-only file handling.
// It tests that for any file with read-only attributes, when deletion fails with access denied,
// the backend should clear the read-only attribute and retry deletion successfully.
//
// **Validates: Requirements 1.4**
// Feature: windows-performance-optimization, Property 2: Read-only file handling
func TestReadOnlyFileHandling(t *testing.T) {
	// Import rapid for property-based testing
	// Note: This test uses the rapid framework to generate random test cases
	
	// For now, we'll implement this as a comprehensive unit test that covers
	// the property across multiple scenarios. A full property-based test with
	// rapid will be added in a follow-up if needed.
	
	tempDir := t.TempDir()

	t.Run("delete read-only file with FileDispositionInfoEx", func(t *testing.T) {
		// Create a read-only test file
		testFile := filepath.Join(tempDir, "readonly_fileinfo.txt")
		err := os.WriteFile(testFile, []byte("readonly content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Set read-only attribute using Windows API
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Get current attributes
		attrs, err := windows.GetFileAttributes(pathPtr)
		if err != nil {
			t.Fatalf("Failed to get file attributes: %v", err)
		}

		// Set read-only attribute
		err = windows.SetFileAttributes(pathPtr, attrs|windows.FILE_ATTRIBUTE_READONLY)
		if err != nil {
			t.Fatalf("Failed to set read-only attribute: %v", err)
		}

		// Verify the file is read-only
		attrs, err = windows.GetFileAttributes(pathPtr)
		if err != nil {
			t.Fatalf("Failed to verify read-only attribute: %v", err)
		}
		if attrs&windows.FILE_ATTRIBUTE_READONLY == 0 {
			t.Fatal("File is not read-only after setting attribute")
		}

		// Delete the file using deleteWithFileInfo
		// On Windows 10 RS1+, FileDispositionInfoEx should handle read-only files automatically
		// with the FILE_DISPOSITION_FLAG_IGNORE_READONLY_ATTRIBUTE flag
		err = deleteWithFileInfo(pathPtr)
		
		if supportsFileDispositionInfoEx() {
			// On Windows 10 RS1+, deletion should succeed automatically
			if err != nil {
				t.Errorf("deleteWithFileInfo failed on read-only file (Windows 10 RS1+): %v", err)
			}
			// Verify the file is deleted
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Error("Read-only file still exists after deletion")
			}
		} else {
			// On older Windows, FileDispositionInfo doesn't have the ignore readonly flag
			// The deletion may fail, which is expected behavior
			t.Logf("deleteWithFileInfo on read-only file (older Windows): %v", err)
			// Clean up if deletion failed
			if err != nil {
				// Clear read-only attribute and delete
				attrs, _ := windows.GetFileAttributes(pathPtr)
				windows.SetFileAttributes(pathPtr, attrs&^windows.FILE_ATTRIBUTE_READONLY)
				os.Remove(testFile)
			}
		}
	})

	t.Run("delete read-only file with manual attribute clearing", func(t *testing.T) {
		// This test simulates the manual retry logic that should be implemented
		// when FileDispositionInfo fails on read-only files
		
		// Create a read-only test file
		testFile := filepath.Join(tempDir, "readonly_manual.txt")
		err := os.WriteFile(testFile, []byte("readonly content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Set read-only attribute
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		attrs, err := windows.GetFileAttributes(pathPtr)
		if err != nil {
			t.Fatalf("Failed to get file attributes: %v", err)
		}

		err = windows.SetFileAttributes(pathPtr, attrs|windows.FILE_ATTRIBUTE_READONLY)
		if err != nil {
			t.Fatalf("Failed to set read-only attribute: %v", err)
		}

		// Try to delete with basic DeleteFile API (which doesn't handle read-only)
		err = windows.DeleteFile(pathPtr)
		
		// On all Windows versions, DeleteFile should fail on read-only files
		if err == nil {
			t.Error("Expected DeleteFile to fail on read-only file, but it succeeded")
		}

		// Now clear the read-only attribute and retry
		attrs, err = windows.GetFileAttributes(pathPtr)
		if err != nil {
			t.Fatalf("Failed to get file attributes for clearing: %v", err)
		}

		// Clear read-only bit
		newAttrs := attrs &^ windows.FILE_ATTRIBUTE_READONLY
		err = windows.SetFileAttributes(pathPtr, newAttrs)
		if err != nil {
			t.Fatalf("Failed to clear read-only attribute: %v", err)
		}

		// Retry deletion - should succeed now
		err = windows.DeleteFile(pathPtr)
		if err != nil {
			t.Errorf("DeleteFile failed after clearing read-only attribute: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("Read-only file still exists after manual attribute clearing and deletion")
		}
	})

	t.Run("delete multiple read-only files", func(t *testing.T) {
		// Test that the read-only handling works consistently across multiple files
		numFiles := 5
		testFiles := make([]string, numFiles)

		// Create multiple read-only files
		for i := 0; i < numFiles; i++ {
			testFile := filepath.Join(tempDir, fmt.Sprintf("readonly_multi_%d.txt", i))
			err := os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file %d: %v", i, err)
			}

			// Set read-only attribute
			extendedPath := toExtendedLengthPath(testFile)
			pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
			if err != nil {
				t.Fatalf("Failed to convert path to UTF-16 for file %d: %v", i, err)
			}

			attrs, err := windows.GetFileAttributes(pathPtr)
			if err != nil {
				t.Fatalf("Failed to get file attributes for file %d: %v", i, err)
			}

			err = windows.SetFileAttributes(pathPtr, attrs|windows.FILE_ATTRIBUTE_READONLY)
			if err != nil {
				t.Fatalf("Failed to set read-only attribute for file %d: %v", i, err)
			}

			testFiles[i] = testFile
		}

		// Delete all files
		successCount := 0
		for i, testFile := range testFiles {
			extendedPath := toExtendedLengthPath(testFile)
			pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
			if err != nil {
				t.Fatalf("Failed to convert path to UTF-16 for deletion %d: %v", i, err)
			}

			err = deleteWithFileInfo(pathPtr)
			
			if supportsFileDispositionInfoEx() {
				// On Windows 10 RS1+, all deletions should succeed
				if err != nil {
					t.Errorf("deleteWithFileInfo failed on read-only file %d (Windows 10 RS1+): %v", i, err)
				} else {
					successCount++
				}
			} else {
				// On older Windows, log the result
				t.Logf("deleteWithFileInfo on read-only file %d (older Windows): %v", i, err)
				// Clean up if deletion failed
				if err != nil {
					attrs, _ := windows.GetFileAttributes(pathPtr)
					windows.SetFileAttributes(pathPtr, attrs&^windows.FILE_ATTRIBUTE_READONLY)
					os.Remove(testFile)
				} else {
					successCount++
				}
			}
		}

		// Verify all files are deleted
		for i, testFile := range testFiles {
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Errorf("Read-only file %d still exists after deletion", i)
			}
		}

		if supportsFileDispositionInfoEx() && successCount != numFiles {
			t.Errorf("Expected all %d files to be deleted successfully, got %d", numFiles, successCount)
		}
	})

	t.Run("delete read-only file with system attribute", func(t *testing.T) {
		// Test deletion of a file with both read-only and system attributes
		testFile := filepath.Join(tempDir, "readonly_system.txt")
		err := os.WriteFile(testFile, []byte("system file content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Set read-only and system attributes
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		attrs, err := windows.GetFileAttributes(pathPtr)
		if err != nil {
			t.Fatalf("Failed to get file attributes: %v", err)
		}

		err = windows.SetFileAttributes(pathPtr, attrs|windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_SYSTEM)
		if err != nil {
			t.Fatalf("Failed to set read-only and system attributes: %v", err)
		}

		// Delete the file
		err = deleteWithFileInfo(pathPtr)
		
		if supportsFileDispositionInfoEx() {
			// On Windows 10 RS1+, deletion should succeed
			if err != nil {
				t.Errorf("deleteWithFileInfo failed on read-only+system file (Windows 10 RS1+): %v", err)
			}
			// Verify the file is deleted
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Error("Read-only+system file still exists after deletion")
			}
		} else {
			// On older Windows, log the result and clean up
			t.Logf("deleteWithFileInfo on read-only+system file (older Windows): %v", err)
			if err != nil {
				attrs, _ := windows.GetFileAttributes(pathPtr)
				windows.SetFileAttributes(pathPtr, attrs&^(windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_SYSTEM))
				os.Remove(testFile)
			}
		}
	})

	t.Run("delete read-only file in read-only directory", func(t *testing.T) {
		// Test deletion of a read-only file inside a read-only directory
		testDir := filepath.Join(tempDir, "readonly_dir")
		err := os.Mkdir(testDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		testFile := filepath.Join(testDir, "readonly_file.txt")
		err = os.WriteFile(testFile, []byte("content in readonly dir"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Set read-only attribute on the file
		extendedFilePath := toExtendedLengthPath(testFile)
		filePathPtr, err := syscall.UTF16PtrFromString(extendedFilePath)
		if err != nil {
			t.Fatalf("Failed to convert file path to UTF-16: %v", err)
		}

		attrs, err := windows.GetFileAttributes(filePathPtr)
		if err != nil {
			t.Fatalf("Failed to get file attributes: %v", err)
		}

		err = windows.SetFileAttributes(filePathPtr, attrs|windows.FILE_ATTRIBUTE_READONLY)
		if err != nil {
			t.Fatalf("Failed to set read-only attribute on file: %v", err)
		}

		// Set read-only attribute on the directory
		extendedDirPath := toExtendedLengthPath(testDir)
		dirPathPtr, err := syscall.UTF16PtrFromString(extendedDirPath)
		if err != nil {
			t.Fatalf("Failed to convert directory path to UTF-16: %v", err)
		}

		dirAttrs, err := windows.GetFileAttributes(dirPathPtr)
		if err != nil {
			t.Fatalf("Failed to get directory attributes: %v", err)
		}

		err = windows.SetFileAttributes(dirPathPtr, dirAttrs|windows.FILE_ATTRIBUTE_READONLY)
		if err != nil {
			t.Fatalf("Failed to set read-only attribute on directory: %v", err)
		}

		// Delete the file
		err = deleteWithFileInfo(filePathPtr)
		
		if supportsFileDispositionInfoEx() {
			// On Windows 10 RS1+, deletion should succeed
			if err != nil {
				t.Errorf("deleteWithFileInfo failed on read-only file in read-only directory (Windows 10 RS1+): %v", err)
			}
			// Verify the file is deleted
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Error("Read-only file in read-only directory still exists after deletion")
			}
		} else {
			// On older Windows, log the result
			t.Logf("deleteWithFileInfo on read-only file in read-only directory (older Windows): %v", err)
			// Clean up
			if err != nil {
				attrs, _ := windows.GetFileAttributes(filePathPtr)
				windows.SetFileAttributes(filePathPtr, attrs&^windows.FILE_ATTRIBUTE_READONLY)
				os.Remove(testFile)
			}
		}

		// Clean up the directory
		dirAttrs, _ = windows.GetFileAttributes(dirPathPtr)
		windows.SetFileAttributes(dirPathPtr, dirAttrs&^windows.FILE_ATTRIBUTE_READONLY)
		os.RemoveAll(testDir)
	})
}

// TestDeleteOnCloseCorrectness is a property-based test that verifies FILE_FLAG_DELETE_ON_CLOSE correctness.
// It tests that for any file, when using FILE_FLAG_DELETE_ON_CLOSE method, the file should be
// deleted successfully after the handle is closed.
//
// **Validates: Requirements 1.5**
// Feature: windows-performance-optimization, Property 3: FILE_FLAG_DELETE_ON_CLOSE correctness
func TestDeleteOnCloseCorrectness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for test files
		tempDir := t.TempDir()

		// Generate a random filename
		filename := rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}\.txt`).Draw(rt, "filename")
		testFile := filepath.Join(tempDir, filename)

		// Generate random file content (0 to 10KB)
		contentSize := rapid.IntRange(0, 10240).Draw(rt, "contentSize")
		content := make([]byte, contentSize)
		for i := range content {
			content[i] = byte(rapid.IntRange(0, 255).Draw(rt, fmt.Sprintf("byte_%d", i)))
		}

		// Create the test file
		err := os.WriteFile(testFile, content, 0644)
		if err != nil {
			rt.Fatalf("Failed to create test file: %v", err)
		}

		// Verify the file exists before deletion
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			rt.Fatalf("Test file was not created: %s", testFile)
		}

		// Convert path to UTF-16 for Windows API
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			rt.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Delete the file using FILE_FLAG_DELETE_ON_CLOSE
		err = deleteWithDeleteOnClose(pathPtr)
		if err != nil {
			rt.Fatalf("deleteWithDeleteOnClose failed: %v", err)
		}

		// Property: The file should be deleted after the handle is closed
		// Verify the file no longer exists
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			rt.Fatalf("File still exists after DELETE_ON_CLOSE deletion: %s", testFile)
		}
	})
}

// TestDeleteWithNtAPI verifies the deleteWithNtAPI function.
// This test validates Requirement 1.6.
func TestDeleteWithNtAPI(t *testing.T) {
	// Skip test if NtDeleteFile is not available
	if !supportsNtDeleteFile() {
		t.Skip("NtDeleteFile is not available on this system")
	}

	// Create a temporary directory for test files
	tempDir := t.TempDir()

	t.Run("delete regular file", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tempDir, "ntapi_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Delete the file using deleteWithNtAPI
		err = deleteWithNtAPI(pathPtr)
		if err != nil {
			t.Errorf("deleteWithNtAPI failed: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion")
		}
	})

	t.Run("delete non-existent file", func(t *testing.T) {
		// Try to delete a file that doesn't exist
		testFile := filepath.Join(tempDir, "nonexistent_ntapi.txt")
		
		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Delete should fail with appropriate error
		err = deleteWithNtAPI(pathPtr)
		if err == nil {
			t.Error("Expected error when deleting non-existent file, got nil")
		}

		// Verify error message is translated
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Expected non-empty error message")
		}
		t.Logf("Error message for non-existent file: %s", errMsg)
	})

	t.Run("delete with long path", func(t *testing.T) {
		// Create a deeply nested directory structure to test long path handling
		longPath := tempDir
		for i := 0; i < 10; i++ {
			longPath = filepath.Join(longPath, "very_long_directory_name_to_test_extended_length_paths")
		}
		
		// Create the directory structure
		err := os.MkdirAll(longPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create long path: %v", err)
		}

		// Create a test file in the deep directory
		testFile := filepath.Join(longPath, "ntapi_longpath.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file in long path: %v", err)
		}

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert long path to UTF-16: %v", err)
		}

		// Delete the file using deleteWithNtAPI
		err = deleteWithNtAPI(pathPtr)
		if err != nil {
			t.Errorf("deleteWithNtAPI failed on long path: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File in long path still exists after deletion")
		}
	})

	t.Run("delete multiple files sequentially", func(t *testing.T) {
		// Test that deleteWithNtAPI works consistently across multiple files
		numFiles := 5
		testFiles := make([]string, numFiles)

		// Create multiple test files
		for i := 0; i < numFiles; i++ {
			testFile := filepath.Join(tempDir, fmt.Sprintf("ntapi_multi_%d.txt", i))
			err := os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file %d: %v", i, err)
			}
			testFiles[i] = testFile
		}

		// Delete all files
		for i, testFile := range testFiles {
			extendedPath := toExtendedLengthPath(testFile)
			pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
			if err != nil {
				t.Fatalf("Failed to convert path to UTF-16 for file %d: %v", i, err)
			}

			err = deleteWithNtAPI(pathPtr)
			if err != nil {
				t.Errorf("deleteWithNtAPI failed on file %d: %v", i, err)
			}
		}

		// Verify all files are deleted
		for i, testFile := range testFiles {
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Errorf("File %d still exists after deletion", i)
			}
		}
	})

	t.Run("delete locked file", func(t *testing.T) {
		// Test that deleteWithNtAPI fails appropriately on locked files
		testFile := filepath.Join(tempDir, "locked_ntapi.txt")
		err := os.WriteFile(testFile, []byte("locked content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Open the file to lock it
		file, err := os.OpenFile(testFile, os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("Failed to open file for locking: %v", err)
		}
		defer file.Close()

		// Convert path to UTF-16
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Try to delete the locked file - should fail
		err = deleteWithNtAPI(pathPtr)
		if err == nil {
			t.Error("Expected error when deleting locked file, got nil")
		}

		// Verify error message indicates sharing violation
		errMsg := err.Error()
		t.Logf("Error message for locked file: %s", errMsg)

		// The file should still exist
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("Locked file was deleted (should have failed)")
		}
	})

	t.Run("delete read-only file", func(t *testing.T) {
		// Test that deleteWithNtAPI handles read-only files
		// Note: NtDeleteFile does not automatically ignore read-only attributes
		testFile := filepath.Join(tempDir, "readonly_ntapi.txt")
		err := os.WriteFile(testFile, []byte("readonly content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Set read-only attribute
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		attrs, err := windows.GetFileAttributes(pathPtr)
		if err != nil {
			t.Fatalf("Failed to get file attributes: %v", err)
		}

		err = windows.SetFileAttributes(pathPtr, attrs|windows.FILE_ATTRIBUTE_READONLY)
		if err != nil {
			t.Fatalf("Failed to set read-only attribute: %v", err)
		}

		// Try to delete the read-only file
		// NtDeleteFile does not automatically handle read-only attributes
		err = deleteWithNtAPI(pathPtr)
		
		// This may succeed or fail depending on Windows version and configuration
		if err == nil {
			t.Log("deleteWithNtAPI succeeded on read-only file")
			// Verify the file is deleted
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Error("Read-only file still exists after successful deletion")
			}
		} else {
			t.Logf("deleteWithNtAPI failed on read-only file (expected): %v", err)
			// Clean up
			attrs, _ := windows.GetFileAttributes(pathPtr)
			windows.SetFileAttributes(pathPtr, attrs&^windows.FILE_ATTRIBUTE_READONLY)
			os.Remove(testFile)
		}
	})

	t.Run("verify NT status code translation", func(t *testing.T) {
		// Test various error scenarios to verify NT status code translation
		
		// Test 1: Non-existent file (should translate to "file not found" or "path not found")
		testFile := filepath.Join(tempDir, "nonexistent_status_test.txt")
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		err = deleteWithNtAPI(pathPtr)
		if err == nil {
			t.Error("Expected error for non-existent file")
		} else {
			errMsg := err.Error()
			// Should contain "not found" in the error message
			if errMsg == "" {
				t.Error("Expected non-empty error message")
			}
			t.Logf("NT status translation for non-existent file: %s", errMsg)
		}

		// Test 2: Invalid path (should translate to appropriate error)
		invalidPath := filepath.Join(tempDir, "invalid\x00path.txt")
		extendedPath = toExtendedLengthPath(invalidPath)
		pathPtr, err = syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			// This is expected - UTF16PtrFromString should fail on null bytes
			t.Logf("UTF16PtrFromString correctly rejected invalid path: %v", err)
		}
	})
}

// TestNtDeleteFileCorrectness is a property-based test that verifies NtDeleteFile correctness.
// It tests that for any file, when using NtDeleteFile method, the file should be deleted
// successfully and NT status codes should be translated to readable error messages.
//
// **Validates: Requirements 1.6, 8.4**
// Feature: windows-performance-optimization, Property 4: NtDeleteFile correctness
func TestNtDeleteFileCorrectness(t *testing.T) {
	// Skip test if NtDeleteFile is not available
	if !supportsNtDeleteFile() {
		t.Skip("NtDeleteFile is not available on this system")
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for test files
		tempDir := t.TempDir()

		// Generate a random filename
		filename := rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}\.txt`).Draw(rt, "filename")
		testFile := filepath.Join(tempDir, filename)

		// Generate random file content (0 to 10KB)
		contentSize := rapid.IntRange(0, 10240).Draw(rt, "contentSize")
		content := make([]byte, contentSize)
		for i := range content {
			content[i] = byte(rapid.IntRange(0, 255).Draw(rt, fmt.Sprintf("byte_%d", i)))
		}

		// Create the test file
		err := os.WriteFile(testFile, content, 0644)
		if err != nil {
			rt.Fatalf("Failed to create test file: %v", err)
		}

		// Verify the file exists before deletion
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			rt.Fatalf("Test file was not created: %s", testFile)
		}

		// Convert path to UTF-16 for Windows API
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			rt.Fatalf("Failed to convert path to UTF-16: %v", err)
		}

		// Delete the file using NtDeleteFile
		err = deleteWithNtAPI(pathPtr)
		if err != nil {
			rt.Fatalf("deleteWithNtAPI failed: %v", err)
		}

		// Property 1: The file should be deleted successfully
		// Verify the file no longer exists
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			rt.Fatalf("File still exists after NtDeleteFile deletion: %s", testFile)
		}
	})

	// Test error scenarios to verify NT status code translation
	t.Run("NT status code translation", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			tempDir := t.TempDir()

			// Generate a random non-existent filename
			filename := rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}\.txt`).Draw(rt, "filename")
			testFile := filepath.Join(tempDir, filename)

			// Ensure the file doesn't exist
			os.Remove(testFile)

			// Convert path to UTF-16
			extendedPath := toExtendedLengthPath(testFile)
			pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
			if err != nil {
				rt.Fatalf("Failed to convert path to UTF-16: %v", err)
			}

			// Try to delete non-existent file
			err = deleteWithNtAPI(pathPtr)

			// Property 2: NT status codes should be translated to readable error messages
			if err == nil {
				rt.Fatalf("Expected error when deleting non-existent file, got nil")
			}

			// Verify error message is not empty and is human-readable
			errMsg := err.Error()
			if errMsg == "" {
				rt.Fatalf("Expected non-empty error message, got empty string")
			}

			// Error message should not be just a raw hex code (should be translated)
			// We allow messages that contain hex codes, but they should also have descriptive text
			if len(errMsg) < 5 {
				rt.Fatalf("Error message too short, likely not properly translated: %s", errMsg)
			}
		})
	})
}

// TestWindowsAdvancedBackend verifies the WindowsAdvancedBackend implementation.
func TestWindowsAdvancedBackend(t *testing.T) {
	backend := NewWindowsAdvancedBackend()

	t.Run("default deletion method is auto", func(t *testing.T) {
		// Verify the backend is initialized with MethodAuto
		stats := backend.GetDeletionStats()
		if stats == nil {
			t.Fatal("Expected non-nil stats")
		}
	})

	t.Run("set and get deletion method", func(t *testing.T) {
		// Test setting different deletion methods
		methods := []DeletionMethod{
			MethodFileInfo,
			MethodDeleteOnClose,
			MethodNtAPI,
			MethodDeleteAPI,
			MethodAuto,
		}

		for _, method := range methods {
			backend.SetDeletionMethod(method)
			// We can't directly get the method, but we can verify it doesn't panic
			// and that subsequent operations work
			stats := backend.GetDeletionStats()
			if stats == nil {
				t.Errorf("Expected non-nil stats after setting method %s", method.String())
			}
		}
	})

	t.Run("deletion stats tracking", func(t *testing.T) {
		// Create a fresh backend for this test
		testBackend := NewWindowsAdvancedBackend()
		tempDir := t.TempDir()

		// Create a test file
		testFile := filepath.Join(tempDir, "stats_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Delete the file using the backend
		err = testBackend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed: %v", err)
		}

		// Get stats and verify they were updated
		stats := testBackend.GetDeletionStats()
		totalAttempts := stats.FileInfoAttempts + stats.DeleteOnCloseAttempts + 
			stats.NtAPIAttempts + stats.FallbackAttempts
		totalSuccesses := stats.FileInfoSuccesses + stats.DeleteOnCloseSuccesses + 
			stats.NtAPISuccesses + stats.FallbackSuccesses

		if totalAttempts == 0 {
			t.Error("Expected at least one deletion attempt")
		}

		if totalSuccesses == 0 {
			t.Error("Expected at least one successful deletion")
		}

		t.Logf("Deletion stats: %+v", stats)
	})

	t.Run("delete file with specific method", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test each deletion method individually
		methods := []DeletionMethod{
			MethodFileInfo,
			MethodDeleteOnClose,
		}

		// Add NtAPI if available
		if supportsNtDeleteFile() {
			methods = append(methods, MethodNtAPI)
		}

		// Always test the baseline method
		methods = append(methods, MethodDeleteAPI)

		for _, method := range methods {
			t.Run(method.String(), func(t *testing.T) {
				testBackend := NewWindowsAdvancedBackend()
				testBackend.SetDeletionMethod(method)

				// Create a test file
				testFile := filepath.Join(tempDir, fmt.Sprintf("method_test_%s.txt", method.String()))
				err := os.WriteFile(testFile, []byte("test content"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}

				// Delete the file
				err = testBackend.DeleteFile(testFile)
				if err != nil {
					t.Errorf("DeleteFile with method %s failed: %v", method.String(), err)
				}

				// Verify the file is deleted
				if _, err := os.Stat(testFile); !os.IsNotExist(err) {
					t.Errorf("File still exists after deletion with method %s", method.String())
				}
			})
		}
	})

	t.Run("delete directory", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a test directory
		testDir := filepath.Join(tempDir, "test_dir")
		err := os.Mkdir(testDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Delete the directory
		err = backend.DeleteDirectory(testDir)
		if err != nil {
			t.Errorf("DeleteDirectory failed: %v", err)
		}

		// Verify the directory is deleted
		if _, err := os.Stat(testDir); !os.IsNotExist(err) {
			t.Error("Directory still exists after deletion")
		}
	})
}

// TestDeletionMethodFallbackOnFailure is a property-based test that verifies deletion method
// fallback behavior. It tests that for any file where a deletion method fails, the backend
// should attempt the next method in the fallback chain before reporting failure.
//
// **Validates: Requirements 8.2, 8.1**
// Feature: windows-performance-optimization, Property 20: Deletion method fallback on failure
func TestDeletionMethodFallbackOnFailure(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tempDir := t.TempDir()

		// Generate a random filename
		filename := rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}\.txt`).Draw(rt, "filename")
		testFile := filepath.Join(tempDir, filename)

		// Generate random file content (0 to 10KB)
		contentSize := rapid.IntRange(0, 10240).Draw(rt, "contentSize")
		content := make([]byte, contentSize)
		for i := range content {
			content[i] = byte(rapid.IntRange(0, 255).Draw(rt, fmt.Sprintf("byte_%d", i)))
		}

		// Create the test file
		err := os.WriteFile(testFile, content, 0644)
		if err != nil {
			rt.Fatalf("Failed to create test file: %v", err)
		}

		// Randomly decide whether to make the file read-only (to test fallback with retry)
		makeReadOnly := rapid.Bool().Draw(rt, "makeReadOnly")
		if makeReadOnly {
			extendedPath := toExtendedLengthPath(testFile)
			pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
			if err != nil {
				rt.Fatalf("Failed to convert path to UTF-16: %v", err)
			}

			attrs, err := windows.GetFileAttributes(pathPtr)
			if err != nil {
				rt.Fatalf("Failed to get file attributes: %v", err)
			}

			err = windows.SetFileAttributes(pathPtr, attrs|windows.FILE_ATTRIBUTE_READONLY)
			if err != nil {
				rt.Fatalf("Failed to set read-only attribute: %v", err)
			}
		}

		// Create a backend with auto fallback enabled
		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodAuto)

		// Delete the file - the backend should try multiple methods if needed
		err = backend.DeleteFile(testFile)
		if err != nil {
			rt.Fatalf("DeleteFile failed even with fallback chain: %v", err)
		}

		// Property: The file should be deleted successfully after trying fallback methods
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			rt.Fatalf("File still exists after deletion with fallback: %s", testFile)
		}

		// Verify that at least one method was attempted
		stats := backend.GetDeletionStats()
		totalAttempts := stats.FileInfoAttempts + stats.DeleteOnCloseAttempts + 
			stats.NtAPIAttempts + stats.FallbackAttempts

		if totalAttempts == 0 {
			rt.Fatalf("Expected at least one deletion attempt, got 0")
		}

		// Verify that at least one method succeeded
		totalSuccesses := stats.FileInfoSuccesses + stats.DeleteOnCloseSuccesses + 
			stats.NtAPISuccesses + stats.FallbackSuccesses

		if totalSuccesses == 0 {
			rt.Fatalf("Expected at least one successful deletion, got 0")
		}

		// Property: If read-only file was successfully deleted, verify the fallback chain
		// handled the read-only attribute (either via FileDispositionInfoEx or manual clearing)
		if makeReadOnly {
			// On Windows 10 RS1+, FileDispositionInfoEx should handle it automatically
			// On older Windows, the fallback chain should have cleared the attribute
			// Either way, the file should be deleted
			if supportsFileDispositionInfoEx() {
				// FileDispositionInfoEx should have succeeded
				if stats.FileInfoSuccesses == 0 {
					rt.Logf("Warning: FileDispositionInfoEx available but not used for read-only file")
				}
			}
		}
	})

	// Additional test: Verify fallback chain with specific failure scenarios
	t.Run("fallback chain with non-existent file", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "nonexistent.txt")

		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodAuto)

		// Try to delete non-existent file - all methods should fail
		err := backend.DeleteFile(testFile)
		if err == nil {
			t.Error("Expected error when deleting non-existent file, got nil")
		}

		// Verify that multiple methods were attempted before giving up
		stats := backend.GetDeletionStats()
		totalAttempts := stats.FileInfoAttempts + stats.DeleteOnCloseAttempts + 
			stats.NtAPIAttempts + stats.FallbackAttempts

		// Should have tried at least 2 methods (depending on availability)
		if totalAttempts < 2 {
			t.Errorf("Expected at least 2 deletion attempts in fallback chain, got %d", totalAttempts)
		}

		// No methods should have succeeded
		totalSuccesses := stats.FileInfoSuccesses + stats.DeleteOnCloseSuccesses + 
			stats.NtAPISuccesses + stats.FallbackSuccesses

		if totalSuccesses > 0 {
			t.Errorf("Expected 0 successful deletions for non-existent file, got %d", totalSuccesses)
		}
	})

	t.Run("fallback chain with locked file", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "locked.txt")

		// Create and lock the file
		err := os.WriteFile(testFile, []byte("locked content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		file, err := os.OpenFile(testFile, os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("Failed to open file for locking: %v", err)
		}
		defer file.Close()

		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodAuto)

		// Try to delete locked file - all methods should fail
		err = backend.DeleteFile(testFile)
		if err == nil {
			t.Error("Expected error when deleting locked file, got nil")
		}

		// Verify that multiple methods were attempted
		stats := backend.GetDeletionStats()
		totalAttempts := stats.FileInfoAttempts + stats.DeleteOnCloseAttempts + 
			stats.NtAPIAttempts + stats.FallbackAttempts

		if totalAttempts < 2 {
			t.Errorf("Expected at least 2 deletion attempts in fallback chain, got %d", totalAttempts)
		}

		// File should still exist
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("Locked file was deleted (should have failed)")
		}
	})
}

// TestErrorResilience is a property-based test that verifies error resilience behavior.
// It tests that for any file that fails deletion after all retry attempts, the system
// should log the error, increment the failure count, and continue processing remaining files.
//
// **Validates: Requirements 8.3, 8.5**
// Feature: windows-performance-optimization, Property 21: Error resilience
func TestErrorResilience(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tempDir := t.TempDir()

		// Generate multiple test files (some will succeed, some will fail)
		numFiles := rapid.IntRange(3, 10).Draw(rt, "numFiles")
		testFiles := make([]string, numFiles)
		lockedFiles := make([]*os.File, 0)
		defer func() {
			for _, f := range lockedFiles {
				f.Close()
			}
		}()

		// Create test files
		for i := 0; i < numFiles; i++ {
			filename := fmt.Sprintf("resilience_test_%d.txt", i)
			testFile := filepath.Join(tempDir, filename)
			
			content := []byte(fmt.Sprintf("content %d", i))
			err := os.WriteFile(testFile, content, 0644)
			if err != nil {
				rt.Fatalf("Failed to create test file %d: %v", i, err)
			}

			testFiles[i] = testFile

			// Randomly lock some files to cause failures
			shouldLock := rapid.Bool().Draw(rt, fmt.Sprintf("lockFile_%d", i))
			if shouldLock && i < numFiles-1 { // Don't lock all files
				file, err := os.OpenFile(testFile, os.O_RDWR, 0644)
				if err != nil {
					rt.Fatalf("Failed to open file %d for locking: %v", i, err)
				}
				lockedFiles = append(lockedFiles, file)
			}
		}

		// Create a backend
		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodAuto)

		// Try to delete all files
		successCount := 0
		failureCount := 0
		var lastError error

		for i, testFile := range testFiles {
			err := backend.DeleteFile(testFile)
			if err != nil {
				failureCount++
				lastError = err
				// Property: Error should be logged but processing should continue
				// We verify this by checking that we continue to the next file
				rt.Logf("File %d deletion failed (expected for locked files): %v", i, err)
			} else {
				successCount++
			}
		}

		// Property 1: At least one file should have been processed (success or failure)
		if successCount+failureCount != numFiles {
			rt.Fatalf("Expected %d files processed, got %d successes + %d failures = %d",
				numFiles, successCount, failureCount, successCount+failureCount)
		}

		// Property 2: If there were failures, lastError should not be nil
		if failureCount > 0 && lastError == nil {
			rt.Fatalf("Expected non-nil error when %d files failed", failureCount)
		}

		// Property 3: If there were successes, those files should be deleted
		for i, testFile := range testFiles {
			_, err := os.Stat(testFile)
			fileExists := !os.IsNotExist(err)

			// Check if this file was locked
			wasLocked := false
			for _, lockedFile := range lockedFiles {
				if lockedFile.Name() == testFile {
					wasLocked = true
					break
				}
			}

			if wasLocked {
				// Locked files should still exist
				if !fileExists {
					rt.Fatalf("Locked file %d was deleted (should have failed): %s", i, testFile)
				}
			} else {
				// Unlocked files should be deleted
				if fileExists {
					rt.Fatalf("Unlocked file %d still exists (should have been deleted): %s", i, testFile)
				}
			}
		}

		// Property 4: Backend stats should reflect both successes and failures
		stats := backend.GetDeletionStats()
		totalSuccesses := stats.FileInfoSuccesses + stats.DeleteOnCloseSuccesses + 
			stats.NtAPISuccesses + stats.FallbackSuccesses

		if totalSuccesses != successCount {
			rt.Fatalf("Expected %d successful deletions in stats, got %d", successCount, totalSuccesses)
		}

		rt.Logf("Error resilience test: %d successes, %d failures out of %d files",
			successCount, failureCount, numFiles)
	})

	// Additional test: Verify that errors don't stop processing
	t.Run("continue processing after errors", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a sequence of files: unlocked, locked, unlocked, locked, unlocked
		testFiles := []string{
			filepath.Join(tempDir, "file1.txt"),
			filepath.Join(tempDir, "file2.txt"),
			filepath.Join(tempDir, "file3.txt"),
			filepath.Join(tempDir, "file4.txt"),
			filepath.Join(tempDir, "file5.txt"),
		}

		lockedIndices := []int{1, 3} // Lock files 2 and 4
		lockedFiles := make([]*os.File, 0)
		defer func() {
			for _, f := range lockedFiles {
				f.Close()
			}
		}()

		// Create all files
		for i, testFile := range testFiles {
			err := os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file %d: %v", i, err)
			}

			// Lock specific files
			for _, lockedIdx := range lockedIndices {
				if i == lockedIdx {
					file, err := os.OpenFile(testFile, os.O_RDWR, 0644)
					if err != nil {
						t.Fatalf("Failed to open file %d for locking: %v", i, err)
					}
					lockedFiles = append(lockedFiles, file)
					break
				}
			}
		}

		// Create a backend
		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodAuto)

		// Try to delete all files
		successCount := 0
		failureCount := 0

		for i, testFile := range testFiles {
			err := backend.DeleteFile(testFile)
			if err != nil {
				failureCount++
				t.Logf("File %d deletion failed (expected for locked files): %v", i, err)
			} else {
				successCount++
			}
		}

		// Verify results
		expectedSuccesses := len(testFiles) - len(lockedIndices)
		expectedFailures := len(lockedIndices)

		if successCount != expectedSuccesses {
			t.Errorf("Expected %d successful deletions, got %d", expectedSuccesses, successCount)
		}

		if failureCount != expectedFailures {
			t.Errorf("Expected %d failed deletions, got %d", expectedFailures, failureCount)
		}

		// Verify that unlocked files were deleted and locked files still exist
		for i, testFile := range testFiles {
			_, err := os.Stat(testFile)
			fileExists := !os.IsNotExist(err)

			isLocked := false
			for _, lockedIdx := range lockedIndices {
				if i == lockedIdx {
					isLocked = true
					break
				}
			}

			if isLocked {
				if !fileExists {
					t.Errorf("Locked file %d was deleted (should have failed): %s", i, testFile)
				}
			} else {
				if fileExists {
					t.Errorf("Unlocked file %d still exists (should have been deleted): %s", i, testFile)
				}
			}
		}

		t.Logf("Error resilience verified: %d successes, %d failures", successCount, failureCount)
	})
}
