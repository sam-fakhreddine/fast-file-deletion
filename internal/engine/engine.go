// Package engine provides the core deletion engine with parallel processing
// using goroutines for high-performance file deletion.
package engine

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/yourusername/fast-file-deletion/internal/backend"
	"github.com/yourusername/fast-file-deletion/internal/logger"
)

// Configuration constants for the deletion engine.
const (
	// DefaultWorkerMultiplier is multiplied by NumCPU to determine default worker count.
	DefaultWorkerMultiplier = 4

	// MaxAutoBufferSize is the maximum auto-detected buffer size for the work channel.
	MaxAutoBufferSize = 10000

	// BatchThreshold is the file count above which batch processing is used.
	BatchThreshold = 100000

	// BatchSize is the number of files per batch during batch processing.
	BatchSize = 30000

	// SlidingWindowThreshold is the fraction of a batch that must complete
	// before the next batch begins (0.8 = 80%).
	SlidingWindowThreshold = 0.8
)

// Engine manages parallel file deletion using goroutines.
// It coordinates multiple worker goroutines that process deletion tasks
// concurrently, providing significant performance improvements over
// sequential deletion, especially for large numbers of files.
type Engine struct {
	backend          backend.Backend
	workers          int
	bufferSize       int // Custom buffer size (0 = auto-detect)
	progressCallback func(int)

	// Live counters accessible during deletion for external monitoring.
	liveCounters atomicCounters
	startTime    atomic.Value // stores time.Time
}

// workItem represents a file or directory to delete with optional UTF-16 path.
type workItem struct {
	pathUTF8    string   // UTF-8 path (always present)
	pathUTF16   *uint16  // Optional pre-converted UTF-16 path
	isDirectory bool     // True if this is a directory (skip DeleteFile attempt)
}

// atomicCounters provides lock-free counters for deletion statistics.
// Using atomic operations eliminates mutex contention when multiple workers
// update statistics concurrently, improving performance.
type atomicCounters struct {
	deleted atomic.Int64 // Number of files successfully deleted
	failed  atomic.Int64 // Number of files that failed to delete
}

// DeletionResult contains statistics and errors from a deletion operation.
// It provides comprehensive information about the deletion process including
// success/failure counts, detailed error information, and timing data.
type DeletionResult struct {
	DeletedCount    int         // Number of files successfully deleted
	FailedCount     int         // Number of files that failed to delete
	Errors          []FileError // List of errors encountered during deletion
	DurationSeconds float64     // Total time taken for deletion
	PeakRate        float64     // Peak deletion rate in files/sec
	AverageRate     float64     // Average deletion rate in files/sec
}

// FileError represents an error that occurred while deleting a specific file.
// This allows tracking which files failed and why, enabling detailed error reporting.
type FileError struct {
	Path  string // Path to the file that failed
	Error string // Error message
}

// NewEngine creates a new deletion engine with the specified backend and worker count.
//
// Parameters:
//   - backend: The platform-specific deletion backend to use
//   - workers: Number of parallel worker goroutines (0 or negative = auto-detect)
//   - progressCallback: Function called after each file deletion with current count
//
// Worker count auto-detection:
// If workers is 0 or negative, the engine automatically detects the optimal
// count based on CPU cores using the formula: NumCPU * 4. This provides optimal
// parallelism for I/O-bound deletion operations without overwhelming the system.
//
// Returns a configured Engine ready to perform parallel deletion.
//
// Validates Requirements: 4.2
func NewEngine(backend backend.Backend, workers int, progressCallback func(int)) *Engine {
	return NewEngineWithBufferSize(backend, workers, 0, progressCallback)
}

