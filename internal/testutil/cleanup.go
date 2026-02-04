package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// CleanupTestDir removes a test directory and all its contents.
// It attempts to use efficient deletion methods but falls back to standard
// os.RemoveAll if needed. Errors are logged but don't cause test failures.
func CleanupTestDir(dir string) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Already cleaned up
	}

	// Try to remove the directory
	// Note: We use os.RemoveAll here instead of the engine to avoid circular dependencies.
	// The engine package depends on testutil, so we can't import engine here.
	// For test cleanup, os.RemoveAll is sufficient and avoids the circular dependency.
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove directory %s: %w", dir, err)
	}

	return nil
}

// RegisterCleanup registers a cleanup function that removes a test directory.
// The cleanup uses t.Cleanup() for automatic execution and handles errors gracefully.
func RegisterCleanup(t *testing.T, dir string) {
	t.Helper()

	t.Cleanup(func() {
		if err := CleanupTestDir(dir); err != nil {
			// Log error but don't fail the test
			t.Logf("Warning: cleanup failed for %s: %v", dir, err)
		}
	})
}

// CleanupWithTimeout removes a test directory with a timeout.
// This is useful for cleanup operations that might hang.
func CleanupWithTimeout(dir string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Channel to signal completion
	done := make(chan error, 1)

	go func() {
		done <- CleanupTestDir(dir)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("cleanup timed out after %s for directory %s", timeout, dir)
	}
}

// RegisterCleanupWithTimeout registers a cleanup function with a timeout.
// This is useful for tests that might create directories that are difficult to delete.
func RegisterCleanupWithTimeout(t *testing.T, dir string, timeout time.Duration) {
	t.Helper()

	t.Cleanup(func() {
		if err := CleanupWithTimeout(dir, timeout); err != nil {
			t.Logf("Warning: cleanup with timeout failed for %s: %v", dir, err)
		}
	})
}

// CleanupTestDirs removes multiple test directories.
// Errors are collected and returned as a single error.
func CleanupTestDirs(dirs ...string) error {
	var errors []error

	for _, dir := range dirs {
		if err := CleanupTestDir(dir); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup failed for %d directories: %v", len(errors), errors)
	}

	return nil
}

// EnsureCleanup ensures a directory is cleaned up even if the test panics.
// This is a more aggressive cleanup that should be used for critical cleanup operations.
func EnsureCleanup(t *testing.T, dir string) {
	t.Helper()

	// Register cleanup with t.Cleanup (runs even on panic)
	t.Cleanup(func() {
		// Try multiple times if cleanup fails
		maxRetries := 3
		for i := 0; i < maxRetries; i++ {
			if err := CleanupTestDir(dir); err == nil {
				return // Success
			}
			// Wait a bit before retrying
			time.Sleep(100 * time.Millisecond)
		}
		// Log final failure
		t.Logf("Warning: cleanup failed after %d retries for %s", maxRetries, dir)
	})
}

// CountFilesInDir counts the number of files in a directory (non-recursively).
func CountFilesInDir(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			count++
		}
	}

	return count, nil
}

// CountFilesRecursive counts all files in a directory recursively.
func CountFilesRecursive(dir string) (int, error) {
	count := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// VerifyCleanup verifies that a directory has been cleaned up.
// Returns an error if the directory still exists.
func VerifyCleanup(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("directory %s still exists after cleanup", dir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error checking directory %s: %w", dir, err)
	}
	return nil
}

// CreateTempDirWithCleanup creates a temporary directory and registers cleanup.
// This is an alternative to t.TempDir() that uses our cleanup mechanism.
func CreateTempDirWithCleanup(t *testing.T, pattern string) string {
	t.Helper()

	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	RegisterCleanup(t, dir)
	return dir
}
