//go:build windows

package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"

	"pgregory.net/rapid"

	"github.com/yourusername/fast-file-deletion/internal/backend"
)

// MockUTF16Backend is a mock backend that tracks UTF-16 conversion calls
// to verify that pre-converted UTF-16 paths are reused without re-conversion.
type MockUTF16Backend struct {
	// Track UTF-16 method calls
	utf16FileCallCount      atomic.Int64
	utf16DirectoryCallCount atomic.Int64

	// Track UTF-8 method calls (which require conversion)
	utf8FileCallCount      atomic.Int64
	utf8DirectoryCallCount atomic.Int64

	// Track which paths were deleted
	deletedPaths   []string
	deletedPathsMu sync.Mutex

	// Simulate deletion behavior
	shouldFail bool
}

// NewMockUTF16Backend creates a new mock backend for testing.
func NewMockUTF16Backend() *MockUTF16Backend {
	return &MockUTF16Backend{
		deletedPaths: make([]string, 0),
		shouldFail:   false,
	}
}

// DeleteFile implements the Backend interface (UTF-8 path).
// This method requires UTF-16 conversion internally.
func (m *MockUTF16Backend) DeleteFile(path string) error {
	m.utf8FileCallCount.Add(1)

	if m.shouldFail {
		return fmt.Errorf("mock deletion failed")
	}

	m.deletedPathsMu.Lock()
	m.deletedPaths = append(m.deletedPaths, path)
	m.deletedPathsMu.Unlock()

	return nil
}

// DeleteDirectory implements the Backend interface (UTF-8 path).
// This method requires UTF-16 conversion internally.
func (m *MockUTF16Backend) DeleteDirectory(path string) error {
	m.utf8DirectoryCallCount.Add(1)

	if m.shouldFail {
		return fmt.Errorf("mock deletion failed")
	}

	m.deletedPathsMu.Lock()
	m.deletedPaths = append(m.deletedPaths, path)
	m.deletedPathsMu.Unlock()

	return nil
}

// DeleteFileUTF16 implements the UTF16Backend interface (pre-converted UTF-16 path).
// This method does NOT require UTF-16 conversion - it reuses the pre-converted path.
func (m *MockUTF16Backend) DeleteFileUTF16(pathUTF16 *uint16, originalPath string) error {
	m.utf16FileCallCount.Add(1)

	if m.shouldFail {
		return fmt.Errorf("mock deletion failed")
	}

	m.deletedPathsMu.Lock()
	m.deletedPaths = append(m.deletedPaths, originalPath)
	m.deletedPathsMu.Unlock()

	return nil
}

// DeleteDirectoryUTF16 implements the UTF16Backend interface (pre-converted UTF-16 path).
// This method does NOT require UTF-16 conversion - it reuses the pre-converted path.
func (m *MockUTF16Backend) DeleteDirectoryUTF16(pathUTF16 *uint16, originalPath string) error {
	m.utf16DirectoryCallCount.Add(1)

	if m.shouldFail {
		return fmt.Errorf("mock deletion failed")
	}

	m.deletedPathsMu.Lock()
	m.deletedPaths = append(m.deletedPaths, originalPath)
	m.deletedPathsMu.Unlock()

	return nil
}

// GetStats returns the call counts for verification.
func (m *MockUTF16Backend) GetStats() (utf16Calls, utf8Calls int64) {
	utf16Total := m.utf16FileCallCount.Load() + m.utf16DirectoryCallCount.Load()
	utf8Total := m.utf8FileCallCount.Load() + m.utf8DirectoryCallCount.Load()
	return utf16Total, utf8Total
}

// GetDeletedPaths returns the list of deleted paths.
func (m *MockUTF16Backend) GetDeletedPaths() []string {
	m.deletedPathsMu.Lock()
	defer m.deletedPathsMu.Unlock()

	paths := make([]string, len(m.deletedPaths))
	copy(paths, m.deletedPaths)
	return paths
}

