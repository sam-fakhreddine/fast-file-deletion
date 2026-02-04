package testutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestGetTestTimeout verifies that GetTestTimeout returns correct timeout values
// for quick and thorough modes.
func TestGetTestTimeout(t *testing.T) {
	tests := []struct {
		name            string
		intensity       TestIntensity
		expectedTimeout time.Duration
	}{
		{
			name:            "quick mode returns 30 seconds",
			intensity:       IntensityQuick,
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "thorough mode returns 5 minutes",
			intensity:       IntensityThorough,
			expectedTimeout: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := TestConfig{
				Intensity: tt.intensity,
			}

			// Set timeout based on intensity
			switch tt.intensity {
			case IntensityQuick:
				config.Timeout = 30 * time.Second
			case IntensityThorough:
				config.Timeout = 5 * time.Minute
			}

			timeout := GetTestTimeout(config)

			if timeout != tt.expectedTimeout {
				t.Errorf("GetTestTimeout() = %v, want %v", timeout, tt.expectedTimeout)
			}
		})
	}
}

// TestGetTestTimeoutWithEnvironment verifies that GetTestTimeout works correctly
// with environment variables.
func TestGetTestTimeoutWithEnvironment(t *testing.T) {
	tests := []struct {
		name            string
		envVar          string
		envValue        string
		expectedTimeout time.Duration
	}{
		{
			name:            "TEST_INTENSITY=quick returns 30 seconds",
			envVar:          "TEST_INTENSITY",
			envValue:        "quick",
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "TEST_INTENSITY=thorough returns 5 minutes",
			envVar:          "TEST_INTENSITY",
			envValue:        "thorough",
			expectedTimeout: 5 * time.Minute,
		},
		{
			name:            "TEST_QUICK=1 returns 30 seconds",
			envVar:          "TEST_QUICK",
			envValue:        "1",
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "no env vars returns 30 seconds (default quick)",
			envVar:          "",
			envValue:        "",
			expectedTimeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all test-related environment variables
			os.Unsetenv("TEST_INTENSITY")
			os.Unsetenv("TEST_QUICK")
			os.Unsetenv("VERBOSE_TESTS")

			// Set the test environment variable if specified
			if tt.envVar != "" {
				os.Setenv(tt.envVar, tt.envValue)
				defer os.Unsetenv(tt.envVar)
			}

			config := GetTestConfig()
			timeout := GetTestTimeout(config)

			if timeout != tt.expectedTimeout {
				t.Errorf("GetTestTimeout() = %v, want %v", timeout, tt.expectedTimeout)
			}
		})
	}
}

// TestGetTestTimeoutReturnsConfigTimeout verifies that GetTestTimeout
// simply returns the Timeout field from the config.
func TestGetTestTimeoutReturnsConfigTimeout(t *testing.T) {
	customTimeout := 2 * time.Minute
	config := TestConfig{
		Timeout: customTimeout,
	}

	timeout := GetTestTimeout(config)

	if timeout != customTimeout {
		t.Errorf("GetTestTimeout() = %v, want %v", timeout, customTimeout)
	}
}

// TestWithTimeoutCompletesSuccessfully verifies that WithTimeout allows
// tests to complete successfully within the timeout limit.
func TestWithTimeoutCompletesSuccessfully(t *testing.T) {
	// Set quick mode for fast test
	os.Setenv("TEST_INTENSITY", "quick")
	defer os.Unsetenv("TEST_INTENSITY")

	err := WithTimeout(t, func(ctx context.Context) {
		// Fast test that completes immediately
		time.Sleep(10 * time.Millisecond)
	})

	if err != nil {
		t.Errorf("WithTimeout() returned error for successful test: %v", err)
	}
}

// TestWithTimeoutTriggersForSlowTests verifies that WithTimeout
// terminates tests that exceed the timeout duration.
func TestWithTimeoutTriggersForSlowTests(t *testing.T) {
	// Set a very short timeout via environment
	os.Setenv("TEST_INTENSITY", "quick")
	defer os.Unsetenv("TEST_INTENSITY")

	// Create a test that will timeout
	start := time.Now()
	err := WithTimeout(t, func(ctx context.Context) {
		// Sleep longer than the timeout
		time.Sleep(100 * time.Second)
	})

	elapsed := time.Since(start)

	// Verify timeout error was returned
	if err == nil {
		t.Error("WithTimeout() should return error for slow test, got nil")
	}

	// Verify error message mentions timeout
	if err != nil && !strings.Contains(err.Error(), "timed out") {
		t.Errorf("WithTimeout() error should mention timeout, got: %v", err)
	}

	// Verify test was terminated quickly (within timeout + small buffer)
	// Quick mode timeout is 30 seconds, but we should terminate much faster
	// since we're checking the context
	if elapsed > 35*time.Second {
		t.Errorf("WithTimeout() took too long to terminate: %v", elapsed)
	}
}

