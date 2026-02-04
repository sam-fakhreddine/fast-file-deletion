// Package testutil provides utilities for configuring and running tests
// with different intensity levels and resource constraints.
package testutil

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// TestIntensity represents the thoroughness level of test execution.
type TestIntensity int

const (
	// IntensityQuick runs tests with minimal resources for fast feedback during development.
	IntensityQuick TestIntensity = iota
	// IntensityThorough runs tests with comprehensive resources for thorough validation in CI.
	IntensityThorough
)

// String returns the string representation of the test intensity.
func (ti TestIntensity) String() string {
	switch ti {
	case IntensityQuick:
		return "quick"
	case IntensityThorough:
		return "thorough"
	default:
		return "unknown"
	}
}

// TestConfig holds configuration parameters for test execution.
type TestConfig struct {
	// Intensity level (quick or thorough)
	Intensity TestIntensity

	// Number of iterations for property tests
	IterationCount int

	// Maximum number of files to create in test cases
	MaxFiles int

	// Maximum size of individual test files (bytes)
	MaxFileSize int64

	// Maximum directory depth for nested structures
	MaxDepth int

	// Timeout duration for individual tests
	Timeout time.Duration

	// Enable verbose test output
	VerboseOutput bool
}

// GetTestConfig returns the current test configuration based on environment variables.
// It reads TEST_INTENSITY, TEST_QUICK, and VERBOSE_TESTS environment variables.
// Defaults to quick mode if no environment variables are set.
func GetTestConfig() TestConfig {
	config := TestConfig{}

	// Check TEST_QUICK override first (takes precedence)
	testQuick := os.Getenv("TEST_QUICK")
	if testQuick == "1" || strings.ToLower(testQuick) == "true" {
		config.Intensity = IntensityQuick
	} else {
		// Check TEST_INTENSITY
		intensity := strings.ToLower(os.Getenv("TEST_INTENSITY"))
		switch intensity {
		case "thorough":
			config.Intensity = IntensityThorough
		case "quick":
			config.Intensity = IntensityQuick
		default:
			// Default to quick mode
			config.Intensity = IntensityQuick
		}
	}

	// Set parameters based on intensity
	switch config.Intensity {
	case IntensityQuick:
		config.IterationCount = 10
		config.MaxFiles = 100
		config.MaxFileSize = 1024 // 1KB
		config.MaxDepth = 3
		config.Timeout = 30 * time.Second
	case IntensityThorough:
		config.IterationCount = 100
		config.MaxFiles = 1000
		config.MaxFileSize = 10240 // 10KB
		config.MaxDepth = 5
		config.Timeout = 5 * time.Minute
	}

	// Check verbose output setting
	verboseTests := os.Getenv("VERBOSE_TESTS")
	config.VerboseOutput = verboseTests == "1" || strings.ToLower(verboseTests) == "true"

	return config
}

// LogConfig logs the current test configuration.
// This is called at package initialization to inform users of the active test mode.
func LogConfig(config TestConfig) {
	fmt.Printf("Test Configuration: intensity=%s, iterations=%d, maxFiles=%d, maxFileSize=%d, maxDepth=%d, timeout=%s, verbose=%v\n",
		config.Intensity,
		config.IterationCount,
		config.MaxFiles,
		config.MaxFileSize,
		config.MaxDepth,
		config.Timeout,
		config.VerboseOutput,
	)
}

// ParseIntensity parses a string into a TestIntensity value.
// Returns IntensityQuick for invalid or empty strings.
func ParseIntensity(s string) TestIntensity {
	switch strings.ToLower(s) {
	case "thorough":
		return IntensityThorough
	case "quick":
		return IntensityQuick
	default:
		return IntensityQuick
	}
}

// ParseBool parses a string into a boolean value.
// Accepts "1", "true", "yes" (case-insensitive) as true.
func ParseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "1" || s == "true" || s == "yes" {
		return true
	}
	if i, err := strconv.Atoi(s); err == nil && i != 0 {
		return true
	}
	return false
}

// init logs the test configuration at package initialization.
var testConfig TestConfig

func init() {
	testConfig = GetTestConfig()
	
	// Always log the active test intensity mode (Requirement 1.5)
	fmt.Printf("Test intensity mode: %s\n", testConfig.Intensity)
	
	// Log full configuration values when VERBOSE_TESTS is enabled
	if testConfig.VerboseOutput {
		LogConfig(testConfig)
	}
}
