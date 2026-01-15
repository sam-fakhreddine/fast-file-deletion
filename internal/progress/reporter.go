// Package progress provides real-time progress reporting during deletion operations.
// It calculates and displays statistics including deletion rate, elapsed time,
// estimated time remaining, and completion percentage.
package progress

import (
	"fmt"
	"math"
	"time"
)

// Reporter handles real-time progress reporting during deletion operations.
// It tracks deletion progress and calculates statistics like deletion rate,
// elapsed time, and estimated time remaining (ETA).
type Reporter struct {
	totalFiles int       // Total number of files to delete
	totalBytes int64     // Total size of files to delete
	startTime  time.Time // When deletion started
}

// NewReporter creates a new Reporter with the specified total counts.
//
// Parameters:
//   - totalFiles: Total number of files to delete
//   - totalBytes: Total size of files to delete (in bytes)
//
// The reporter uses these values to calculate progress percentages and ETAs.
// The start time is recorded when the reporter is created.
func NewReporter(totalFiles int, totalBytes int64) *Reporter {
	return &Reporter{
		totalFiles: totalFiles,
		totalBytes: totalBytes,
		startTime:  time.Now(),
	}
}

// Update displays the current progress with statistics.
// This method is called after each file deletion to update the progress display.
// It uses \r (carriage return) to overwrite the previous line, creating an
// animated progress display that updates in place.
//
// Parameters:
//   - deletedCount: Number of files deleted so far
//
// The display includes:
//   - Files deleted / total files (percentage)
//   - Current deletion rate (files/second)
//   - Elapsed time
//   - Estimated time remaining (ETA)
func (r *Reporter) Update(deletedCount int) {
	if r.totalFiles == 0 {
		return
	}

	// Calculate elapsed time
	elapsed := time.Since(r.startTime)

	// Calculate deletion rate (files per second)
	rate := r.calculateRate(deletedCount, elapsed)

	// Calculate ETA
	eta := r.calculateETA(deletedCount, rate)

	// Calculate percentage
	percentage := r.calculatePercentage(deletedCount)

	// Format and display progress
	fmt.Printf("\rDeleting: %s / %s files (%.1f%%) | Rate: %s files/sec | Elapsed: %s | ETA: %s",
		formatNumber(deletedCount),
		formatNumber(r.totalFiles),
		percentage,
		formatNumber(int(rate)),
		formatDuration(elapsed),
		formatDuration(eta),
	)
}

// calculateRate calculates the deletion rate in files per second.
// Returns 0 if no time has elapsed to avoid division by zero.
func (r *Reporter) calculateRate(deletedCount int, elapsed time.Duration) float64 {
	if elapsed.Seconds() == 0 {
		return 0
	}
	return float64(deletedCount) / elapsed.Seconds()
}

// calculateETA calculates the estimated time remaining.
// Returns a very large duration if no files have been deleted yet or rate is zero.
// Returns 0 if all files have been deleted.
// Uses the current deletion rate to estimate how long the remaining files will take.
func (r *Reporter) calculateETA(deletedCount int, rate float64) time.Duration {
	if deletedCount == 0 || rate == 0 {
		return time.Duration(math.MaxInt64)
	}

	remaining := r.totalFiles - deletedCount
	if remaining <= 0 {
		return 0
	}

	secondsRemaining := float64(remaining) / rate
	return time.Duration(secondsRemaining) * time.Second
}

// calculatePercentage calculates the completion percentage.
// Returns 0 if totalFiles is 0 to avoid division by zero.
func (r *Reporter) calculatePercentage(deletedCount int) float64 {
	if r.totalFiles == 0 {
		return 0
	}
	return (float64(deletedCount) / float64(r.totalFiles)) * 100
}

// formatNumber formats a number with thousands separators (commas).
// This makes large numbers more readable (e.g., 1,234,567 instead of 1234567).
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Format with commas
	str := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// Finish displays final statistics after deletion completes.
// This method should be called once at the end of the deletion operation.
//
// Parameters:
//   - deletedCount: Number of files successfully deleted
//   - failedCount: Number of files that failed to delete
//   - retainedCount: Number of files retained due to age filtering (0 if no filtering)
//
// The final statistics include:
//   - Total time taken
//   - Average deletion rate
//   - Success/failure counts
//   - Retention statistics (if age filtering was used)
func (r *Reporter) Finish(deletedCount int, failedCount int, retainedCount int) {
	// Print newline to move past the progress line
	fmt.Println()

	// Calculate total time and average rate
	totalTime := time.Since(r.startTime)
	averageRate := r.calculateRate(deletedCount, totalTime)

	// Display final statistics
	fmt.Println("\n=== Deletion Complete ===")
	fmt.Printf("Total time: %s\n", formatDuration(totalTime))
	fmt.Printf("Average rate: %s files/sec\n", formatNumber(int(averageRate)))
	fmt.Printf("Successfully deleted: %s files\n", formatNumber(deletedCount))

	if failedCount > 0 {
		fmt.Printf("Failed to delete: %s files\n", formatNumber(failedCount))
	}

	// Display retention statistics if age filtering was used
	if retainedCount > 0 {
		fmt.Printf("Retained (due to age): %s files\n", formatNumber(retainedCount))
	}

	fmt.Println()
}

// formatDuration formats a duration in a human-readable format.
// Formats as "Xh Ym Zs" for durations with hours, "Ym Zs" for minutes, or "Zs" for seconds.
// Returns "unknown" for very large durations and "0s" for negative durations.
func formatDuration(d time.Duration) string {
	if d >= time.Duration(math.MaxInt64) {
		return "unknown"
	}

	if d < 0 {
		return "0s"
	}

	// Format as hours, minutes, seconds
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}