// NewEngineWithBufferSize creates a new deletion engine with custom buffer size.
//
// Parameters:
//   - backend: The platform-specific deletion backend to use
//   - workers: Number of parallel worker goroutines (0 or negative = auto-detect)
//   - bufferSize: Work queue buffer size (0 = auto-detect)
//   - progressCallback: Function called after each file deletion with current count
//
// Worker count auto-detection:
// If workers is 0 or negative, the engine automatically detects the optimal
// count based on CPU cores using the formula: NumCPU * 4. This provides optimal
// parallelism for I/O-bound deletion operations without overwhelming the system.
//
// Buffer size auto-detection:
// If bufferSize is 0, the engine automatically calculates the buffer size as
// min(fileCount, 10000) during deletion. If bufferSize is positive, that value
// is used directly.
//
// Returns a configured Engine ready to perform parallel deletion.
//
// Validates Requirements: 4.2, 11.2
func NewEngineWithBufferSize(backend backend.Backend, workers int, bufferSize int, progressCallback func(int)) *Engine {
	// Auto-detect optimal worker count if not specified
	if workers <= 0 {
		workers = runtime.NumCPU() * DefaultWorkerMultiplier
	}

	return &Engine{
		backend:          backend,
		workers:          workers,
		bufferSize:       bufferSize,
		progressCallback: progressCallback,
	}
}

// FilesDeleted returns the current count of successfully deleted files.
// This is safe to call concurrently during deletion for live monitoring.
func (e *Engine) FilesDeleted() int {
	return int(e.liveCounters.deleted.Load())
}

// DeletionRate returns the current deletion rate in files/sec.
// This is safe to call concurrently during deletion for live monitoring.
// Returns 0 if deletion has not started.
func (e *Engine) DeletionRate() float64 {
	v := e.startTime.Load()
	if v == nil {
		return 0
	}
	start := v.(time.Time)
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(e.liveCounters.deleted.Load()) / elapsed
}

// Delete deletes the specified files using parallel goroutines.
//
// The deletion process uses a worker pool pattern with depth-based batching:
//  1. Groups files by directory depth to ensure children are deleted before parents
//  2. Creates a buffered channel for work distribution
//  3. Starts multiple worker goroutines that process files concurrently
//  4. Processes files level by level (deepest first) to avoid "directory not empty" errors
//  5. Collects results and errors in a thread-safe manner
//  6. Supports graceful cancellation via context
//
// Parameters:
//   - ctx: Context for cancellation support (use SetupInterruptHandler for Ctrl+C handling)
//   - files: List of file paths to delete (should be in bottom-up order)
//   - dryRun: If true, simulates deletion without actually deleting files
//
// Returns DeletionResult with statistics and any errors encountered.
func (e *Engine) Delete(ctx context.Context, files []string, dryRun bool) (*DeletionResult, error) {
	return e.DeleteWithUTF16(ctx, files, nil, nil, dryRun)
}

