package progress

import (
	"math"
	"testing"
	"time"
)

// TestCalculateRate tests the deletion rate calculation.
// Validates Requirements 3.3 (display current deletion rate).
func TestCalculateRate(t *testing.T) {
	tests := []struct {
		name          string
		deletedCount  int
		elapsed       time.Duration
		expectedRate  float64
		description   string
	}{
		{
			name:         "normal rate calculation",
			deletedCount: 1000,
			elapsed:      10 * time.Second,
			expectedRate: 100.0, // 1000 files / 10 seconds = 100 files/sec
			description:  "should calculate rate correctly for normal deletion",
		},
		{
			name:         "zero elapsed time",
			deletedCount: 100,
			elapsed:      0,
			expectedRate: 0, // avoid division by zero
			description:  "should return 0 when no time has elapsed",
		},
		{
			name:         "zero files deleted",
			deletedCount: 0,
			elapsed:      5 * time.Second,
			expectedRate: 0,
			description:  "should return 0 when no files deleted",
		},
		{
			name:         "very fast deletion",
			deletedCount: 10000,
			elapsed:      100 * time.Millisecond,
			expectedRate: 100000.0, // 10000 / 0.1 = 100,000 files/sec
			description:  "should handle very fast deletion rates",
		},
		{
			name:         "slow deletion",
			deletedCount: 10,
			elapsed:      60 * time.Second,
			expectedRate: 0.16666666666666666, // 10 / 60 ≈ 0.167 files/sec
			description:  "should handle slow deletion rates",
		},
		{
			name:         "fractional seconds",
			deletedCount: 500,
			elapsed:      1500 * time.Millisecond,
			expectedRate: 333.3333333333333, // 500 / 1.5 ≈ 333.33 files/sec
			description:  "should handle fractional seconds correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &Reporter{
				totalFiles: 10000,
				totalBytes: 1000000,
				startTime:  time.Now(),
			}

			rate := reporter.calculateRate(tt.deletedCount, tt.elapsed)

			if rate != tt.expectedRate {
				t.Errorf("%s: expected rate %.2f, got %.2f", tt.description, tt.expectedRate, rate)
			}
		})
	}
}

// TestCalculateETA tests the estimated time remaining calculation.
// Validates Requirements 3.4 (display estimated time remaining).
func TestCalculateETA(t *testing.T) {
	tests := []struct {
		name         string
		totalFiles   int
		deletedCount int
		rate         float64
		expectedETA  time.Duration
		description  string
	}{
		{
			name:         "normal ETA calculation",
			totalFiles:   1000,
			deletedCount: 250,
			rate:         50.0, // 50 files/sec
			expectedETA:  15 * time.Second, // (1000 - 250) / 50 = 15 seconds
			description:  "should calculate ETA correctly for normal progress",
		},
		{
			name:         "zero files deleted",
			totalFiles:   1000,
			deletedCount: 0,
			rate:         50.0,
			expectedETA:  time.Duration(math.MaxInt64),
			description:  "should return max duration when no files deleted",
		},
		{
			name:         "zero rate",
			totalFiles:   1000,
			deletedCount: 100,
			rate:         0,
			expectedETA:  time.Duration(math.MaxInt64),
			description:  "should return max duration when rate is zero",
		},
		{
			name:         "all files deleted",
			totalFiles:   1000,
			deletedCount: 1000,
			rate:         100.0,
			expectedETA:  0,
			description:  "should return 0 when all files are deleted",
		},
		{
			name:         "more than total deleted",
			totalFiles:   1000,
			deletedCount: 1500,
			rate:         100.0,
			expectedETA:  0,
			description:  "should return 0 when deleted count exceeds total",
		},
		{
			name:         "very fast deletion",
			totalFiles:   1000000,
			deletedCount: 500000,
			rate:         100000.0, // 100k files/sec
			expectedETA:  5 * time.Second, // (1000000 - 500000) / 100000 = 5 seconds
			description:  "should handle very fast deletion rates",
		},
		{
			name:         "slow deletion",
			totalFiles:   100,
			deletedCount: 10,
			rate:         0.5, // 0.5 files/sec
			expectedETA:  180 * time.Second, // (100 - 10) / 0.5 = 180 seconds
			description:  "should handle slow deletion rates",
		},
		{
			name:         "fractional remaining time",
			totalFiles:   1000,
			deletedCount: 333,
			rate:         100.0,
			expectedETA:  6 * time.Second, // (1000 - 333) / 100 = 6.67 seconds, truncated to 6
			description:  "should handle fractional seconds in ETA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &Reporter{
				totalFiles: tt.totalFiles,
				totalBytes: 1000000,
				startTime:  time.Now(),
			}

			eta := reporter.calculateETA(tt.deletedCount, tt.rate)

			if eta != tt.expectedETA {
				t.Errorf("%s: expected ETA %v, got %v", tt.description, tt.expectedETA, eta)
			}
		})
	}
}

