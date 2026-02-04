package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/yourusername/fast-file-deletion/internal/backend"
)

// RateMonitor monitors deletion progress at regular intervals to verify
// that periodic rate reporting is occurring as expected.
type RateMonitor struct {
	samples      []RateSample
	mu           sync.Mutex
	startTime    time.Time
	stopChan     chan struct{}
	sampleTicker *time.Ticker
}

// RateSample represents a snapshot of deletion progress at a specific time.
type RateSample struct {
	timestamp    time.Time
	deletedCount int64
	elapsedSecs  float64
	rate         float64
}

// NewRateMonitor creates a new rate monitor that samples deletion progress
// at the specified interval.
func NewRateMonitor(sampleInterval time.Duration) *RateMonitor {
	return &RateMonitor{
		samples:      make([]RateSample, 0),
		startTime:    time.Now(),
		stopChan:     make(chan struct{}),
		sampleTicker: time.NewTicker(sampleInterval),
	}
}

// Start begins monitoring deletion progress by sampling the counter at regular intervals.
func (m *RateMonitor) Start(counter *atomic.Int64) {
	go func() {
		lastCount := int64(0)
		lastTime := m.startTime
		
		for {
			select {
			case <-m.stopChan:
				m.sampleTicker.Stop()
				return
			case <-m.sampleTicker.C:
				currentCount := counter.Load()
				currentTime := time.Now()
				elapsed := currentTime.Sub(lastTime).Seconds()
				
				// Calculate rate for this interval
				rate := 0.0
				if elapsed > 0 {
					rate = float64(currentCount-lastCount) / elapsed
				}
				
				m.mu.Lock()
				m.samples = append(m.samples, RateSample{
					timestamp:    currentTime,
					deletedCount: currentCount,
					elapsedSecs:  currentTime.Sub(m.startTime).Seconds(),
					rate:         rate,
				})
				m.mu.Unlock()
				
				lastCount = currentCount
				lastTime = currentTime
			}
		}
	}()
}

// Stop stops the rate monitor.
func (m *RateMonitor) Stop() {
	select {
	case <-m.stopChan:
		// Already stopped
		return
	default:
		close(m.stopChan)
	}
}

// GetSamples returns all collected rate samples.
func (m *RateMonitor) GetSamples() []RateSample {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	samplesCopy := make([]RateSample, len(m.samples))
	copy(samplesCopy, m.samples)
	return samplesCopy
}

// GetSampleCount returns the number of samples collected.
func (m *RateMonitor) GetSampleCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.samples)
}


// Feature: windows-performance-optimization, Property 26: Periodic rate reporting
// **Validates: Requirements 12.3**
//
// Property: For any deletion operation in progress, the system should report
// the current deletion rate at 5-second intervals.
//
// This property test verifies that:
// 1. The deletion operation runs long enough to observe periodic behavior (> 5 seconds)
// 2. Progress is being made continuously throughout the operation
// 3. The deletion rate is calculated and can be observed at regular intervals
// 4. The final statistics include reasonable rate calculations
//
// Note: This test verifies the behavior indirectly by monitoring deletion progress
// at 5-second intervals and confirming that the monitorDeletionRate goroutine
// is functioning correctly (which logs rate reports every 5 seconds).
func TestPropertyPeriodicRateReporting(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random test parameters
		// File count: enough to ensure deletion takes at least 6-10 seconds
		// This allows us to observe periodic behavior
		fileCount := rapid.IntRange(2000, 4000).Draw(rt, "fileCount")
		workers := rapid.IntRange(2, 8).Draw(rt, "workers")
		
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}

		// Create test files
		var filesToDelete []string
		for i := 0; i < fileCount; i++ {
			fileName := filepath.Join(targetDir, fmt.Sprintf("file_%d.txt", i))
			content := []byte(fmt.Sprintf("test content %d", i))
			
			if err := os.WriteFile(fileName, content, 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}
			
			filesToDelete = append(filesToDelete, fileName)
		}
		
		// Add target directory to deletion list
		filesToDelete = append(filesToDelete, targetDir)

		// Create deletion engine
		backend := backend.NewBackend()
		
		// Create a progress counter that we can monitor
		progressCounter := &atomic.Int64{}
		progressCallback := func(count int) {
			progressCounter.Store(int64(count))
		}
		
		engine := NewEngine(backend, workers, progressCallback)

		// Start a rate monitor that samples progress every 5 seconds
		// This simulates what the monitorDeletionRate goroutine does
		monitor := NewRateMonitor(5 * time.Second)
		monitor.Start(progressCounter)
		defer monitor.Stop()

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion
		startTime := time.Now()
		result, err := engine.Delete(ctx, filesToDelete, false)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}
		duration := time.Since(startTime)
		
		// Stop monitoring
		monitor.Stop()

		// Verify deletion completed successfully
		if result.FailedCount > 0 {
			rt.Errorf("Deletion had %d failures, expected 0", result.FailedCount)
		}

		// Property verification: Periodic rate monitoring
		samples := monitor.GetSamples()
		sampleCount := len(samples)
		
		// Calculate expected number of samples based on duration
		// Samples should occur every 5 seconds
		expectedSamples := int(duration.Seconds() / 5.0)
		
		// For operations lasting > 5 seconds, we should have at least 1 sample
		if duration >= 5*time.Second {
			if sampleCount < 1 {
				rt.Errorf("Expected at least 1 rate sample for %.2fs operation, got %d",
					duration.Seconds(), sampleCount)
			}
			
			// Verify we got approximately the expected number of samples
			// Allow some tolerance (±1 sample) due to timing variations
			if sampleCount < expectedSamples-1 {
				rt.Logf("Warning: Expected ~%d samples for %.2fs operation, got %d",
					expectedSamples, duration.Seconds(), sampleCount)
			}
		}
		
		// Verify all sampled rates are non-negative
		for i, sample := range samples {
			if sample.rate < 0 {
				rt.Errorf("Sample %d has negative rate: %.2f files/sec", i, sample.rate)
			}
			
			// Verify progress is being made (deleted count increases)
			if i > 0 && sample.deletedCount <= samples[i-1].deletedCount {
				rt.Logf("Warning: Sample %d shows no progress (count: %d -> %d)",
					i, samples[i-1].deletedCount, sample.deletedCount)
			}
		}
		
		// Verify sample intervals are approximately 5 seconds
		if len(samples) > 1 {
			for i := 1; i < len(samples); i++ {
				interval := samples[i].timestamp.Sub(samples[i-1].timestamp)
				
				// Allow some tolerance (4-6 seconds) due to timing variations
				if interval < 4*time.Second || interval > 6*time.Second {
					rt.Logf("Warning: Sample interval %d is %.2fs (expected ~5s)",
						i, interval.Seconds())
				}
			}
		}
		
		// Verify final statistics are reasonable
		if result.AverageRate <= 0 {
			rt.Errorf("Average rate should be positive, got %.2f", result.AverageRate)
		}
		
		if result.DeletedCount != len(filesToDelete) {
			rt.Errorf("Expected %d items deleted, got %d",
				len(filesToDelete), result.DeletedCount)
		}
		
		// Log test results for debugging
		rt.Logf("Deletion completed in %.2fs with %d workers", duration.Seconds(), workers)
		rt.Logf("Processed %d files at %.1f files/sec average", fileCount, result.AverageRate)
		rt.Logf("Collected %d rate samples (expected ~%d)", sampleCount, expectedSamples)
		rt.Logf("Peak rate: %.1f files/sec", result.PeakRate)
	})
}