// Feature: windows-performance-optimization, Property 15: UTF-16 reuse without re-conversion
// **Validates: Requirements 5.2, 5.3**
//
// Property: For any path with pre-converted UTF-16 representation, deletion should reuse
// the stored UTF-16 pointer without performing re-conversion or allocating new buffers.
// This is verified by ensuring that when UTF-16 paths are provided to the engine,
// the backend's UTF16 methods are called instead of the UTF-8 methods (which require conversion).
func TestUTF16ReuseWithoutReconversionProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random number of files
		numFiles := rapid.IntRange(5, 20).Draw(rt, "numFiles")

		// Create test files
		files := make([]string, 0, numFiles)
		filesUTF16 := make([]*uint16, 0, numFiles)

		for i := 0; i < numFiles; i++ {
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
			if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}

			// Pre-convert to UTF-16 (simulating scanner behavior)
			utf16Path, err := syscall.UTF16PtrFromString(fileName)
			if err != nil {
				rt.Fatalf("Failed to convert path to UTF-16: %v", err)
			}

			files = append(files, fileName)
			filesUTF16 = append(filesUTF16, utf16Path)
		}

		// Create mock backend that tracks UTF-16 vs UTF-8 method calls
		mockBackend := NewMockUTF16Backend()

		// Create engine with mock backend
		workers := rapid.IntRange(2, 4).Draw(rt, "workers")
		eng := NewEngine(mockBackend, workers, nil)

		// Execute deletion with pre-converted UTF-16 paths
		ctx := context.Background()
		result, err := eng.DeleteWithUTF16(ctx, files, filesUTF16, nil, false)
		if err != nil {
			rt.Fatalf("Deletion failed: %v", err)
		}

		// Property 1: All files should be deleted successfully
		if result.DeletedCount != numFiles {
			rt.Fatalf("Expected %d files deleted, got %d", numFiles, result.DeletedCount)
		}

		if result.FailedCount != 0 {
			rt.Fatalf("Expected 0 failures, got %d", result.FailedCount)
		}

		// Property 2: UTF-16 methods should be called (not UTF-8 methods)
		// This verifies that pre-converted UTF-16 paths are being reused
		utf16Calls, utf8Calls := mockBackend.GetStats()

		if utf16Calls == 0 {
			rt.Fatal("UTF-16 methods were not called - pre-converted paths are not being reused")
		}

		// Property 3: UTF-8 methods should NOT be called when UTF-16 paths are provided
		// If UTF-8 methods are called, it means the engine is performing unnecessary conversions
		if utf8Calls > 0 {
			rt.Fatalf("UTF-8 methods were called %d times - this indicates re-conversion is happening (should be 0)", utf8Calls)
		}

		// Property 4: The number of UTF-16 calls should match the number of files
		// (Each file requires one deletion call)
		if utf16Calls != int64(numFiles) {
			rt.Fatalf("Expected %d UTF-16 calls, got %d", numFiles, utf16Calls)
		}

		// Property 5: All files should be in the deleted paths list
		deletedPaths := mockBackend.GetDeletedPaths()
		if len(deletedPaths) != numFiles {
			rt.Fatalf("Expected %d deleted paths, got %d", numFiles, len(deletedPaths))
		}

		// Verify all files are in the deleted list
		deletedSet := make(map[string]bool)
		for _, path := range deletedPaths {
			deletedSet[path] = true
		}

		for _, file := range files {
			if !deletedSet[file] {
				rt.Fatalf("File %s was not deleted", file)
			}
		}
	})
}