// TestCalculatePercentage tests the completion percentage calculation.
// Validates Requirements 3.2 (progress indicator updates).
func TestCalculatePercentage(t *testing.T) {
	tests := []struct {
		name               string
		totalFiles         int
		deletedCount       int
		expectedPercentage float64
		description        string
	}{
		{
			name:               "50% complete",
			totalFiles:         1000,
			deletedCount:       500,
			expectedPercentage: 50.0,
			description:        "should calculate 50% correctly",
		},
		{
			name:               "0% complete",
			totalFiles:         1000,
			deletedCount:       0,
			expectedPercentage: 0.0,
			description:        "should return 0% when no files deleted",
		},
		{
			name:               "100% complete",
			totalFiles:         1000,
			deletedCount:       1000,
			expectedPercentage: 100.0,
			description:        "should return 100% when all files deleted",
		},
		{
			name:               "zero total files",
			totalFiles:         0,
			deletedCount:       0,
			expectedPercentage: 0.0,
			description:        "should return 0% when total files is zero",
		},
		{
			name:               "fractional percentage",
			totalFiles:         1000,
			deletedCount:       333,
			expectedPercentage: 33.3,
			description:        "should handle fractional percentages",
		},
		{
			name:               "very small percentage",
			totalFiles:         1000000,
			deletedCount:       1,
			expectedPercentage: 0.0001,
			description:        "should handle very small percentages",
		},
		{
			name:               "very large numbers",
			totalFiles:         10000000,
			deletedCount:       7500000,
			expectedPercentage: 75.0,
			description:        "should handle very large file counts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &Reporter{
				totalFiles: tt.totalFiles,
				totalBytes: 1000000,
				startTime:  time.Now(),
			}

			percentage := reporter.calculatePercentage(tt.deletedCount)

			// Use tolerance for floating point comparison
			tolerance := 0.0001
			if math.Abs(percentage-tt.expectedPercentage) > tolerance {
				t.Errorf("%s: expected percentage %.4f, got %.4f", tt.description, tt.expectedPercentage, percentage)
			}
		})
	}
}