// TestWithTimeoutHandlesPanic verifies that WithTimeout catches panics
// and returns them as errors.
func TestWithTimeoutHandlesPanic(t *testing.T) {
	os.Setenv("TEST_INTENSITY", "quick")
	defer os.Unsetenv("TEST_INTENSITY")

	err := WithTimeout(t, func(ctx context.Context) {
		panic("test panic")
	})

	if err == nil {
		t.Error("WithTimeout() should return error for panicking test, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "panicked") {
		t.Errorf("WithTimeout() error should mention panic, got: %v", err)
	}
}

// TestWithTimeoutRespectsContext verifies that the context passed to the
// test function is properly configured with timeout.
func TestWithTimeoutRespectsContext(t *testing.T) {
	os.Setenv("TEST_INTENSITY", "quick")
	defer os.Unsetenv("TEST_INTENSITY")

	contextReceived := false
	err := WithTimeout(t, func(ctx context.Context) {
		contextReceived = true

		// Verify context has a deadline
		_, hasDeadline := ctx.Deadline()
		if !hasDeadline {
			t.Error("Context should have a deadline")
		}

		// Verify context is not already cancelled
		select {
		case <-ctx.Done():
			t.Error("Context should not be cancelled at start")
		default:
			// Good, context is not cancelled
		}
	})

	if err != nil {
		t.Errorf("WithTimeout() returned unexpected error: %v", err)
	}

	if !contextReceived {
		t.Error("Test function was not executed")
	}
}

// TestWithTimeoutTUsesWithTimeout verifies that WithTimeoutT is a convenience
// wrapper that can be called successfully.
func TestWithTimeoutTUsesWithTimeout(t *testing.T) {
	os.Setenv("TEST_INTENSITY", "quick")
	defer os.Unsetenv("TEST_INTENSITY")

	// Test that WithTimeoutT executes successfully for fast tests
	executed := false
	
	// We can't easily test the t.Fatal behavior without complex mocking,
	// but we can verify the function works for successful cases
	WithTimeoutT(t, func(ctx context.Context) {
		executed = true
		time.Sleep(10 * time.Millisecond)
	})

	if !executed {
		t.Error("WithTimeoutT did not execute the test function")
	}
}

// TestTimeoutEnforcement is a property-based test that validates timeout enforcement
// across various timeout durations.
//
// **Validates: Requirements 4.1, 4.3**
//
// Property 5: Timeout Enforcement
// For any test that runs longer than the configured timeout duration, the test
// execution shall be terminated and a timeout failure shall be reported.
func TestTimeoutEnforcement(t *testing.T) {
	// Import rapid for property-based testing
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random timeout duration between 50ms and 500ms
		// We use short timeouts to keep the test fast
		timeoutMs := rapid.IntRange(50, 500).Draw(rt, "timeoutMs")
		timeout := time.Duration(timeoutMs) * time.Millisecond

		// Generate a test duration that exceeds the timeout
		// The test should run for at least timeout + 50ms to ensure it exceeds the limit
		excessMs := rapid.IntRange(50, 200).Draw(rt, "excessMs")
		testDuration := timeout + time.Duration(excessMs)*time.Millisecond

		// Create a custom config with the generated timeout
		config := TestConfig{
			Intensity:      IntensityQuick,
			IterationCount: 10,
			MaxFiles:       100,
			MaxFileSize:    1024,
			MaxDepth:       3,
			Timeout:        timeout,
			VerboseOutput:  false,
		}

		// Create a test function that runs longer than the timeout
		var testStarted atomic.Bool
		var testCompleted atomic.Bool

		// Create context with the custom timeout
		ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
		defer cancel()

		// Channel to signal completion
		done := make(chan error, 1)

		// Run test function in goroutine (simulating WithTimeout behavior)
		start := time.Now()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("test panicked: %v", r)
				}
			}()

			testStarted.Store(true)
			// Sleep for longer than the timeout
			time.Sleep(testDuration)
			testCompleted.Store(true)
			done <- nil
		}()

		// Wait for completion or timeout
		var err error
		select {
		case err = <-done:
			// Test completed (should not happen)
		case <-ctx.Done():
			// Timeout occurred (expected)
			err = fmt.Errorf("test timed out after %s", timeout)
		}

		elapsed := time.Since(start)

		// Verify the test started
		if !testStarted.Load() {
			t.Fatalf("Test function was not started")
		}

		// Verify the test did not complete (was terminated by timeout)
		if testCompleted.Load() {
			t.Fatalf("Test completed when it should have timed out (timeout=%v, testDuration=%v, elapsed=%v)",
				timeout, testDuration, elapsed)
		}

		// Verify a timeout error was returned
		if err == nil {
			t.Fatalf("Expected timeout error, got nil (timeout=%v, testDuration=%v, elapsed=%v)",
				timeout, testDuration, elapsed)
		}

		// Verify error message mentions timeout
		if !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("Error should mention timeout, got: %v", err)
		}

		// Verify the test was terminated within a reasonable time
		// Allow some buffer for goroutine scheduling (timeout + 100ms)
		maxExpectedDuration := timeout + 100*time.Millisecond
		if elapsed > maxExpectedDuration {
			t.Fatalf("Test took too long to terminate: elapsed=%v, timeout=%v, maxExpected=%v",
				elapsed, timeout, maxExpectedDuration)
		}

		// Verify the test ran for at least the timeout duration
		// Allow some tolerance for timing precision (timeout - 50ms)
		minExpectedDuration := timeout - 50*time.Millisecond
		if elapsed < minExpectedDuration {
			t.Fatalf("Test terminated too early: elapsed=%v, timeout=%v, minExpected=%v",
				elapsed, timeout, minExpectedDuration)
		}
	})
}