// DeleteWithUTF16 deletes the specified files using parallel goroutines with optional
// pre-converted UTF-16 paths for performance optimization.
//
// This method is identical to Delete() but accepts pre-converted UTF-16 paths to avoid
// repeated UTF-16 conversions during deletion. If filesUTF16 is provided and the backend
// implements UTF16Backend, the UTF-16 paths will be used directly. Otherwise, it falls
// back to standard UTF-8 path conversion.
//
// The deletion process uses a worker pool pattern with depth-based batching:
//  1. Groups files by directory depth to ensure children are deleted before parents
//  2. Creates a buffered channel for work distribution
//  3. Starts multiple worker goroutines that process files concurrently
//  4. Processes files level by level (deepest first) to avoid "directory not empty" errors
//  5. Processes files in batches to release memory from completed batches
//  6. Collects results and errors in a thread-safe manner
//  7. Supports graceful cancellation via context
//
// Parameters:
//   - ctx: Context for cancellation support (use SetupInterruptHandler for Ctrl+C handling)
//   - files: List of file paths to delete (UTF-8, should be in bottom-up order)
//   - filesUTF16: Optional pre-converted UTF-16 paths (must match files array length)
//   - isDirectory: Optional flags indicating if each path is a directory (must match files array length)
//   - dryRun: If true, simulates deletion without actually deleting files
//
// Returns DeletionResult with statistics and any errors encountered.
//
// Validates Requirements: 4.5, 5.2, 5.3, 5.5
func (e *Engine) DeleteWithUTF16(ctx context.Context, files []string, filesUTF16 []*uint16, isDirectory []bool, dryRun bool) (*DeletionResult, error) {
	startTime := time.Now()
	e.startTime.Store(startTime)
	// Reset live counters for this deletion run
	e.liveCounters.deleted.Store(0)
	e.liveCounters.failed.Store(0)

	logger.Info("Starting deletion of %d files with %d workers", len(files), e.workers)
	if dryRun {
		logger.Info("Running in DRY-RUN mode - no files will be deleted")
	}

	// Validate that filesUTF16 matches files length if provided
	if filesUTF16 != nil && len(filesUTF16) != len(files) {
		return nil, fmt.Errorf("filesUTF16 length (%d) does not match files length (%d)", len(filesUTF16), len(files))
	}

	// Validate that isDirectory matches files length if provided
	if isDirectory != nil && len(isDirectory) != len(files) {
		return nil, fmt.Errorf("isDirectory length (%d) does not match files length (%d)", len(isDirectory), len(files))
	}

	// Check if backend supports UTF-16 optimization
	utf16Backend, supportsUTF16 := e.backend.(backend.UTF16Backend)
	if filesUTF16 != nil && supportsUTF16 {
		logger.Debug("Using UTF-16 pre-converted paths for deletion")
	}

	result := &DeletionResult{
		Errors: make([]FileError, 0),
	}

	// Use the engine's live counters for thread-safe statistics tracking
	counters := &e.liveCounters

	// Mutex only for thread-safe access to error slice
	var errorsMu sync.Mutex

	// Create buffered channel for work distribution
	// Use dynamic buffer size: min(fileCount, 10000) to balance memory usage and performance
	// If custom buffer size is specified, use that instead
	// Validates Requirements: 4.3, 5.4, 11.2
	bufferSize := e.bufferSize
	if bufferSize <= 0 {
		bufferSize = len(files)
		if bufferSize > MaxAutoBufferSize {
			bufferSize = MaxAutoBufferSize
		}
	}
	workChan := make(chan workItem, bufferSize)

	// WaitGroup to track worker completion
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < e.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.workerWithUTF16(ctx, workChan, dryRun, result, counters, &errorsMu, utf16Backend, supportsUTF16)
		}()
	}

	// Start rate monitoring goroutine that tracks deletion performance
	// and records peak rate every 5 seconds
	// Validates Requirements: 4.4, 12.3
	peakRateChan := make(chan float64, 1)
	go e.monitorDeletionRate(ctx, counters, peakRateChan)

	// Process files in batches to manage memory usage
	// For very large file sets (millions of files), processing in batches allows
	// memory from completed batches to be released before processing subsequent batches
	// Validates Requirements: 5.5
	err := e.processBatches(ctx, files, filesUTF16, isDirectory, workChan, counters)
	if err != nil {
		close(workChan)
		wg.Wait()
		return nil, err
	}

	// Close work channel to signal workers to stop
	close(workChan)

	// Wait for all workers to complete
	wg.Wait()

	// Copy atomic counter values to result
	result.DeletedCount = int(counters.deleted.Load())
	result.FailedCount = int(counters.failed.Load())

	// Calculate duration and rates
	result.DurationSeconds = time.Since(startTime).Seconds()
	
	// Calculate average rate
	if result.DurationSeconds > 0 {
		result.AverageRate = float64(result.DeletedCount) / result.DurationSeconds
	}
	
	// Get peak rate from adaptive tuning goroutine
	select {
	case peakRate := <-peakRateChan:
		result.PeakRate = peakRate
	default:
		// If no peak rate available, use average rate
		result.PeakRate = result.AverageRate
	}

	logger.Info("Deletion completed: %d succeeded, %d failed in %.2f seconds",
		result.DeletedCount, result.FailedCount, result.DurationSeconds)

	return result, nil
}

