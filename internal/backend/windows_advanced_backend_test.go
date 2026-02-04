//go:build windows

package backend

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWindowsAdvancedBackend_SetDeletionMethod verifies that SetDeletionMethod
// correctly configures the deletion method.
func TestWindowsAdvancedBackend_SetDeletionMethod(t *testing.T) {
	backend := NewWindowsAdvancedBackend()

	// Test default method is MethodAuto
	stats := backend.GetDeletionStats()
	if stats == nil {
		t.Fatal("GetDeletionStats returned nil")
	}

	// Test setting different methods
	methods := []DeletionMethod{
		MethodFileInfo,
		MethodDeleteOnClose,
		MethodNtAPI,
		MethodDeleteAPI,
		MethodAuto,
	}

	for _, method := range methods {
		backend.SetDeletionMethod(method)
		// Verify the method was set (we can't directly access the field,
		// but we can verify it doesn't panic and stats are still accessible)
		stats := backend.GetDeletionStats()
		if stats == nil {
			t.Errorf("GetDeletionStats returned nil after setting method %s", method.String())
		}
	}
}

// TestWindowsAdvancedBackend_DeleteFile verifies that DeleteFile correctly
// routes to the appropriate deletion method.
func TestWindowsAdvancedBackend_DeleteFile(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("delete with MethodAuto", func(t *testing.T) {
		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodAuto)

		// Create a test file
		testFile := filepath.Join(tempDir, "auto_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Delete the file
		err = backend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed with MethodAuto: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion with MethodAuto")
		}

		// Verify stats were updated
		stats := backend.GetDeletionStats()
		totalAttempts := stats.FileInfoAttempts + stats.DeleteOnCloseAttempts +
			stats.NtAPIAttempts + stats.FallbackAttempts
		totalSuccesses := stats.FileInfoSuccesses + stats.DeleteOnCloseSuccesses +
			stats.NtAPISuccesses + stats.FallbackSuccesses

		if totalAttempts == 0 {
			t.Error("Expected at least one deletion attempt, got 0")
		}
		if totalSuccesses == 0 {
			t.Error("Expected at least one successful deletion, got 0")
		}
	})

	t.Run("delete with MethodFileInfo", func(t *testing.T) {
		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodFileInfo)

		// Create a test file
		testFile := filepath.Join(tempDir, "fileinfo_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Delete the file
		err = backend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed with MethodFileInfo: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion with MethodFileInfo")
		}

		// Verify stats show FileInfo was used
		stats := backend.GetDeletionStats()
		if stats.FileInfoAttempts == 0 {
			t.Error("Expected FileInfo attempts, got 0")
		}
		if stats.FileInfoSuccesses == 0 {
			t.Error("Expected FileInfo successes, got 0")
		}
	})

	t.Run("delete with MethodDeleteOnClose", func(t *testing.T) {
		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodDeleteOnClose)

		// Create a test file
		testFile := filepath.Join(tempDir, "deleteonclose_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Delete the file
		err = backend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed with MethodDeleteOnClose: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion with MethodDeleteOnClose")
		}

		// Verify stats show DeleteOnClose was used
		stats := backend.GetDeletionStats()
		if stats.DeleteOnCloseAttempts == 0 {
			t.Error("Expected DeleteOnClose attempts, got 0")
		}
		if stats.DeleteOnCloseSuccesses == 0 {
			t.Error("Expected DeleteOnClose successes, got 0")
		}
	})

	t.Run("delete with MethodNtAPI", func(t *testing.T) {
		// Skip if NtDeleteFile is not available
		if !supportsNtDeleteFile() {
			t.Skip("NtDeleteFile is not available on this system")
		}

		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodNtAPI)

		// Create a test file
		testFile := filepath.Join(tempDir, "ntapi_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Delete the file
		err = backend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed with MethodNtAPI: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion with MethodNtAPI")
		}

		// Verify stats show NtAPI was used
		stats := backend.GetDeletionStats()
		if stats.NtAPIAttempts == 0 {
			t.Error("Expected NtAPI attempts, got 0")
		}
		if stats.NtAPISuccesses == 0 {
			t.Error("Expected NtAPI successes, got 0")
		}
	})

	t.Run("delete with MethodDeleteAPI", func(t *testing.T) {
		backend := NewWindowsAdvancedBackend()
		backend.SetDeletionMethod(MethodDeleteAPI)

		// Create a test file
		testFile := filepath.Join(tempDir, "deleteapi_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Delete the file
		err = backend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed with MethodDeleteAPI: %v", err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File still exists after deletion with MethodDeleteAPI")
		}

		// Verify stats show DeleteAPI was used
		stats := backend.GetDeletionStats()
		if stats.FallbackAttempts == 0 {
			t.Error("Expected Fallback attempts, got 0")
		}
		if stats.FallbackSuccesses == 0 {
			t.Error("Expected Fallback successes, got 0")
		}
	})
}