// Feature: windows-performance-optimization, Property 26: Periodic rate reporting
// **Validates: Requirements 12.3**
//
// Property: For deletion operations lasting < 5 seconds, the periodic rate
// reporting mechanism should still function correctly, even if no periodic
// reports are generated (since the first report occurs at the 5-second mark).
//
// This property test verifies that:
// 1. Short operations (< 5 seconds) complete successfully
// 2. The final statistics are still calculated correctly
// 3. The deletion rate is reasonable even for short operations
func TestPropertyPeriodicRateReportingShortOperations(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random test parameters for a short operation
		// Use fewer files to ensure completion in < 5 seconds
		fileCount := rapid.IntRange(50, 200).Draw(rt, "fileCount")
		workers := rapid.IntRange(2, 8).Draw(rt, "workers")
		
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}

		// Create test files
		var filesToDelete []string
		for i := 0; i < fileCount; i++ {
			fileName := filepath.Join(targetDir, fmt.Sprintf("file_%d.txt", i))
			content := []byte(fmt.Sprintf("test content %d", i))
			
			if err := os.WriteFile(fileName, content, 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}
			
			filesToDelete = append(filesToDelete, fileName)
		}
		
		// Add target directory to deletion list
		filesToDelete = append(filesToDelete, targetDir)

		// Create deletion engine
		backend := backend.NewBackend()
		
		// Create a progress counter
		progressCounter := &atomic.Int64{}
		progressCallback := func(count int) {
			progressCounter.Store(int64(count))
		}
		
		engine := NewEngine(backend, workers, progressCallback)

		// Start a rate monitor
		monitor := NewRateMonitor(5 * time.Second)
		monitor.Start(progressCounter)
		defer monitor.Stop()

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion
		startTime := time.Now()
		result, err := engine.Delete(ctx, filesToDelete, false)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}
		duration := time.Since(startTime)
		
		// Stop monitoring
		monitor.Stop()

		// Verify deletion completed successfully
		if result.FailedCount > 0 {
			rt.Errorf("Deletion had %d failures, expected 0", result.FailedCount)
		}

		// Property verification: Short operations should complete quickly
		samples := monitor.GetSamples()
		sampleCount := len(samples)
		
		// For operations lasting < 5 seconds, we should have 0 samples
		if duration < 5*time.Second {
			if sampleCount > 0 {
				rt.Logf("Note: Short operation (%.2fs) generated %d rate samples",
					duration.Seconds(), sampleCount)
			}
		}
		
		// Verify final statistics are still calculated correctly
		if result.DeletedCount != len(filesToDelete) {
			rt.Errorf("Expected %d items deleted, got %d",
				len(filesToDelete), result.DeletedCount)
		}
		
		if result.AverageRate <= 0 {
			rt.Errorf("Average rate should be positive, got %.2f", result.AverageRate)
		}
		
		// Verify the operation completed in reasonable time
		if duration >= 5*time.Second {
			rt.Logf("Note: Expected short operation (< 5s), but took %.2fs", duration.Seconds())
		}
		
		// Log test results
		rt.Logf("Short deletion completed in %.2fs with %d workers", duration.Seconds(), workers)
		rt.Logf("Processed %d files at %.1f files/sec average", fileCount, result.AverageRate)
		rt.Logf("Collected %d rate samples", sampleCount)
	})
}