// processBatches groups files by depth and processes them level by level (deepest first)
// to ensure children are deleted before parents. For large file sets (>= 100k files),
// each depth level is processed in batches to limit memory usage.
//
// Validates Requirements: 5.5
func (e *Engine) processBatches(ctx context.Context, files []string, filesUTF16 []*uint16, isDirectory []bool, workChan chan<- workItem, counters *atomicCounters) error {

	// Build the depth map once: group file indices by directory depth
	depthMap := make(map[int][]int) // depth -> list of file indices
	maxDepth := 0

	for i, file := range files {
		depth := countPathSeparators(file)
		depthMap[depth] = append(depthMap[depth], i)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	totalFiles := len(files)
	useBatching := totalFiles >= BatchThreshold

	if useBatching {
		logger.Info("Processing %d files in batches of %d to manage memory usage", totalFiles, BatchSize)
	} else {
		logger.Debug("Processing all %d files in single pass", totalFiles)
	}

	// Process files level by level, starting from deepest
	for depth := maxDepth; depth >= 0; depth-- {
		indices := depthMap[depth]
		if len(indices) == 0 {
			continue
		}

		logger.Debug("Processing depth %d: %d files", depth, len(indices))

		if useBatching {
			// Process in batches for large file sets
			if err := e.processIndicesInBatches(ctx, files, filesUTF16, isDirectory, indices, BatchSize, workChan, counters); err != nil {
				return err
			}
			runtime.GC()
			logger.Debug("Released memory after processing depth %d", depth)
		} else {
			// Send all items at this depth, then wait for completion
			if err := e.processIndicesAndWait(ctx, files, filesUTF16, isDirectory, indices, workChan, counters); err != nil {
				return err
			}
		}
	}

	return nil
}

// makeWorkItem creates a workItem from the file arrays at the given index.
func makeWorkItem(files []string, filesUTF16 []*uint16, isDirectory []bool, i int) workItem {
	item := workItem{pathUTF8: files[i]}
	if filesUTF16 != nil && i < len(filesUTF16) {
		item.pathUTF16 = filesUTF16[i]
	}
	if isDirectory != nil && i < len(isDirectory) {
		item.isDirectory = isDirectory[i]
	}
	return item
}

// processIndicesInBatches processes a list of file indices in batches, using a sliding
// window approach where the next batch starts after 80% of the current batch completes.
//
// Validates Requirements: 5.5
func (e *Engine) processIndicesInBatches(ctx context.Context, files []string, filesUTF16 []*uint16, isDirectory []bool, indices []int, batchSize int, workChan chan<- workItem, counters *atomicCounters) error {
	for start := 0; start < len(indices); start += batchSize {
		end := start + batchSize
		if end > len(indices) {
			end = len(indices)
		}

		batchIndices := indices[start:end]
		countBefore := counters.deleted.Load() + counters.failed.Load()

		// Send all items in the batch
		for _, i := range batchIndices {
			select {
			case workChan <- makeWorkItem(files, filesUTF16, isDirectory, i):
			case <-ctx.Done():
				return fmt.Errorf("deletion interrupted by user")
			}
		}

		// Sliding window: wait until the threshold fraction of the batch is processed
		threshold := int64(float64(len(batchIndices)) * SlidingWindowThreshold)
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("deletion interrupted by user")
			default:
				processed := (counters.deleted.Load() + counters.failed.Load()) - countBefore
				if processed >= threshold {
					time.Sleep(2 * time.Millisecond)
					goto nextBatch
				}
				time.Sleep(time.Millisecond)
			}
		}
	nextBatch:
	}
	return nil
}

// processIndicesAndWait sends all items at the given indices to workers and waits
// for them all to be processed before returning. Used for smaller file sets.
func (e *Engine) processIndicesAndWait(ctx context.Context, files []string, filesUTF16 []*uint16, isDirectory []bool, indices []int, workChan chan<- workItem, counters *atomicCounters) error {
	countBefore := counters.deleted.Load() + counters.failed.Load()

	for _, i := range indices {
		select {
		case workChan <- makeWorkItem(files, filesUTF16, isDirectory, i):
		case <-ctx.Done():
			return fmt.Errorf("deletion interrupted by user")
		}
	}

	// Wait until all items at this depth level have been processed
	expected := countBefore + int64(len(indices))
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("deletion interrupted by user")
		default:
			if counters.deleted.Load()+counters.failed.Load() >= expected {
				return nil
			}
			time.Sleep(time.Millisecond)
		}
	}
}

// countPathSeparators counts the number of path separators in a path
// to determine its depth in the directory hierarchy.
func countPathSeparators(path string) int {
	count := 0
	for _, char := range path {
		if char == '/' || char == '\\' {
			count++
		}
	}
	return count
}