// TestWindowsAdvancedBackend_DeleteDirectory verifies that DeleteDirectory works correctly.
func TestWindowsAdvancedBackend_DeleteDirectory(t *testing.T) {
	tempDir := t.TempDir()
	backend := NewWindowsAdvancedBackend()

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
}

// TestWindowsAdvancedBackend_AutoFallback verifies that MethodAuto correctly
// falls back through the method chain.
func TestWindowsAdvancedBackend_AutoFallback(t *testing.T) {
	tempDir := t.TempDir()
	backend := NewWindowsAdvancedBackend()
	backend.SetDeletionMethod(MethodAuto)

	// Create multiple test files
	numFiles := 10
	for i := 0; i < numFiles; i++ {
		testFile := filepath.Join(tempDir, filepath.Base(t.Name())+"_"+string(rune('a'+i))+".txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}

		// Delete the file
		err = backend.DeleteFile(testFile)
		if err != nil {
			t.Errorf("DeleteFile failed for file %d: %v", i, err)
		}

		// Verify the file is deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Errorf("File %d still exists after deletion", i)
		}
	}

	// Verify stats show at least one method was used successfully
	stats := backend.GetDeletionStats()
	totalSuccesses := stats.FileInfoSuccesses + stats.DeleteOnCloseSuccesses +
		stats.NtAPISuccesses + stats.FallbackSuccesses

	if totalSuccesses != numFiles {
		t.Errorf("Expected %d successful deletions, got %d", numFiles, totalSuccesses)
	}

	// Log which methods were used
	t.Logf("FileInfo: %d attempts, %d successes", stats.FileInfoAttempts, stats.FileInfoSuccesses)
	t.Logf("DeleteOnClose: %d attempts, %d successes", stats.DeleteOnCloseAttempts, stats.DeleteOnCloseSuccesses)
	t.Logf("NtAPI: %d attempts, %d successes", stats.NtAPIAttempts, stats.NtAPISuccesses)
	t.Logf("Fallback: %d attempts, %d successes", stats.FallbackAttempts, stats.FallbackSuccesses)
}

// TestWindowsAdvancedBackend_NonExistentFile verifies error handling for non-existent files.
func TestWindowsAdvancedBackend_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	backend := NewWindowsAdvancedBackend()

	methods := []DeletionMethod{
		MethodAuto,
		MethodFileInfo,
		MethodDeleteOnClose,
		MethodDeleteAPI,
	}

	// Add MethodNtAPI if available
	if supportsNtDeleteFile() {
		methods = append(methods, MethodNtAPI)
	}

	for _, method := range methods {
		t.Run(method.String(), func(t *testing.T) {
			backend.SetDeletionMethod(method)

			// Try to delete a non-existent file
			testFile := filepath.Join(tempDir, "nonexistent_"+method.String()+".txt")
			err := backend.DeleteFile(testFile)

			// Should return an error
			if err == nil {
				t.Errorf("Expected error when deleting non-existent file with %s, got nil", method.String())
			}
		})
	}
}

// TestWindowsAdvancedBackend_InterfaceCompliance verifies that WindowsAdvancedBackend
// implements both Backend and AdvancedBackend interfaces.
func TestWindowsAdvancedBackend_InterfaceCompliance(t *testing.T) {
	var _ Backend = (*WindowsAdvancedBackend)(nil)
	var _ AdvancedBackend = (*WindowsAdvancedBackend)(nil)
}

// TestWindowsAdvancedBackend_ConcurrentAccess verifies that the backend is safe
// for concurrent access (thread-safe statistics).
func TestWindowsAdvancedBackend_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	backend := NewWindowsAdvancedBackend()
	backend.SetDeletionMethod(MethodAuto)

	// Create test files
	numFiles := 20
	testFiles := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		testFile := filepath.Join(tempDir, filepath.Base(t.Name())+"_concurrent_"+string(rune('a'+i))+".txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
		testFiles[i] = testFile
	}

	// Delete files concurrently
	done := make(chan bool, numFiles)
	for _, testFile := range testFiles {
		go func(file string) {
			err := backend.DeleteFile(file)
			if err != nil {
				t.Errorf("DeleteFile failed: %v", err)
			}
			done <- true
		}(testFile)
	}

	// Wait for all deletions to complete
	for i := 0; i < numFiles; i++ {
		<-done
	}

	// Verify all files are deleted
	for i, testFile := range testFiles {
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Errorf("File %d still exists after concurrent deletion", i)
		}
	}

	// Verify stats are consistent (no race conditions)
	stats := backend.GetDeletionStats()
	totalSuccesses := stats.FileInfoSuccesses + stats.DeleteOnCloseSuccesses +
		stats.NtAPISuccesses + stats.FallbackSuccesses

	if totalSuccesses != numFiles {
		t.Errorf("Expected %d successful deletions, got %d (possible race condition)", numFiles, totalSuccesses)
	}
}