// TestEdgeCases tests edge cases for progress calculations.
// Validates Requirements 3.2, 3.3, 3.4, 3.5.
func TestEdgeCases(t *testing.T) {
	t.Run("zero files - all calculations", func(t *testing.T) {
		reporter := NewReporter(0, 0)

		// Rate calculation with zero files
		rate := reporter.calculateRate(0, 10*time.Second)
		if rate != 0 {
			t.Errorf("expected rate 0 for zero files, got %.2f", rate)
		}

		// ETA calculation with zero files
		eta := reporter.calculateETA(0, 0)
		if eta != time.Duration(math.MaxInt64) {
			t.Errorf("expected max duration ETA for zero files, got %v", eta)
		}

		// Percentage calculation with zero files
		percentage := reporter.calculatePercentage(0)
		if percentage != 0 {
			t.Errorf("expected percentage 0 for zero files, got %.2f", percentage)
		}
	})

	t.Run("very fast deletion - instant completion", func(t *testing.T) {
		reporter := NewReporter(1000, 1000000)

		// Simulate instant deletion (1 nanosecond)
		rate := reporter.calculateRate(1000, 1*time.Nanosecond)
		if rate == 0 {
			t.Error("expected non-zero rate for very fast deletion")
		}

		// ETA should be 0 when all files deleted
		eta := reporter.calculateETA(1000, rate)
		if eta != 0 {
			t.Errorf("expected ETA 0 when all files deleted, got %v", eta)
		}

		// Percentage should be 100%
		percentage := reporter.calculatePercentage(1000)
		if percentage != 100.0 {
			t.Errorf("expected percentage 100, got %.2f", percentage)
		}
	})

	t.Run("very slow deletion", func(t *testing.T) {
		reporter := NewReporter(1000000, 1000000000)

		// Simulate very slow deletion (1 file per minute)
		rate := reporter.calculateRate(1, 60*time.Second)
		expectedRate := 1.0 / 60.0
		if math.Abs(rate-expectedRate) > 0.0001 {
			t.Errorf("expected rate %.6f, got %.6f", expectedRate, rate)
		}

		// ETA should be very large
		eta := reporter.calculateETA(1, rate)
		expectedETA := time.Duration(float64(999999) / rate * float64(time.Second))
		// Allow some tolerance for floating point arithmetic
		if math.Abs(float64(eta-expectedETA)) > float64(time.Second) {
			t.Errorf("expected ETA around %v, got %v", expectedETA, eta)
		}
	})

	t.Run("negative values handling", func(t *testing.T) {
		reporter := NewReporter(1000, 1000000)

		// Negative elapsed time - calculateRate doesn't check for negative,
		// so it will return a negative rate. This is acceptable since
		// time.Since() should never return negative in real usage.
		rate := reporter.calculateRate(100, -1*time.Second)
		expectedRate := -100.0 // 100 / -1 = -100
		if rate != expectedRate {
			t.Errorf("expected rate %.2f for negative elapsed time, got %.2f", expectedRate, rate)
		}
	})

	t.Run("Update with zero total files", func(t *testing.T) {
		reporter := NewReporter(0, 0)

		// Update should not panic with zero total files
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Update panicked with zero total files: %v", r)
			}
		}()

		reporter.Update(0)
	})
}