// workerWithUTF16 is a goroutine that processes deletion work with optional UTF-16 paths.
// This worker uses pre-converted UTF-16 paths when available to avoid repeated conversions.
// Each worker runs in its own goroutine and processes files concurrently with other workers.
// Workers use atomic operations for lock-free statistics updates, improving performance.
// The worker stops when the context is cancelled or the work channel is closed.
//
// Validates Requirements: 4.1, 4.5
func (e *Engine) workerWithUTF16(ctx context.Context, workChan <-chan workItem, dryRun bool, result *DeletionResult, counters *atomicCounters, errorsMu *sync.Mutex, utf16Backend backend.UTF16Backend, supportsUTF16 bool) {
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop processing
			return
		case item, ok := <-workChan:
			if !ok {
				// Channel closed, no more work
				return
			}

			// Process this file
			logger.Debug("Processing: %s", item.pathUTF8)
			
			var err error
			if dryRun {
				// In dry-run mode, don't actually delete
				err = nil
			} else if supportsUTF16 && item.pathUTF16 != nil {
				// Use UTF-16 path if available and backend supports it
				err = e.deleteFileUTF16(item.pathUTF8, item.pathUTF16, item.isDirectory, utf16Backend)
			} else {
				// Fall back to UTF-8 path
				err = e.deleteFile(item.pathUTF8, item.isDirectory, dryRun)
			}

			// Update statistics using atomic operations (lock-free)
			if err != nil {
				counters.failed.Add(1)
				
				// Only lock when appending to error slice
				errorsMu.Lock()
				result.Errors = append(result.Errors, FileError{
					Path:  item.pathUTF8,
					Error: err.Error(),
				})
				errorsMu.Unlock()
				
				// Log the error with structured formatting
				logger.LogFileError(item.pathUTF8, err)
			} else {
				deletedCount := counters.deleted.Add(1)
				logger.Debug("Successfully deleted: %s", item.pathUTF8)
				
				// Call progress callback if provided
				if e.progressCallback != nil {
					e.progressCallback(int(deletedCount))
				}
			}
		}
	}
}

// deleteFile deletes a single file or directory using the backend.
// If the isDirectory flag is set, it skips the DeleteFile attempt and calls
// DeleteDirectory directly, avoiding an unnecessary system call.
// In dry-run mode, it skips actual deletion and returns success immediately.
// This dual approach handles both files and directories with a single function.
//
// Validates Requirements: 4.5
func (e *Engine) deleteFile(path string, isDirectory bool, dryRun bool) error {
	if dryRun {
		// In dry-run mode, don't actually delete
		return nil
	}

	// If we know it's a directory, skip the file deletion attempt
	if isDirectory {
		return e.backend.DeleteDirectory(path)
	}

	// Try to delete as a file first
	err := e.backend.DeleteFile(path)
	if err != nil {
		// If it fails, try as a directory
		err = e.backend.DeleteDirectory(path)
		if err != nil {
			return fmt.Errorf("failed to delete: %w", err)
		}
	}

	return nil
}

// deleteFileUTF16 deletes a single file or directory using pre-converted UTF-16 paths.
// This method uses the UTF16Backend interface to avoid repeated UTF-16 conversions.
// If the isDirectory flag is set, it skips the DeleteFileUTF16 attempt and calls
// DeleteDirectoryUTF16 directly, avoiding an unnecessary system call.
// This dual approach handles both files and directories with a single function.
//
// Validates Requirements: 4.5, 5.2, 5.3
func (e *Engine) deleteFileUTF16(pathUTF8 string, pathUTF16 *uint16, isDirectory bool, utf16Backend backend.UTF16Backend) error {
	// If we know it's a directory, skip the file deletion attempt
	if isDirectory {
		return utf16Backend.DeleteDirectoryUTF16(pathUTF16, pathUTF8)
	}

	// Try to delete as a file first
	err := utf16Backend.DeleteFileUTF16(pathUTF16, pathUTF8)
	if err != nil {
		// If it fails, try as a directory
		err = utf16Backend.DeleteDirectoryUTF16(pathUTF16, pathUTF8)
		if err != nil {
			return fmt.Errorf("failed to delete: %w", err)
		}
	}

	return nil
}