// Feature: windows-performance-optimization, Property 15: UTF-16 reuse without re-conversion
// **Validates: Requirements 5.2, 5.3**
//
// This test verifies that when UTF-16 paths are NOT provided, the engine falls back
// to UTF-8 methods (which perform conversion internally).
func TestUTF16FallbackToUTF8Methods(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random number of files
		numFiles := rapid.IntRange(5, 15).Draw(rt, "numFiles")

		// Create test files
		files := make([]string, 0, numFiles)

		for i := 0; i < numFiles; i++ {
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
			if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}
			files = append(files, fileName)
		}

		// Create mock backend
		mockBackend := NewMockUTF16Backend()

		// Create engine with mock backend
		workers := rapid.IntRange(2, 4).Draw(rt, "workers")
		eng := NewEngine(mockBackend, workers, nil)

		// Execute deletion WITHOUT pre-converted UTF-16 paths (nil)
		ctx := context.Background()
		result, err := eng.DeleteWithUTF16(ctx, files, nil, nil, false)
		if err != nil {
			rt.Fatalf("Deletion failed: %v", err)
		}

		// Property 1: All files should be deleted successfully
		if result.DeletedCount != numFiles {
			rt.Fatalf("Expected %d files deleted, got %d", numFiles, result.DeletedCount)
		}

		// Property 2: UTF-8 methods should be called (since no UTF-16 paths provided)
		utf16Calls, utf8Calls := mockBackend.GetStats()

		if utf8Calls == 0 {
			rt.Fatal("UTF-8 methods were not called when UTF-16 paths were not provided")
		}

		// Property 3: UTF-16 methods should NOT be called when UTF-16 paths are not provided
		if utf16Calls > 0 {
			rt.Fatalf("UTF-16 methods were called %d times when no UTF-16 paths were provided", utf16Calls)
		}

		// Property 4: The number of UTF-8 calls should match the number of files
		if utf8Calls != int64(numFiles) {
			rt.Fatalf("Expected %d UTF-8 calls, got %d", numFiles, utf8Calls)
		}
	})
}

// Feature: windows-performance-optimization, Property 15: UTF-16 reuse without re-conversion
// **Validates: Requirements 5.2, 5.3**
//
// This test verifies UTF-16 reuse with a mix of files and directories.
func TestUTF16ReuseWithDirectories(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate random structure
		numSubdirs := rapid.IntRange(2, 6).Draw(rt, "numSubdirs")
		filesPerSubdir := rapid.IntRange(2, 5).Draw(rt, "filesPerSubdir")

		// Create subdirectories and files
		files := make([]string, 0)
		filesUTF16 := make([]*uint16, 0)

		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(tmpDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdir: %v", err)
			}

			// Create files in subdirectory
			for j := 0; j < filesPerSubdir; j++ {
				fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
				if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}

				utf16Path, err := syscall.UTF16PtrFromString(fileName)
				if err != nil {
					rt.Fatalf("Failed to convert file path to UTF-16: %v", err)
				}

				files = append(files, fileName)
				filesUTF16 = append(filesUTF16, utf16Path)
			}

			// Add subdirectory to deletion list (after files for bottom-up ordering)
			utf16Path, err := syscall.UTF16PtrFromString(subdir)
			if err != nil {
				rt.Fatalf("Failed to convert subdir path to UTF-16: %v", err)
			}

			files = append(files, subdir)
			filesUTF16 = append(filesUTF16, utf16Path)
		}

		// Add root directory at the end
		utf16Root, err := syscall.UTF16PtrFromString(tmpDir)
		if err != nil {
			rt.Fatalf("Failed to convert root path to UTF-16: %v", err)
		}
		files = append(files, tmpDir)
		filesUTF16 = append(filesUTF16, utf16Root)

		// Create mock backend
		mockBackend := NewMockUTF16Backend()

		// Create engine
		workers := rapid.IntRange(2, 4).Draw(rt, "workers")
		eng := NewEngine(mockBackend, workers, nil)

		// Execute deletion with UTF-16 paths
		ctx := context.Background()
		result, err := eng.DeleteWithUTF16(ctx, files, filesUTF16, nil, false)
		if err != nil {
			rt.Fatalf("Deletion failed: %v", err)
		}

		// Property: All items should be deleted using UTF-16 methods
		expectedCount := len(files)
		if result.DeletedCount != expectedCount {
			rt.Fatalf("Expected %d items deleted, got %d", expectedCount, result.DeletedCount)
		}

		utf16Calls, utf8Calls := mockBackend.GetStats()

		// Property: Only UTF-16 methods should be called
		if utf8Calls > 0 {
			rt.Fatalf("UTF-8 methods were called %d times - re-conversion is happening", utf8Calls)
		}

		if utf16Calls != int64(expectedCount) {
			rt.Fatalf("Expected %d UTF-16 calls, got %d", expectedCount, utf16Calls)
		}
	})
}

