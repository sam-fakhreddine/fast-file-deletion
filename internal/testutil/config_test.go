package testutil

import (
	"os"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestGetTestConfig_DefaultsToQuick tests that GetTestConfig defaults to quick mode
// when no environment variables are set.
func TestGetTestConfig_DefaultsToQuick(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("TEST_INTENSITY")
	os.Unsetenv("TEST_QUICK")
	os.Unsetenv("VERBOSE_TESTS")

	config := GetTestConfig()

	if config.Intensity != IntensityQuick {
		t.Errorf("Expected IntensityQuick, got %v", config.Intensity)
	}

	if config.IterationCount != 10 {
		t.Errorf("Expected IterationCount=10, got %d", config.IterationCount)
	}

	if config.MaxFiles != 100 {
		t.Errorf("Expected MaxFiles=100, got %d", config.MaxFiles)
	}

	if config.MaxFileSize != 1024 {
		t.Errorf("Expected MaxFileSize=1024, got %d", config.MaxFileSize)
	}

	if config.MaxDepth != 3 {
		t.Errorf("Expected MaxDepth=3, got %d", config.MaxDepth)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout=30s, got %v", config.Timeout)
	}

	if config.VerboseOutput {
		t.Errorf("Expected VerboseOutput=false, got true")
	}
}

// TestGetTestConfig_QuickMode tests that GetTestConfig correctly parses "quick" mode.
func TestGetTestConfig_QuickMode(t *testing.T) {
	os.Setenv("TEST_INTENSITY", "quick")
	defer os.Unsetenv("TEST_INTENSITY")

	config := GetTestConfig()

	if config.Intensity != IntensityQuick {
		t.Errorf("Expected IntensityQuick, got %v", config.Intensity)
	}

	if config.IterationCount != 10 {
		t.Errorf("Expected IterationCount=10, got %d", config.IterationCount)
	}
}

// TestGetTestConfig_ThoroughMode tests that GetTestConfig correctly parses "thorough" mode.
func TestGetTestConfig_ThoroughMode(t *testing.T) {
	os.Setenv("TEST_INTENSITY", "thorough")
	defer os.Unsetenv("TEST_INTENSITY")

	config := GetTestConfig()

	if config.Intensity != IntensityThorough {
		t.Errorf("Expected IntensityThorough, got %v", config.Intensity)
	}

	if config.IterationCount != 100 {
		t.Errorf("Expected IterationCount=100, got %d", config.IterationCount)
	}

	if config.MaxFiles != 1000 {
		t.Errorf("Expected MaxFiles=1000, got %d", config.MaxFiles)
	}

	if config.MaxFileSize != 10240 {
		t.Errorf("Expected MaxFileSize=10240, got %d", config.MaxFileSize)
	}

	if config.MaxDepth != 5 {
		t.Errorf("Expected MaxDepth=5, got %d", config.MaxDepth)
	}

	if config.Timeout != 5*time.Minute {
		t.Errorf("Expected Timeout=5m, got %v", config.Timeout)
	}
}

// TestGetTestConfig_TestQuickOverride tests that TEST_QUICK overrides TEST_INTENSITY.
func TestGetTestConfig_TestQuickOverride(t *testing.T) {
	os.Setenv("TEST_INTENSITY", "thorough")
	os.Setenv("TEST_QUICK", "1")
	defer os.Unsetenv("TEST_INTENSITY")
	defer os.Unsetenv("TEST_QUICK")

	config := GetTestConfig()

	if config.Intensity != IntensityQuick {
		t.Errorf("Expected TEST_QUICK to override TEST_INTENSITY, got %v", config.Intensity)
	}
}

// TestGetTestConfig_TestQuickTrue tests that TEST_QUICK=true works.
func TestGetTestConfig_TestQuickTrue(t *testing.T) {
	os.Setenv("TEST_QUICK", "true")
	defer os.Unsetenv("TEST_QUICK")

	config := GetTestConfig()

	if config.Intensity != IntensityQuick {
		t.Errorf("Expected IntensityQuick with TEST_QUICK=true, got %v", config.Intensity)
	}
}

// TestGetTestConfig_VerboseOutput tests that VERBOSE_TESTS is correctly parsed.
func TestGetTestConfig_VerboseOutput(t *testing.T) {
	os.Setenv("VERBOSE_TESTS", "1")
	defer os.Unsetenv("VERBOSE_TESTS")

	config := GetTestConfig()

	if !config.VerboseOutput {
		t.Errorf("Expected VerboseOutput=true, got false")
	}
}

// TestGetTestConfig_VerboseOutputTrue tests that VERBOSE_TESTS=true works.
func TestGetTestConfig_VerboseOutputTrue(t *testing.T) {
	os.Setenv("VERBOSE_TESTS", "true")
	defer os.Unsetenv("VERBOSE_TESTS")

	config := GetTestConfig()

	if !config.VerboseOutput {
		t.Errorf("Expected VerboseOutput=true, got false")
	}
}

// TestGetTestConfig_InvalidIntensity tests that invalid intensity values default to quick.
func TestGetTestConfig_InvalidIntensity(t *testing.T) {
	os.Setenv("TEST_INTENSITY", "invalid")
	defer os.Unsetenv("TEST_INTENSITY")

	config := GetTestConfig()

	if config.Intensity != IntensityQuick {
		t.Errorf("Expected invalid intensity to default to IntensityQuick, got %v", config.Intensity)
	}
}

// TestParseIntensity tests the ParseIntensity function.
func TestParseIntensity(t *testing.T) {
	tests := []struct {
		input    string
		expected TestIntensity
	}{
		{"quick", IntensityQuick},
		{"Quick", IntensityQuick},
		{"QUICK", IntensityQuick},
		{"thorough", IntensityThorough},
		{"Thorough", IntensityThorough},
		{"THOROUGH", IntensityThorough},
		{"invalid", IntensityQuick},
		{"", IntensityQuick},
	}

	for _, tt := range tests {
		result := ParseIntensity(tt.input)
		if result != tt.expected {
			t.Errorf("ParseIntensity(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

// TestParseBool tests the ParseBool function.
func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"Yes", true},
		{"YES", true},
		{"0", false},
		{"false", false},
		{"False", false},
		{"no", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		result := ParseBool(tt.input)
		if result != tt.expected {
			t.Errorf("ParseBool(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

// TestTestIntensity_String tests the String method of TestIntensity.
func TestTestIntensity_String(t *testing.T) {
	if IntensityQuick.String() != "quick" {
		t.Errorf("IntensityQuick.String() = %q, expected \"quick\"", IntensityQuick.String())
	}

	if IntensityThorough.String() != "thorough" {
		t.Errorf("IntensityThorough.String() = %q, expected \"thorough\"", IntensityThorough.String())
	}
}

// TestConfigurationParsingIdempotence is a property-based test that validates
// configuration parsing is idempotent.
//
// **Validates: Requirements 1.1, 1.2, 1.3, 1.4**
//
// Property 4: Configuration Parsing Idempotence
// For any valid environment variable configuration, parsing the configuration
// multiple times shall produce identical TestConfig values.
func TestConfigurationParsingIdempotence(t *testing.T) {
	RapidCheck(t, func(rt *rapid.T) {
		// Generate random environment variable values
		testIntensity := rapid.SampledFrom([]string{"", "quick", "thorough", "QUICK", "THOROUGH", "invalid"}).Draw(rt, "testIntensity")
		testQuick := rapid.SampledFrom([]string{"", "0", "1", "true", "false", "TRUE", "FALSE"}).Draw(rt, "testQuick")
		verboseTests := rapid.SampledFrom([]string{"", "0", "1", "true", "false", "TRUE", "FALSE"}).Draw(rt, "verboseTests")

		// Set environment variables
		if testIntensity != "" {
			os.Setenv("TEST_INTENSITY", testIntensity)
		} else {
			os.Unsetenv("TEST_INTENSITY")
		}

		if testQuick != "" {
			os.Setenv("TEST_QUICK", testQuick)
		} else {
			os.Unsetenv("TEST_QUICK")
		}

		if verboseTests != "" {
			os.Setenv("VERBOSE_TESTS", verboseTests)
		} else {
			os.Unsetenv("VERBOSE_TESTS")
		}

		// Parse configuration twice
		config1 := GetTestConfig()
		config2 := GetTestConfig()

		// Verify both configs are identical
		if config1.Intensity != config2.Intensity {
			t.Fatalf("Intensity mismatch: first=%v, second=%v", config1.Intensity, config2.Intensity)
		}

		if config1.IterationCount != config2.IterationCount {
			t.Fatalf("IterationCount mismatch: first=%d, second=%d", config1.IterationCount, config2.IterationCount)
		}

		if config1.MaxFiles != config2.MaxFiles {
			t.Fatalf("MaxFiles mismatch: first=%d, second=%d", config1.MaxFiles, config2.MaxFiles)
		}

		if config1.MaxFileSize != config2.MaxFileSize {
			t.Fatalf("MaxFileSize mismatch: first=%d, second=%d", config1.MaxFileSize, config2.MaxFileSize)
		}

		if config1.MaxDepth != config2.MaxDepth {
			t.Fatalf("MaxDepth mismatch: first=%d, second=%d", config1.MaxDepth, config2.MaxDepth)
		}

		if config1.Timeout != config2.Timeout {
			t.Fatalf("Timeout mismatch: first=%v, second=%v", config1.Timeout, config2.Timeout)
		}

		if config1.VerboseOutput != config2.VerboseOutput {
			t.Fatalf("VerboseOutput mismatch: first=%v, second=%v", config1.VerboseOutput, config2.VerboseOutput)
		}

		// Clean up environment variables
		os.Unsetenv("TEST_INTENSITY")
		os.Unsetenv("TEST_QUICK")
		os.Unsetenv("VERBOSE_TESTS")
	})
}
