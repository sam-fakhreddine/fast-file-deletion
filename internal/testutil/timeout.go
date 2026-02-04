package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// GetTestTimeout returns the appropriate timeout for current test intensity.
// Quick mode: 30 seconds, Thorough mode: 5 minutes.
func GetTestTimeout(config TestConfig) time.Duration {
	return config.Timeout
}

// WithTimeout executes a test function with a timeout based on the current configuration.
// If the test exceeds the timeout, it returns an error and the test is terminated.
// The context is passed to the test function to allow for graceful cancellation.
func WithTimeout(t *testing.T, fn func(ctx context.Context)) error {
	t.Helper()

	config := GetTestConfig()
	timeout := GetTestTimeout(config)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Channel to signal completion
	done := make(chan error, 1)

	// Run test function in goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("test panicked: %v", r)
			}
		}()

		fn(ctx)
		done <- nil
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("test failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("test timed out after %s", timeout)
	}
}

// WithTimeoutAndCleanup executes a test function with a timeout and cleans up test directories on timeout.
// This ensures that test files are removed even when tests timeout.
// The cleanupDirs parameter specifies directories to clean up on timeout.
func WithTimeoutAndCleanup(t *testing.T, cleanupDirs []string, fn func(ctx context.Context)) error {
	t.Helper()

	config := GetTestConfig()
	timeout := GetTestTimeout(config)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Channel to signal completion
	done := make(chan error, 1)

	// Run test function in goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("test panicked: %v", r)
			}
		}()

		fn(ctx)
		done <- nil
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("test failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		// Test timed out - clean up test directories
		for _, dir := range cleanupDirs {
			if cleanupErr := CleanupTestDir(dir); cleanupErr != nil {
				t.Logf("Warning: failed to cleanup directory %s after timeout: %v", dir, cleanupErr)
			}
		}
		return fmt.Errorf("test timed out after %s", timeout)
	}
}

// WithTimeoutT is a convenience wrapper around WithTimeout that fails the test on timeout.
// This is the recommended way to add timeouts to tests.
func WithTimeoutT(t *testing.T, fn func(ctx context.Context)) {
	t.Helper()

	if err := WithTimeout(t, fn); err != nil {
		t.Fatal(err)
	}
}

// WithTimeoutAndCleanupT is a convenience wrapper that fails the test on timeout and cleans up directories.
// This is the recommended way to add timeouts with cleanup to tests.
func WithTimeoutAndCleanupT(t *testing.T, cleanupDirs []string, fn func(ctx context.Context)) {
	t.Helper()

	if err := WithTimeoutAndCleanup(t, cleanupDirs, fn); err != nil {
		t.Fatal(err)
	}
}

// CheckDeadline checks if a test has exceeded its deadline.
// This can be called periodically within long-running tests to check for timeout.
func CheckDeadline(t *testing.T) error {
	t.Helper()

	deadline, ok := t.Deadline()
	if !ok {
		// No deadline set
		return nil
	}

	if time.Now().After(deadline) {
		return fmt.Errorf("test exceeded deadline")
	}

	return nil
}

// RemainingTime returns the time remaining until the test deadline.
// Returns 0 if no deadline is set or if the deadline has passed.
func RemainingTime(t *testing.T) time.Duration {
	t.Helper()

	deadline, ok := t.Deadline()
	if !ok {
		return 0
	}

	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}

	return remaining
}

// ShouldSkipSlowTest returns true if slow tests should be skipped.
// This checks if we're in quick mode and the test is marked as slow.
func ShouldSkipSlowTest(t *testing.T) bool {
	t.Helper()

	config := GetTestConfig()
	return config.Intensity == IntensityQuick && testing.Short()
}

// SkipIfSlow skips the test if we're in quick mode and testing.Short() is true.
// This is useful for tests that are known to be slow.
func SkipIfSlow(t *testing.T, reason string) {
	t.Helper()

	if ShouldSkipSlowTest(t) {
		t.Skipf("Skipping slow test in quick mode: %s", reason)
	}
}

// TimeoutContext creates a context with timeout based on test configuration.
// This is useful for operations that need to respect test timeouts.
func TimeoutContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	config := GetTestConfig()
	return context.WithTimeout(context.Background(), config.Timeout)
}

// TimeoutContextWithFraction creates a context with a fraction of the test timeout.
// For example, fraction=0.5 creates a context with half the test timeout.
// This is useful for operations that should complete well before the test timeout.
func TimeoutContextWithFraction(t *testing.T, fraction float64) (context.Context, context.CancelFunc) {
	t.Helper()

	config := GetTestConfig()
	timeout := time.Duration(float64(config.Timeout) * fraction)
	return context.WithTimeout(context.Background(), timeout)
}