// Feature: windows-performance-optimization, Property 15: UTF-16 reuse without re-conversion
// **Validates: Requirements 5.2, 5.3**
//
// This test verifies that UTF-16 reuse works correctly in dry-run mode.
func TestUTF16ReuseInDryRunMode(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate random files
		numFiles := rapid.IntRange(5, 15).Draw(rt, "numFiles")

		files := make([]string, 0, numFiles)
		filesUTF16 := make([]*uint16, 0, numFiles)

		for i := 0; i < numFiles; i++ {
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
			if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}

			utf16Path, err := syscall.UTF16PtrFromString(fileName)
			if err != nil {
				rt.Fatalf("Failed to convert path to UTF-16: %v", err)
			}

			files = append(files, fileName)
			filesUTF16 = append(filesUTF16, utf16Path)
		}

		// Create mock backend
		mockBackend := NewMockUTF16Backend()

		// Create engine
		workers := rapid.IntRange(2, 4).Draw(rt, "workers")
		eng := NewEngine(mockBackend, workers, nil)

		// Execute deletion in DRY-RUN mode with UTF-16 paths
		ctx := context.Background()
		result, err := eng.DeleteWithUTF16(ctx, files, filesUTF16, nil, true) // dryRun = true
		if err != nil {
			rt.Fatalf("Dry-run deletion failed: %v", err)
		}

		// Property 1: All files should be "deleted" (counted) in dry-run mode
		if result.DeletedCount != numFiles {
			rt.Fatalf("Expected %d files counted in dry-run, got %d", numFiles, result.DeletedCount)
		}

		// Property 2: No actual backend calls should be made in dry-run mode
		utf16Calls, utf8Calls := mockBackend.GetStats()

		if utf16Calls > 0 || utf8Calls > 0 {
			rt.Fatalf("Backend methods were called in dry-run mode (UTF-16: %d, UTF-8: %d)",
				utf16Calls, utf8Calls)
		}

		// Property 3: Files should still exist after dry-run
		for _, file := range files {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				rt.Fatalf("File %s was deleted in dry-run mode", file)
			}
		}
	})
}

// Feature: windows-performance-optimization, Property 15: UTF-16 reuse without re-conversion
// **Validates: Requirements 5.2, 5.3**
//
// This test verifies that the engine correctly validates UTF-16 array length.
func TestUTF16ArrayLengthValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []string{
		filepath.Join(tmpDir, "file1.txt"),
		filepath.Join(tmpDir, "file2.txt"),
		filepath.Join(tmpDir, "file3.txt"),
	}

	for _, file := range files {
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Create UTF-16 array with WRONG length (intentionally mismatched)
	filesUTF16 := make([]*uint16, 2) // Only 2 elements, but we have 3 files
	for i := 0; i < 2; i++ {
		utf16Path, err := syscall.UTF16PtrFromString(files[i])
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}
		filesUTF16[i] = utf16Path
	}

	// Create mock backend
	mockBackend := NewMockUTF16Backend()

	// Create engine
	eng := NewEngine(mockBackend, 2, nil)

	// Execute deletion with mismatched array lengths
	ctx := context.Background()
	_, err := eng.DeleteWithUTF16(ctx, files, filesUTF16, nil, false)

	// Property: Engine should return an error for mismatched array lengths
	if err == nil {
		t.Fatal("Expected error for mismatched UTF-16 array length, got nil")
	}

	// Verify error message mentions the length mismatch
	expectedErrMsg := "does not match files length"
	if err.Error() == "" || len(err.Error()) == 0 {
		t.Fatalf("Error message is empty")
	}

	// Check if error message contains expected text
	if !contains(err.Error(), "length") {
		t.Errorf("Error message should mention length mismatch, got: %s", err.Error())
	}
}

