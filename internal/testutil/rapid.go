package testutil

import (
	"fmt"
	"os"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// RapidOptions represents the configuration options for rapid property tests.
// Note: rapid v1.2.0 doesn't support passing options to rapid.Check directly.
// Instead, we configure rapid via the RAPID_CHECKS environment variable.
// This struct documents what options we would use if rapid supported them.
type RapidOptions struct {
	// MinCount is the minimum number of test iterations to run
	MinCount int
	
	// Intensity indicates whether this is a quick or thorough test run
	Intensity TestIntensity
}

// GetRapidOptions returns the rapid check options for current test intensity.
// This function documents the iteration counts we use for different test intensities.
//
// Note: rapid v1.2.0 doesn't support variadic options, so these values are applied
// via the RAPID_CHECKS environment variable in GetRapidCheckConfig().
func GetRapidOptions() RapidOptions {
	config := GetTestConfig()
	
	return RapidOptions{
		MinCount:  config.IterationCount,
		Intensity: config.Intensity,
	}
}

// GetRapidCheckConfig returns the rapid check configuration for current test intensity.
// This sets the number of iterations via environment variable that rapid reads.
func GetRapidCheckConfig(t *testing.T) {
	config := GetTestConfig()

	// Set RAPID_CHECKS environment variable to control iteration count
	// rapid.Check reads this environment variable
	os.Setenv("RAPID_CHECKS", fmt.Sprintf("%d", config.IterationCount))

	// Log iteration count if verbose output is enabled
	if config.VerboseOutput {
		t.Logf("Rapid property test configured with %d iterations (intensity: %s)",
			config.IterationCount, config.Intensity)
	}
}

// RapidCheck wraps rapid.Check with configured iteration count, timeout integration, and reporting.
// This is the recommended way to run property tests in this project.
//
// Timeout Integration:
// - Checks if test has a deadline set (via -timeout flag or t.Deadline())
// - Warns if no timeout is configured (tests should always have timeouts)
// - rapid.Check will automatically respect Go's test timeout mechanism
// - Reports iteration count only in verbose mode to reduce output
func RapidCheck(t *testing.T, fn func(*rapid.T)) {
	t.Helper()
	
	config := GetTestConfig()

	// Configure rapid iteration count
	GetRapidCheckConfig(t)

	// Check timeout integration - ensure test has a deadline
	deadline, hasDeadline := t.Deadline()
	if !hasDeadline {
		// No deadline set - this is a warning condition
		// Tests should be run with -timeout flag to prevent infinite loops
		if config.VerboseOutput {
			t.Logf("WARNING: No test deadline set. Run tests with -timeout flag for safety.")
		}
	} else {
		// Calculate remaining time until deadline
		remaining := time.Until(deadline)
		if config.VerboseOutput {
			t.Logf("Property test starting with %d iterations (timeout: %s remaining)", 
				config.IterationCount, remaining.Round(time.Second))
		}
		
		// Warn if remaining time is less than configured timeout
		if remaining < config.Timeout {
			t.Logf("WARNING: Remaining test time (%s) is less than configured timeout (%s)", 
				remaining.Round(time.Second), config.Timeout)
		}
	}

	// Log start of property test if verbose (and not already logged above)
	if config.VerboseOutput && !hasDeadline {
		t.Logf("Starting property test with %d iterations", config.IterationCount)
	}

	// Run the property test
	// rapid.Check automatically respects t.Deadline() and will stop if timeout is reached
	rapid.Check(t, fn)

	// Log completion if verbose
	if config.VerboseOutput {
		t.Logf("Property test completed successfully (%d iterations)", config.IterationCount)
	}
}

// RapidFileCountGenerator returns a rapid generator for file counts within config limits.
// The generator produces values in the range [1, MaxFiles].
func RapidFileCountGenerator(config TestConfig) *rapid.Generator[int] {
	if config.MaxFiles <= 1 {
		return rapid.Just(1)
	}
	return rapid.IntRange(1, config.MaxFiles)
}

// RapidFileSizeGenerator returns a rapid generator for file sizes within config limits.
// The generator produces values in the range [1, MaxFileSize].
func RapidFileSizeGenerator(config TestConfig) *rapid.Generator[int64] {
	if config.MaxFileSize <= 1 {
		return rapid.Just(int64(1))
	}
	return rapid.Int64Range(1, config.MaxFileSize)
}

// RapidDepthGenerator returns a rapid generator for directory depths within config limits.
// The generator produces values in the range [0, MaxDepth].
func RapidDepthGenerator(config TestConfig) *rapid.Generator[int] {
	if config.MaxDepth <= 0 {
		return rapid.Just(0)
	}
	return rapid.IntRange(0, config.MaxDepth)
}

// LogIterationProgress logs progress during property test execution.
// This should be called periodically (e.g., every 10 iterations) when verbose output is enabled.
func LogIterationProgress(t *testing.T, iteration int) {
	config := GetTestConfig()
	if config.VerboseOutput && iteration%10 == 0 {
		t.Logf("Property test progress: iteration %d/%d", iteration, config.IterationCount)
	}
}

// RapidIntensityGenerator returns a generator for TestIntensity values.
// This is useful for testing the configuration system itself.
func RapidIntensityGenerator(t *rapid.T) TestIntensity {
	if rapid.Bool().Draw(t, "intensity") {
		return IntensityThorough
	}
	return IntensityQuick
}

// RapidConfigGenerator generates random TestConfig values for property testing.
// This is useful for property testing the configuration system.
func RapidConfigGenerator(t *rapid.T) TestConfig {
	intensity := RapidIntensityGenerator(t)

	var config TestConfig
	config.Intensity = intensity

	// Generate values appropriate for the intensity
	if intensity == IntensityQuick {
		config.IterationCount = rapid.IntRange(5, 20).Draw(t, "iterationCount")
		config.MaxFiles = rapid.IntRange(10, 200).Draw(t, "maxFiles")
		config.MaxFileSize = rapid.Int64Range(1, 2048).Draw(t, "maxFileSize")
		config.MaxDepth = rapid.IntRange(1, 5).Draw(t, "maxDepth")
		timeoutSeconds := rapid.IntRange(10, 60).Draw(t, "timeoutSeconds")
		config.Timeout = time.Duration(timeoutSeconds) * time.Second
	} else {
		config.IterationCount = rapid.IntRange(50, 200).Draw(t, "iterationCount")
		config.MaxFiles = rapid.IntRange(500, 2000).Draw(t, "maxFiles")
		config.MaxFileSize = rapid.Int64Range(1024, 20480).Draw(t, "maxFileSize")
		config.MaxDepth = rapid.IntRange(3, 10).Draw(t, "maxDepth")
		timeoutSeconds := rapid.IntRange(60, 600).Draw(t, "timeoutSeconds")
		config.Timeout = time.Duration(timeoutSeconds) * time.Second
	}

	config.VerboseOutput = rapid.Bool().Draw(t, "verboseOutput")

	return config
}
