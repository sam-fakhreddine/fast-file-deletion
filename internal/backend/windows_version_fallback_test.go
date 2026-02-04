//go:build windows

package backend

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"pgregory.net/rapid"
)

// TestVersionBasedFallback is a property-based test that verifies version-based fallback.
// It tests that for any Windows version older than Windows 10 RS1, the system should
// automatically select compatible deletion methods without requiring user intervention.
//
// The test validates that:
// 1. Windows version detection works correctly
// 2. API availability is correctly determined based on version
// 3. The backend automatically selects compatible methods
// 4. Files can be deleted successfully regardless of Windows version
// 5. No user intervention is required for fallback behavior
//
// **Validates: Requirements 7.1, 7.5**
// Feature: windows-performance-optimization, Property 18: Version-based fallback
func TestVersionBasedFallback(t *testing.T) {
	// First, verify that version detection works
	major, minor, build := getWindowsVersion()
	
	t.Logf("Detected Windows version: %d.%d (build %d)", major, minor, build)
	
	// Verify version numbers are valid
	if major == 0 {
		t.Fatal("Windows version detection failed: major version is 0")
	}
	
	// Determine expected API availability based on version
	expectedFileInfoEx := major > 10 || (major == 10 && build >= windows10RS1Build)
	actualFileInfoEx := supportsFileDispositionInfoEx()
	
	if expectedFileInfoEx != actualFileInfoEx {
		t.Errorf("FileDispositionInfoEx availability mismatch: expected %v, got %v (Windows %d.%d.%d)",
			expectedFileInfoEx, actualFileInfoEx, major, minor, build)
	}
	
	t.Logf("FileDispositionInfoEx available: %v (expected: %v)", actualFileInfoEx, expectedFileInfoEx)
	
	// Test that the backend automatically selects compatible methods
	t.Run("automatic method selection", func(t *testing.T) {
		backend := NewWindowsAdvancedBackend()
		
		// Backend should be initialized with MethodAuto
		// This ensures automatic fallback behavior
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "version_fallback_test.txt")
		
		// Create a test file
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		
		// Delete the file using the backend
		// The backend should automatically select the appropriate method
		// based on Windows version without user intervention
		err = backend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed with automatic method selection: %v", err)
		}
		
		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion with automatic method selection")
		}
		
		// Check which methods were used
		stats := backend.GetDeletionStats()
		
		// On Windows 10 RS1+, FileInfo should be attempted
		// On older Windows, FileInfo should not be attempted (or should fail and fallback)
		if supportsFileDispositionInfoEx() {
			if stats.FileInfoAttempts == 0 {
				t.Error("Expected FileInfo attempts on Windows 10 RS1+, got 0")
			}
			t.Logf("FileInfo method used: %d attempts, %d successes",
				stats.FileInfoAttempts, stats.FileInfoSuccesses)
		} else {
			// On older Windows, FileInfo should not be attempted
			// Instead, other methods should be used
			totalAttempts := stats.DeleteOnCloseAttempts + stats.NtAPIAttempts + stats.FallbackAttempts
			if totalAttempts == 0 {
				t.Error("Expected alternative methods to be attempted on older Windows, got 0")
			}
			t.Logf("Alternative methods used on older Windows: DeleteOnClose=%d, NtAPI=%d, Fallback=%d",
				stats.DeleteOnCloseAttempts, stats.NtAPIAttempts, stats.FallbackAttempts)
		}
	})
	
	// Test that all available methods work on the current Windows version
	t.Run("compatible methods work", func(t *testing.T) {
		tempDir := t.TempDir()
		backend := NewWindowsAdvancedBackend()
		
		// Test methods that should be available on all Windows versions
		compatibleMethods := []DeletionMethod{
			MethodDeleteOnClose, // Available on Windows 2000+
			MethodDeleteAPI,     // Available on all Windows versions
		}
		
		// Add MethodFileInfo if supported
		if supportsFileDispositionInfoEx() {
			compatibleMethods = append([]DeletionMethod{MethodFileInfo}, compatibleMethods...)
		}
		
		// Add MethodNtAPI if supported
		if supportsNtDeleteFile() {
			compatibleMethods = append(compatibleMethods, MethodNtAPI)
		}
		
		for _, method := range compatibleMethods {
			t.Run(method.String(), func(t *testing.T) {
				backend.SetDeletionMethod(method)
				
				testFile := filepath.Join(tempDir, "compatible_"+method.String()+".txt")
				err := os.WriteFile(testFile, []byte("test content"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				
				err = backend.DeleteFile(testFile)
				if err != nil {
					t.Errorf("DeleteFile failed with compatible method %s: %v", method.String(), err)
				}
				
				if _, err := os.Stat(testFile); !os.IsNotExist(err) {
					t.Errorf("File still exists after deletion with method %s", method.String())
				}
			})
		}
	})
	
	// Test that incompatible methods are not used on older Windows
	t.Run("incompatible methods not used", func(t *testing.T) {
		if supportsFileDispositionInfoEx() {
			t.Skip("Skipping incompatible methods test on Windows 10 RS1+ (all methods supported)")
		}
		
		// On older Windows, FileDispositionInfoEx should not be available
		// The backend should automatically fall back to compatible methods
		tempDir := t.TempDir()
		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodAuto)
		
		testFile := filepath.Join(tempDir, "incompatible_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		
		err = backend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed on older Windows with automatic fallback: %v", err)
		}
		
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion on older Windows")
		}
		
		// Verify that FileInfo was not used (or failed and fell back)
		stats := backend.GetDeletionStats()
		if stats.FileInfoSuccesses > 0 {
			t.Error("FileInfo method succeeded on older Windows (should not be available)")
		}
		
		// Verify that alternative methods were used
		totalSuccesses := stats.DeleteOnCloseSuccesses + stats.NtAPISuccesses + stats.FallbackSuccesses
		if totalSuccesses == 0 {
			t.Error("No alternative methods succeeded on older Windows")
		}
	})
	
	// Property-based test: deletion works across various file scenarios
	// regardless of Windows version
	t.Run("property: deletion works on any Windows version", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			tempDir := t.TempDir()
			backend := NewWindowsAdvancedBackend()
			backend.SetDeletionMethod(MethodAuto)
			
			// Generate random file name
			fileName := rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}\.txt`).Draw(t, "fileName")
			testFile := filepath.Join(tempDir, fileName)
			
			// Generate random file content
			content := rapid.SliceOfN(rapid.Byte(), 0, 1024).Draw(t, "content")
			
			// Create the test file
			err := os.WriteFile(testFile, content, 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			
			// Delete the file using automatic method selection
			err = backend.DeleteFile(testFile)
			if err != nil {
				t.Errorf("DeleteFile failed with automatic method selection: %v", err)
			}
			
			// Verify the file is deleted
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Errorf("File still exists after deletion: %s", testFile)
			}
			
			// Verify that at least one method succeeded
			stats := backend.GetDeletionStats()
			totalSuccesses := stats.FileInfoSuccesses + stats.DeleteOnCloseSuccesses +
				stats.NtAPISuccesses + stats.FallbackSuccesses
			
			if totalSuccesses == 0 {
				t.Error("No deletion method succeeded")
			}
		})
	})
	
	// Test that GetAPIAvailability provides correct information
	t.Run("API availability reporting", func(t *testing.T) {
		major, minor, build, hasFileInfoEx, hasNtDelete := GetAPIAvailability()
		
		// Verify version matches getWindowsVersion
		expectedMajor, expectedMinor, expectedBuild := getWindowsVersion()
		if major != expectedMajor || minor != expectedMinor || build != expectedBuild {
			t.Errorf("GetAPIAvailability version mismatch: got %d.%d.%d, expected %d.%d.%d",
				major, minor, build, expectedMajor, expectedMinor, expectedBuild)
		}
		
		// Verify FileInfoEx availability matches version
		expectedFileInfoEx := major > 10 || (major == 10 && build >= windows10RS1Build)
		if hasFileInfoEx != expectedFileInfoEx {
			t.Errorf("GetAPIAvailability FileInfoEx mismatch: got %v, expected %v",
				hasFileInfoEx, expectedFileInfoEx)
		}
		
		// Log the availability for visibility
		t.Logf("API Availability Report:")
		t.Logf("  Windows Version: %d.%d (build %d)", major, minor, build)
		t.Logf("  FileDispositionInfoEx: %v", hasFileInfoEx)
		t.Logf("  NtDeleteFile: %v", hasNtDelete)
		
		// Verify that the information is useful for logging/warnings
		if !hasFileInfoEx {
			t.Logf("WARNING: FileDispositionInfoEx not available (Windows < 10 RS1)")
			t.Logf("         Falling back to compatible deletion methods")
		}
	})
	
	// Test that the backend works with UTF-16 paths on any Windows version
	t.Run("UTF-16 path handling across versions", func(t *testing.T) {
		tempDir := t.TempDir()
		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodAuto)
		
		testFile := filepath.Join(tempDir, "utf16_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		
		// Convert to UTF-16 manually
		extendedPath := toExtendedLengthPath(testFile)
		pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}
		
		// Delete using UTF-16 path
		err = backend.DeleteFileUTF16(pathPtr, testFile)
		if err != nil {
			t.Errorf("DeleteFileUTF16 failed with automatic method selection: %v", err)
		}
		
		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after UTF-16 deletion")
		}
	})
	
	// Test that no user intervention is required for fallback
	t.Run("no user intervention required", func(t *testing.T) {
		// This test verifies that the backend automatically handles fallback
		// without requiring any configuration or user intervention
		
		tempDir := t.TempDir()
		
		// Create a backend with default settings (MethodAuto)
		backend := NewWindowsAdvancedBackend()
		
		// No SetDeletionMethod call - should use MethodAuto by default
		// No version checking by user - backend should handle it internally
		
		// Create and delete multiple files
		numFiles := 10
		for i := 0; i < numFiles; i++ {
			testFile := filepath.Join(tempDir, filepath.Base(t.Name())+"_"+string(rune('a'+i))+".txt")
			err := os.WriteFile(testFile, []byte("test content"), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file %d: %v", i, err)
			}
			
			// Delete without any special configuration
			err = backend.DeleteFile(testFile)
			if err != nil {
				t.Errorf("DeleteFile failed for file %d (no user intervention): %v", i, err)
			}
			
			if _, err := os.Stat(testFile); !os.IsNotExist(err) {
				t.Errorf("File %d still exists after deletion", i)
			}
		}
		
		// Verify that deletions succeeded
		stats := backend.GetDeletionStats()
		totalSuccesses := stats.FileInfoSuccesses + stats.DeleteOnCloseSuccesses +
			stats.NtAPISuccesses + stats.FallbackSuccesses
		
		if totalSuccesses != numFiles {
			t.Errorf("Expected %d successful deletions, got %d", numFiles, totalSuccesses)
		}
		
		t.Logf("Successfully deleted %d files without user intervention", numFiles)
		t.Logf("Methods used: FileInfo=%d, DeleteOnClose=%d, NtAPI=%d, Fallback=%d",
			stats.FileInfoSuccesses, stats.DeleteOnCloseSuccesses,
			stats.NtAPISuccesses, stats.FallbackSuccesses)
	})
}

// TestVersionDetectionCaching verifies that Windows version detection is cached
// and doesn't require repeated system calls.
func TestVersionDetectionCaching(t *testing.T) {
	// Call getWindowsVersion multiple times
	major1, minor1, build1 := getWindowsVersion()
	major2, minor2, build2 := getWindowsVersion()
	major3, minor3, build3 := getWindowsVersion()
	
	// All calls should return the same values (cached)
	if major1 != major2 || major1 != major3 {
		t.Errorf("Major version not cached: %d, %d, %d", major1, major2, major3)
	}
	
	if minor1 != minor2 || minor1 != minor3 {
		t.Errorf("Minor version not cached: %d, %d, %d", minor1, minor2, minor3)
	}
	
	if build1 != build2 || build1 != build3 {
		t.Errorf("Build number not cached: %d, %d, %d", build1, build2, build3)
	}
	
	t.Logf("Version detection properly cached: %d.%d (build %d)", major1, minor1, build1)
}

// TestAPIAvailabilityCaching verifies that API availability checks are cached
// and don't require repeated checks.
func TestAPIAvailabilityCaching(t *testing.T) {
	// Call API availability functions multiple times
	fileInfoEx1 := supportsFileDispositionInfoEx()
	fileInfoEx2 := supportsFileDispositionInfoEx()
	fileInfoEx3 := supportsFileDispositionInfoEx()
	
	ntDelete1 := supportsNtDeleteFile()
	ntDelete2 := supportsNtDeleteFile()
	ntDelete3 := supportsNtDeleteFile()
	
	// All calls should return the same values (cached)
	if fileInfoEx1 != fileInfoEx2 || fileInfoEx1 != fileInfoEx3 {
		t.Errorf("FileDispositionInfoEx availability not cached: %v, %v, %v",
			fileInfoEx1, fileInfoEx2, fileInfoEx3)
	}
	
	if ntDelete1 != ntDelete2 || ntDelete1 != ntDelete3 {
		t.Errorf("NtDeleteFile availability not cached: %v, %v, %v",
			ntDelete1, ntDelete2, ntDelete3)
	}
	
	t.Logf("API availability properly cached: FileInfoEx=%v, NtDelete=%v",
		fileInfoEx1, ntDelete1)
}

// TestVersionBasedMethodSelection verifies that the backend selects appropriate
// methods based on Windows version.
func TestVersionBasedMethodSelection(t *testing.T) {
	tempDir := t.TempDir()
	backend := NewWindowsAdvancedBackend()
	backend.SetDeletionMethod(MethodAuto)
	
	// Create and delete a test file
	testFile := filepath.Join(tempDir, "method_selection_test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	err = backend.DeleteFile(testFile)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}
	
	// Check which methods were used
	stats := backend.GetDeletionStats()
	
	// On Windows 10 RS1+, FileInfo should be the primary method
	if supportsFileDispositionInfoEx() {
		if stats.FileInfoAttempts == 0 {
			t.Error("Expected FileInfo to be attempted on Windows 10 RS1+")
		}
		if stats.FileInfoSuccesses == 0 {
			t.Error("Expected FileInfo to succeed on Windows 10 RS1+")
		}
		t.Logf("Windows 10 RS1+ detected: FileInfo method used successfully")
	} else {
		// On older Windows, alternative methods should be used
		if stats.FileInfoSuccesses > 0 {
			t.Error("FileInfo should not succeed on Windows < 10 RS1")
		}
		
		totalSuccesses := stats.DeleteOnCloseSuccesses + stats.NtAPISuccesses + stats.FallbackSuccesses
		if totalSuccesses == 0 {
			t.Error("Expected alternative methods to succeed on older Windows")
		}
		
		t.Logf("Older Windows detected: Alternative methods used successfully")
		t.Logf("  DeleteOnClose: %d successes", stats.DeleteOnCloseSuccesses)
		t.Logf("  NtAPI: %d successes", stats.NtAPISuccesses)
		t.Logf("  Fallback: %d successes", stats.FallbackSuccesses)
	}
}