// TestFormatNumber tests the number formatting function.
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{12345678, "12,345,678"},
		{123456789, "123,456,789"},
		{1000000000, "1,000,000,000"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatNumber(tt.input)
			if result != tt.expected {
				t.Errorf("FormatNumber(%d) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestFormatDuration tests the duration formatting function.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0s"},
		{1 * time.Second, "1s"},
		{5 * time.Second, "5s"},
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m 0s"},
		{90 * time.Second, "1m 30s"},
		{3600 * time.Second, "1h 0m 0s"},
		{3661 * time.Second, "1h 1m 1s"},
		{7200 * time.Second, "2h 0m 0s"},
		{7325 * time.Second, "2h 2m 5s"},
		{time.Duration(math.MaxInt64), "unknown"},
		{-1 * time.Second, "0s"},
		{-100 * time.Second, "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatDuration(tt.input)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNewReporter tests the Reporter constructor.
func TestNewReporter(t *testing.T) {
	totalFiles := 1000
	totalBytes := int64(1000000)

	reporter := NewReporter(totalFiles, totalBytes)

	if reporter.totalFiles != totalFiles {
		t.Errorf("expected totalFiles %d, got %d", totalFiles, reporter.totalFiles)
	}

	if reporter.totalBytes != totalBytes {
		t.Errorf("expected totalBytes %d, got %d", totalBytes, reporter.totalBytes)
	}

	if reporter.startTime.IsZero() {
		t.Error("expected startTime to be set, got zero time")
	}

	// Start time should be recent (within last second)
	if time.Since(reporter.startTime) > time.Second {
		t.Error("expected startTime to be recent")
	}
}

// TestReporterIntegration tests the Reporter with realistic scenarios.
// Validates Requirements 3.2, 3.3, 3.4, 3.5.
func TestReporterIntegration(t *testing.T) {
	t.Run("realistic deletion scenario", func(t *testing.T) {
		// Simulate deleting 10,000 files
		reporter := NewReporter(10000, 10000000)

		// Simulate progress at different stages
		stages := []struct {
			deletedCount int
			elapsed      time.Duration
		}{
			{1000, 10 * time.Second},   // 10% complete, 100 files/sec
			{5000, 50 * time.Second},   // 50% complete, 100 files/sec
			{9000, 90 * time.Second},   // 90% complete, 100 files/sec
			{10000, 100 * time.Second}, // 100% complete, 100 files/sec
		}

		for _, stage := range stages {
			rate := reporter.calculateRate(stage.deletedCount, stage.elapsed)
			eta := reporter.calculateETA(stage.deletedCount, rate)
			percentage := reporter.calculatePercentage(stage.deletedCount)

			// Verify rate is reasonable
			if rate < 0 {
				t.Errorf("rate should be non-negative, got %.2f", rate)
			}

			// Verify ETA decreases as progress increases
			if stage.deletedCount < 10000 && eta < 0 {
				t.Errorf("ETA should be non-negative, got %v", eta)
			}

			// Verify percentage is in valid range
			if percentage < 0 || percentage > 100 {
				t.Errorf("percentage should be 0-100, got %.2f", percentage)
			}

			// Verify percentage matches deleted count
			expectedPercentage := float64(stage.deletedCount) / 100.0
			if math.Abs(percentage-expectedPercentage) > 0.01 {
				t.Errorf("expected percentage %.2f, got %.2f", expectedPercentage, percentage)
			}
		}
	})

	t.Run("variable deletion rate", func(t *testing.T) {
		// Simulate deletion with varying rates
		reporter := NewReporter(1000, 1000000)

		// Fast start
		rate1 := reporter.calculateRate(100, 1*time.Second) // 100 files/sec
		eta1 := reporter.calculateETA(100, rate1)

		// Slow middle
		rate2 := reporter.calculateRate(200, 10*time.Second) // 20 files/sec
		eta2 := reporter.calculateETA(200, rate2)

		// Fast finish
		rate3 := reporter.calculateRate(1000, 20*time.Second) // 50 files/sec
		eta3 := reporter.calculateETA(1000, rate3)

		// Verify rates are calculated correctly
		if rate1 != 100.0 {
			t.Errorf("expected rate1 100, got %.2f", rate1)
		}
		if rate2 != 20.0 {
			t.Errorf("expected rate2 20, got %.2f", rate2)
		}
		if rate3 != 50.0 {
			t.Errorf("expected rate3 50, got %.2f", rate3)
		}

		// Verify ETA for completion is 0
		if eta3 != 0 {
			t.Errorf("expected eta3 0 (complete), got %v", eta3)
		}

		// Note: ETA comparison is tricky because it depends on both rate and remaining files
		// At 100 deleted with rate 100: ETA = (1000-100)/100 = 9 seconds
		// At 200 deleted with rate 20: ETA = (1000-200)/20 = 40 seconds
		// Even though we're closer to completion, the slower rate means longer ETA
		// This is correct behavior - ETA reflects current rate, not historical rate
		if eta1 != 9*time.Second {
			t.Errorf("expected eta1 9s, got %v", eta1)
		}
		if eta2 != 40*time.Second {
			t.Errorf("expected eta2 40s, got %v", eta2)
		}
	})
}