// contains checks if a string contains a substring (case-insensitive helper).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 findSubstring(s, substr)))
}

// findSubstring is a helper to find substring in string.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MockSkipDoubleCallBackend is a mock backend that tracks whether DeleteFile
// or DeleteDirectory was called, to verify the skip-double-call optimization.
type MockSkipDoubleCallBackend struct {
	// Track method calls per path
	fileCallPaths      []string
	directoryCallPaths []string
	mu                 sync.Mutex
}

// NewMockSkipDoubleCallBackend creates a new mock backend for testing skip-double-call.
func NewMockSkipDoubleCallBackend() *MockSkipDoubleCallBackend {
	return &MockSkipDoubleCallBackend{
		fileCallPaths:      make([]string, 0),
		directoryCallPaths: make([]string, 0),
	}
}

// DeleteFile implements the Backend interface.
func (m *MockSkipDoubleCallBackend) DeleteFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fileCallPaths = append(m.fileCallPaths, path)
	return nil
}

// DeleteDirectory implements the Backend interface.
func (m *MockSkipDoubleCallBackend) DeleteDirectory(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.directoryCallPaths = append(m.directoryCallPaths, path)
	return nil
}

// GetCallCounts returns the number of DeleteFile and DeleteDirectory calls.
func (m *MockSkipDoubleCallBackend) GetCallCounts() (fileCallCount, dirCallCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.fileCallPaths), len(m.directoryCallPaths)
}

// WasFileMethodCalled checks if DeleteFile was called for a specific path.
func (m *MockSkipDoubleCallBackend) WasFileMethodCalled(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.fileCallPaths {
		if p == path {
			return true
		}
	}
	return false
}

// WasDirectoryMethodCalled checks if DeleteDirectory was called for a specific path.
func (m *MockSkipDoubleCallBackend) WasDirectoryMethodCalled(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.directoryCallPaths {
		if p == path {
			return true
		}
	}
	return false
}

