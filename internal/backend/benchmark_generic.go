//go:build !windows

package backend

import (
	"fmt"
	"time"
)

// BenchmarkConfig specifies the configuration for benchmark runs.
// On non-Windows platforms, benchmarking is not supported.
type BenchmarkConfig struct {
	Methods    []DeletionMethod
	Iterations int
	TestDir    string
	Workers    int
	BufferSize int
}

// BenchmarkResult contains the timing and performance metrics for a single
// deletion method benchmark run.
// On non-Windows platforms, this is a stub type.
type BenchmarkResult struct {
	Method          DeletionMethod
	FilesPerSecond  float64
	TotalTime       time.Duration
	ScanTime        time.Duration
	QueueTime       time.Duration
	DeleteTime      time.Duration
	SyscallCount    int
	MemoryUsedBytes int64
	FilesDeleted    int
	FilesFailed     int
	ErrorRate       float64
	Stats           *DeletionStats
}

// PercentageImprovement calculates the percentage improvement of this result
// compared to a baseline result.
func (r *BenchmarkResult) PercentageImprovement(baseline *BenchmarkResult) float64 {
	if baseline == nil || baseline.FilesPerSecond == 0 {
		return 0.0
	}
	return ((r.FilesPerSecond - baseline.FilesPerSecond) / baseline.FilesPerSecond) * 100.0
}

// IsSuccessful returns true if the benchmark completed successfully.
func (r *BenchmarkResult) IsSuccessful() bool {
	return r.FilesDeleted > 0 && r.ErrorRate < 5.0 && r.TotalTime > 0
}

// RunBenchmark returns an error on non-Windows platforms.
// Benchmarking is only supported on Windows.
func RunBenchmark(config BenchmarkConfig) ([]BenchmarkResult, error) {
	return nil, fmt.Errorf("benchmarking is only supported on Windows platforms")
}