// SetupInterruptHandler sets up a signal handler for graceful interruption (Ctrl+C).
// It creates a context that will be cancelled when an interrupt signal (SIGINT or SIGTERM)
// is received. This allows the deletion engine to stop gracefully and report partial progress
// instead of terminating abruptly.
//
// Usage:
//
//	ctx, cancel := engine.SetupInterruptHandler()
//	defer cancel()
//	result, err := eng.Delete(ctx, files, dryRun)
//
// Returns a context and cancel function. The context will be cancelled when Ctrl+C is pressed.
func SetupInterruptHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	// Set up signal handling for graceful interruption
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start goroutine to handle signals
	go func() {
		<-sigChan
		// Signal received, cancel the context
		cancel()
	}()

	return ctx, cancel
}

// monitorDeletionRate monitors deletion rate every 5 seconds, tracks peak rate,
// and logs worker efficiency recommendations for future runs.
//
// Validates Requirements: 4.4, 12.3
func (e *Engine) monitorDeletionRate(ctx context.Context, counters *atomicCounters, peakRateChan chan<- float64) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastCount := int64(0)
	lastTime := time.Now()
	lastRate := 0.0
	peakRate := 0.0
	measurementCount := 0
	stableRateCount := 0

	// Ensure we send the peak rate when done
	defer func() {
		if peakRate > 0 {
			select {
			case peakRateChan <- peakRate:
			default:
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop monitoring
			// Log final adaptive tuning summary
			if peakRate > 0 {
				logger.Info("Peak deletion rate: %.1f files/sec with %d workers", peakRate, e.workers)
			}
			return
		case <-ticker.C:
			// Calculate current deletion rate
			currentCount := counters.deleted.Load()
			currentTime := time.Now()
			
			elapsed := currentTime.Sub(lastTime).Seconds()
			filesProcessed := currentCount - lastCount
			rate := float64(filesProcessed) / elapsed

			// Report current deletion rate (Requirement 12.3)
			if filesProcessed > 0 {
				logger.Info("Current deletion rate: %.1f files/sec (processed %d files in last %.1fs)",
					rate, filesProcessed, elapsed)
			}

			// Track peak rate
			if rate > peakRate {
				peakRate = rate
			}

			// Adaptive tuning analysis (Requirement 4.4)
			measurementCount++
			if measurementCount > 1 && rate > 0 {
				// Calculate rate change percentage
				rateChange := 0.0
				if lastRate > 0 {
					rateChange = ((rate - lastRate) / lastRate) * 100
				}

				// Detect stable rate (within 10% of previous measurement)
				if rateChange >= -10 && rateChange <= 10 {
					stableRateCount++
				} else {
					stableRateCount = 0
				}

				// Analyze performance patterns and provide recommendations
				cpuCount := runtime.NumCPU()
				optimalWorkers := cpuCount * DefaultWorkerMultiplier

				// If rate has been stable for 3+ measurements, analyze efficiency
				if stableRateCount >= 3 {
					// Calculate workers per file/sec ratio
					efficiency := rate / float64(e.workers)
					
					// If efficiency is low (< 10 files/sec per worker), system may be I/O bound
					if efficiency < 10 {
						logger.Debug("Worker efficiency: %.1f files/sec per worker (I/O bound)", efficiency)
						if e.workers > optimalWorkers {
							logger.Info("Adaptive tuning: Current worker count (%d) may be higher than optimal. Consider using %d workers (NumCPU*4) for future runs.", e.workers, optimalWorkers)
						}
					} else if efficiency > 50 && e.workers < cpuCount*8 {
						// High efficiency suggests we could benefit from more workers
						suggestedWorkers := cpuCount * 6
						if suggestedWorkers > e.workers {
							logger.Info("Adaptive tuning: High worker efficiency (%.1f files/sec per worker) detected. Consider increasing workers to %d (NumCPU*6) for future runs.", efficiency, suggestedWorkers)
						}
					}
				}

				// Detect declining rate (potential bottleneck)
				if measurementCount > 3 && rateChange < -20 {
					logger.Warning("Adaptive tuning: Deletion rate declining (%.1f%% decrease). This may indicate I/O saturation or filesystem bottleneck.", rateChange)
				}
			}

			// Update tracking variables for next iteration
			lastCount = currentCount
			lastTime = currentTime
			lastRate = rate
		}
	}
}