// Feature: windows-performance-optimization, Property 14: Skip double-call optimization
// **Validates: Requirements 4.5**
//
// Property: For any path identified as a directory, the backend should call only
// DeleteDirectory without first attempting DeleteFile. This optimization eliminates
// an unnecessary system call, improving performance.
func TestSkipDoubleCallOptimizationProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random number of directories and files
		numDirectories := rapid.IntRange(3, 10).Draw(rt, "numDirectories")
		numFiles := rapid.IntRange(3, 10).Draw(rt, "numFiles")

		// Create test directories
		directories := make([]string, 0, numDirectories)
		for i := 0; i < numDirectories; i++ {
			dirPath := filepath.Join(tmpDir, fmt.Sprintf("dir_%d", i))
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				rt.Fatalf("Failed to create directory: %v", err)
			}
			directories = append(directories, dirPath)
		}

		// Create test files
		files := make([]string, 0, numFiles)
		for i := 0; i < numFiles; i++ {
			filePath := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
			if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}
			files = append(files, filePath)
		}

		// Combine all paths and create isDirectory flags
		allPaths := make([]string, 0, len(files)+len(directories))
		isDirectory := make([]bool, 0, len(files)+len(directories))

		// Add files first (isDirectory = false)
		for _, file := range files {
			allPaths = append(allPaths, file)
			isDirectory = append(isDirectory, false)
		}

		// Add directories (isDirectory = true)
		for _, dir := range directories {
			allPaths = append(allPaths, dir)
			isDirectory = append(isDirectory, true)
		}

		// Create mock backend that tracks method calls
		mockBackend := NewMockSkipDoubleCallBackend()

		// Create engine with mock backend
		workers := rapid.IntRange(2, 4).Draw(rt, "workers")
		eng := NewEngine(mockBackend, workers, nil)

		// Execute deletion with isDirectory flags
		ctx := context.Background()
		result, err := eng.DeleteWithUTF16(ctx, allPaths, nil, isDirectory, false)
		if err != nil {
			rt.Fatalf("Deletion failed: %v", err)
		}

		// Property 1: All items should be deleted successfully
		expectedCount := len(allPaths)
		if result.DeletedCount != expectedCount {
			rt.Fatalf("Expected %d items deleted, got %d", expectedCount, result.DeletedCount)
		}

		if result.FailedCount != 0 {
			rt.Fatalf("Expected 0 failures, got %d", result.FailedCount)
		}

		// Property 2: For paths marked as directories (isDirectory = true),
		// DeleteFile should NOT be called - only DeleteDirectory should be called
		for _, dir := range directories {
			if mockBackend.WasFileMethodCalled(dir) {
				rt.Fatalf("DeleteFile was called for directory %s (optimization violated - should skip DeleteFile)", dir)
			}

			if !mockBackend.WasDirectoryMethodCalled(dir) {
				rt.Fatalf("DeleteDirectory was not called for directory %s", dir)
			}
		}

		// Property 3: For paths marked as files (isDirectory = false),
		// DeleteFile should be called first
		for _, file := range files {
			if !mockBackend.WasFileMethodCalled(file) {
				rt.Fatalf("DeleteFile was not called for file %s", file)
			}
		}

		// Property 4: The total number of backend calls should be optimized
		// Files: 1 call each (DeleteFile)
		// Directories: 1 call each (DeleteDirectory, skipping DeleteFile)
		fileCallCount, dirCallCount := mockBackend.GetCallCounts()

		// Each file should have exactly 1 DeleteFile call
		expectedFileCallCount := len(files)
		if fileCallCount != expectedFileCallCount {
			rt.Fatalf("Expected %d DeleteFile calls, got %d", expectedFileCallCount, fileCallCount)
		}

		// Each directory should have exactly 1 DeleteDirectory call (no DeleteFile attempt)
		expectedDirCallCount := len(directories)
		if dirCallCount != expectedDirCallCount {
			rt.Fatalf("Expected %d DeleteDirectory calls, got %d", expectedDirCallCount, dirCallCount)
		}

		// Property 5: Total backend calls should equal the number of items
		// (proving no double-calls occurred)
		totalBackendCalls := fileCallCount + dirCallCount
		if totalBackendCalls != expectedCount {
			rt.Fatalf("Expected %d total backend calls, got %d (indicates double-calling)", 
				expectedCount, totalBackendCalls)
		}
	})
}

// Feature: windows-performance-optimization, Property 14: Skip double-call optimization
// **Validates: Requirements 4.5**
//
// This test verifies that when isDirectory flags are NOT provided (nil),
// the engine falls back to the traditional behavior of trying DeleteFile first,
// then DeleteDirectory on failure.
func TestSkipDoubleCallFallbackBehavior(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random number of directories
		numDirectories := rapid.IntRange(2, 6).Draw(rt, "numDirectories")

		// Create test directories
		directories := make([]string, 0, numDirectories)
		for i := 0; i < numDirectories; i++ {
			dirPath := filepath.Join(tmpDir, fmt.Sprintf("dir_%d", i))
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				rt.Fatalf("Failed to create directory: %v", err)
			}
			directories = append(directories, dirPath)
		}

		// Create mock backend
		mockBackend := NewMockSkipDoubleCallBackend()

		// Create engine
		workers := rapid.IntRange(2, 4).Draw(rt, "workers")
		eng := NewEngine(mockBackend, workers, nil)

		// Execute deletion WITHOUT isDirectory flags (nil)
		ctx := context.Background()
		result, err := eng.DeleteWithUTF16(ctx, directories, nil, nil, false)
		if err != nil {
			rt.Fatalf("Deletion failed: %v", err)
		}

		// Property 1: All directories should be deleted successfully
		if result.DeletedCount != len(directories) {
			rt.Fatalf("Expected %d directories deleted, got %d", len(directories), result.DeletedCount)
		}

		// Property 2: When isDirectory is not provided, the engine should try
		// DeleteFile first, then DeleteDirectory (traditional fallback behavior)
		fileCallCount, dirCallCount := mockBackend.GetCallCounts()

		// Each directory should have both DeleteFile and DeleteDirectory calls
		// (because DeleteFile fails, then DeleteDirectory succeeds)
		expectedFileCallCount := len(directories)
		expectedDirCallCount := len(directories)

		if fileCallCount != expectedFileCallCount {
			rt.Fatalf("Expected %d DeleteFile calls (fallback behavior), got %d", 
				expectedFileCallCount, fileCallCount)
		}

		if dirCallCount != expectedDirCallCount {
			rt.Fatalf("Expected %d DeleteDirectory calls, got %d", 
				expectedDirCallCount, dirCallCount)
		}

		// Property 3: Total backend calls should be 2x the number of directories
		// (proving double-calling occurred as expected without optimization)
		totalBackendCalls := fileCallCount + dirCallCount
		expectedTotalCalls := len(directories) * 2
		if totalBackendCalls != expectedTotalCalls {
			rt.Fatalf("Expected %d total backend calls (2x for fallback), got %d", 
				expectedTotalCalls, totalBackendCalls)
		}
	})
}

// Feature: windows-performance-optimization, Property 14: Skip double-call optimization
// **Validates: Requirements 4.5**
//
// This test verifies the optimization works correctly in dry-run mode.
func TestSkipDoubleCallOptimizationInDryRun(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate random directories and files
		numDirectories := rapid.IntRange(2, 6).Draw(rt, "numDirectories")
		numFiles := rapid.IntRange(2, 6).Draw(rt, "numFiles")

		// Create test directories
		directories := make([]string, 0, numDirectories)
		for i := 0; i < numDirectories; i++ {
			dirPath := filepath.Join(tmpDir, fmt.Sprintf("dir_%d", i))
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				rt.Fatalf("Failed to create directory: %v", err)
			}
			directories = append(directories, dirPath)
		}

		// Create test files
		files := make([]string, 0, numFiles)
		for i := 0; i < numFiles; i++ {
			filePath := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}
			files = append(files, filePath)
		}

		// Combine paths and create isDirectory flags
		allPaths := make([]string, 0, len(files)+len(directories))
		isDirectory := make([]bool, 0, len(files)+len(directories))

		for _, file := range files {
			allPaths = append(allPaths, file)
			isDirectory = append(isDirectory, false)
		}

		for _, dir := range directories {
			allPaths = append(allPaths, dir)
			isDirectory = append(isDirectory, true)
		}

		// Create mock backend
		mockBackend := NewMockSkipDoubleCallBackend()

		// Create engine
		workers := rapid.IntRange(2, 4).Draw(rt, "workers")
		eng := NewEngine(mockBackend, workers, nil)

		// Execute deletion in DRY-RUN mode with isDirectory flags
		ctx := context.Background()
		result, err := eng.DeleteWithUTF16(ctx, allPaths, nil, isDirectory, true) // dryRun = true
		if err != nil {
			rt.Fatalf("Dry-run deletion failed: %v", err)
		}

		// Property 1: All items should be counted as "deleted" in dry-run
		expectedCount := len(allPaths)
		if result.DeletedCount != expectedCount {
			rt.Fatalf("Expected %d items counted in dry-run, got %d", expectedCount, result.DeletedCount)
		}

		// Property 2: No backend calls should be made in dry-run mode
		fileCallCount, dirCallCount := mockBackend.GetCallCounts()

		if fileCallCount > 0 || dirCallCount > 0 {
			rt.Fatalf("Backend methods were called in dry-run mode (File: %d, Dir: %d)",
				fileCallCount, dirCallCount)
		}

		// Property 3: All paths should still exist after dry-run
		for _, path := range allPaths {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				rt.Fatalf("Path %s was deleted in dry-run mode", path)
			}
		}
	})
}

// Feature: windows-performance-optimization, Property 15: UTF-16 reuse without re-conversion
// **Validates: Requirements 5.2, 5.3**
//
// This test verifies that backend type checking works correctly.
func TestBackendTypeCheckingForUTF16Support(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []string{
		filepath.Join(tmpDir, "file1.txt"),
		filepath.Join(tmpDir, "file2.txt"),
	}

	filesUTF16 := make([]*uint16, len(files))

	for i, file := range files {
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		utf16Path, err := syscall.UTF16PtrFromString(file)
		if err != nil {
			t.Fatalf("Failed to convert path to UTF-16: %v", err)
		}
		filesUTF16[i] = utf16Path
	}

	// Test with UTF16Backend (should use UTF-16 methods)
	t.Run("WithUTF16Backend", func(t *testing.T) {
		mockBackend := NewMockUTF16Backend()
		eng := NewEngine(mockBackend, 2, nil)

		ctx := context.Background()
		result, err := eng.DeleteWithUTF16(ctx, files, filesUTF16, nil, false)
		if err != nil {
			t.Fatalf("Deletion failed: %v", err)
		}

		if result.DeletedCount != len(files) {
			t.Errorf("Expected %d files deleted, got %d", len(files), result.DeletedCount)
		}

		utf16Calls, utf8Calls := mockBackend.GetStats()

		// Should use UTF-16 methods
		if utf16Calls == 0 {
			t.Error("UTF-16 methods were not called with UTF16Backend")
		}

		if utf8Calls > 0 {
			t.Errorf("UTF-8 methods were called %d times with UTF16Backend", utf8Calls)
		}
	})

	// Test with non-UTF16Backend (should fall back to UTF-8 methods)
	t.Run("WithoutUTF16Backend", func(t *testing.T) {
		// Create a backend that does NOT implement UTF16Backend
		// (just implements the basic Backend interface)
		basicBackend := &basicMockBackend{
			deletedPaths: make([]string, 0),
		}

		eng := NewEngine(basicBackend, 2, nil)

		ctx := context.Background()
		result, err := eng.DeleteWithUTF16(ctx, files, filesUTF16, nil, false)
		if err != nil {
			t.Fatalf("Deletion failed: %v", err)
		}

		if result.DeletedCount != len(files) {
			t.Errorf("Expected %d files deleted, got %d", len(files), result.DeletedCount)
		}

		// Should use UTF-8 methods (fallback)
		if basicBackend.utf8CallCount.Load() == 0 {
			t.Error("UTF-8 methods were not called with basic backend")
		}
	})
}

// basicMockBackend is a mock backend that only implements the basic Backend interface
// (does NOT implement UTF16Backend).
type basicMockBackend struct {
	utf8CallCount  atomic.Int64
	deletedPaths   []string
	deletedPathsMu sync.Mutex
}

func (b *basicMockBackend) DeleteFile(path string) error {
	b.utf8CallCount.Add(1)
	b.deletedPathsMu.Lock()
	b.deletedPaths = append(b.deletedPaths, path)
	b.deletedPathsMu.Unlock()
	return nil
}

func (b *basicMockBackend) DeleteDirectory(path string) error {
	b.utf8CallCount.Add(1)
	b.deletedPathsMu.Lock()
	b.deletedPaths = append(b.deletedPaths, path)
	b.deletedPathsMu.Unlock()
	return nil
}
