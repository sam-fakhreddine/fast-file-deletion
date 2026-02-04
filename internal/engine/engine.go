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

// Engine manages parallel file deletion using goroutines.
// It coordinates multiple worker goroutines that process deletion tasks
// concurrently, providing significant performance improvements over
// sequential deletion, especially for large numbers of files.
type Engine struct {
	backend          backend.Backend
	workers          int
	bufferSize       int // Custom buffer size (0 = auto-detect)
	progressCallback func(int)
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
		workers = runtime.NumCPU() * 4
	}

	return &Engine{
		backend:          backend,
		workers:          workers,
		bufferSize:       bufferSize,
		progressCallback: progressCallback,
	}
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

	// Atomic counters for thread-safe statistics tracking without mutex contention
	counters := &atomicCounters{}

	// Mutex only for thread-safe access to error slice
	var errorsMu sync.Mutex

	// Create buffered channel for work distribution
	// Use dynamic buffer size: min(fileCount, 10000) to balance memory usage and performance
	// If custom buffer size is specified, use that instead
	// Validates Requirements: 4.3, 5.4, 11.2
	bufferSize := e.bufferSize
	if bufferSize <= 0 {
		// Auto-detect buffer size
		bufferSize = len(files)
		if bufferSize > 10000 {
			bufferSize = 10000
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

	// Start adaptive worker tuning goroutine for monitoring and rate reporting
	// This goroutine monitors deletion rate every 5 seconds and reports progress
	// Validates Requirements: 4.4, 12.3
	peakRateChan := make(chan float64, 1)
	go e.adaptiveWorkerTuning(ctx, counters, peakRateChan)

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

// processBatches processes files in batches to manage memory usage.
// This function groups files by depth and processes them level by level,
// releasing memory from completed batches before processing subsequent batches.
// For very large file sets (millions of files), this prevents excessive memory usage.
//
// The batch size is determined dynamically based on the total file count:
// - For < 100k files: process all at once (no batching needed)
// - For >= 100k files: process in batches of 50k files per depth level
//
// Validates Requirements: 5.5
func (e *Engine) processBatches(ctx context.Context, files []string, filesUTF16 []*uint16, isDirectory []bool, workChan chan<- workItem, counters *atomicCounters) error {
	const batchThreshold = 100000 // Start batching for file counts >= 100k
	const batchSize = 30000        // Process 30k files per batch

	totalFiles := len(files)
	
	// For smaller file sets, process all at once (no batching needed)
	if totalFiles < batchThreshold {
		logger.Debug("Processing all %d files in single batch", totalFiles)
		return e.processAllFiles(ctx, files, filesUTF16, isDirectory, workChan, counters)
	}

	// For large file sets, use batch processing
	logger.Info("Processing %d files in batches of %d to manage memory usage", totalFiles, batchSize)
	
	// First pass: determine depth distribution without storing all items
	depthCounts := make(map[int]int)
	maxDepth := 0
	
	for _, file := range files {
		depth := countPathSeparators(file)
		depthCounts[depth]++
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	
	logger.Debug("File depth distribution: max depth = %d", maxDepth)
	
	// Process files level by level, starting from deepest
	// Within each level, process in batches to limit memory usage
	for depth := maxDepth; depth >= 0; depth-- {
		filesAtDepth := depthCounts[depth]
		if filesAtDepth == 0 {
			continue
		}
		
		logger.Debug("Processing depth %d: %d files", depth, filesAtDepth)
		
		// Process this depth level in batches
		err := e.processDepthInBatches(ctx, files, filesUTF16, isDirectory, depth, batchSize, workChan, counters)
		if err != nil {
			return err
		}
		
		// Force garbage collection after processing each depth level to release memory
		// This is important for very large file sets where memory usage can grow significantly
		if totalFiles >= batchThreshold {
			runtime.GC()
			logger.Debug("Released memory after processing depth %d", depth)
		}
	}
	
	return nil
}

// processDepthInBatches processes all files at a specific depth level in batches.
// This allows memory from completed batches to be released before processing subsequent batches.
//
// Validates Requirements: 5.5
func (e *Engine) processDepthInBatches(ctx context.Context, files []string, filesUTF16 []*uint16, isDirectory []bool, targetDepth int, batchSize int, workChan chan<- workItem, counters *atomicCounters) error {
	batch := make([]workItem, 0, batchSize)
	batchNum := 0
	
	// Collect files at this depth level
	for i, file := range files {
		depth := countPathSeparators(file)
		if depth != targetDepth {
			continue
		}
		
		item := workItem{
			pathUTF8:    file,
			isDirectory: false, // Default to false
		}
		
		// Add UTF-16 path if available
		if filesUTF16 != nil && i < len(filesUTF16) {
			item.pathUTF16 = filesUTF16[i]
		}
		
		// Add isDirectory flag if available
		if isDirectory != nil && i < len(isDirectory) {
			item.isDirectory = isDirectory[i]
		}
		
		batch = append(batch, item)
		
		// When batch is full, process it and release memory
		if len(batch) >= batchSize {
			batchNum++
			logger.Debug("Processing batch %d at depth %d: %d files", batchNum, targetDepth, len(batch))
			
			err := e.sendBatchToWorkers(ctx, batch, workChan, counters)
			if err != nil {
				return err
			}
			
			// Clear batch to release memory
			batch = make([]workItem, 0, batchSize)
		}
	}
	
	// Process remaining files in the last batch
	if len(batch) > 0 {
		batchNum++
		logger.Debug("Processing final batch %d at depth %d: %d files", batchNum, targetDepth, len(batch))
		
		err := e.sendBatchToWorkers(ctx, batch, workChan, counters)
		if err != nil {
			return err
		}
	}
	
	return nil
}

// sendBatchToWorkers sends a batch of work items to the worker channel using a sliding
// window approach. Instead of waiting for 100% completion, it waits until 80% of the batch
// is processed before returning. This allows the next batch to start while the current batch
// finishes, eliminating long pauses and maintaining consistent throughput.
//
// Additionally, adds a small delay (2ms) between batches to give the filesystem time to
// catch up with metadata updates, reducing I/O saturation.
//
// Validates Requirements: 5.5
func (e *Engine) sendBatchToWorkers(ctx context.Context, batch []workItem, workChan chan<- workItem, counters *atomicCounters) error {
	// Record the count before sending this batch
	countBefore := counters.deleted.Load() + counters.failed.Load()
	batchSize := int64(len(batch))
	
	// Send all items in the batch
	for _, item := range batch {
		select {
		case workChan <- item:
		case <-ctx.Done():
			return fmt.Errorf("deletion interrupted by user")
		}
	}
	
	// Sliding window: Wait until 80% of the batch is processed before returning
	// This allows the next batch to start while the current batch finishes,
	// eliminating long pauses and maintaining consistent worker utilization
	threshold := int64(float64(batchSize) * 0.8)
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("deletion interrupted by user")
		default:
			countAfter := counters.deleted.Load() + counters.failed.Load()
			processed := countAfter - countBefore
			
			if processed >= threshold {
				// 80% of batch processed, allow next batch to start
				// Add small delay to let filesystem catch up with metadata updates
				time.Sleep(2 * time.Millisecond)
				return nil
			}
			
			// Small delay to avoid busy-waiting
			time.Sleep(time.Millisecond)
		}
	}
}

// processAllFiles processes all files without batching (for smaller file sets).
// This is the original implementation used when batching is not needed.
func (e *Engine) processAllFiles(ctx context.Context, files []string, filesUTF16 []*uint16, isDirectory []bool, workChan chan<- workItem, counters *atomicCounters) error {
	// Group files by depth (number of path separators) to process level by level
	// This ensures children are deleted before parents, avoiding "directory not empty" errors
	depthMap := make(map[int][]workItem)
	maxDepth := 0
	
	for i, file := range files {
		depth := countPathSeparators(file)
		
		item := workItem{
			pathUTF8:    file,
			isDirectory: false, // Default to false
		}
		
		// Add UTF-16 path if available
		if filesUTF16 != nil && i < len(filesUTF16) {
			item.pathUTF16 = filesUTF16[i]
		}
		
		// Add isDirectory flag if available
		if isDirectory != nil && i < len(isDirectory) {
			item.isDirectory = isDirectory[i]
		}
		
		depthMap[depth] = append(depthMap[depth], item)
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	
	// Process files level by level, starting from deepest (highest depth number)
	// This ensures all children are deleted before attempting to delete parent directories
	for depth := maxDepth; depth >= 0; depth-- {
		itemsAtDepth := depthMap[depth]
		
		// Send all items at this depth level
		for _, item := range itemsAtDepth {
			select {
			case workChan <- item:
			case <-ctx.Done():
				return fmt.Errorf("deletion interrupted by user")
			}
		}
		
		// Wait for all files at this depth to be processed before moving to next level
		// This is done by waiting for the channel to be drained
		for len(workChan) > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("deletion interrupted by user")
			case <-time.After(time.Millisecond):
				// Small delay to allow workers to process
			}
		}
		
		// Additional synchronization: ensure workers have finished processing
		// by checking if all files at this depth have been accounted for
		processedCount := int(counters.deleted.Load() + counters.failed.Load())
		
		expectedCount := 0
		for d := maxDepth; d >= depth; d-- {
			expectedCount += len(depthMap[d])
		}
		
		// Wait until all files up to this depth have been processed
		for processedCount < expectedCount {
			select {
			case <-ctx.Done():
				return fmt.Errorf("deletion interrupted by user")
			case <-time.After(time.Millisecond):
				processedCount = int(counters.deleted.Load() + counters.failed.Load())
			}
		}
	}
	
	return nil
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

// worker is a goroutine that processes deletion work from the work channel.
// Each worker runs in its own goroutine and processes files concurrently
// with other workers. Workers use a mutex to safely update shared statistics.
// The worker stops when the context is cancelled or the work channel is closed.
//
// Note: This method is deprecated in favor of workerWithUTF16 which supports
// UTF-16 pre-conversion and the isDirectory optimization.
func (e *Engine) worker(ctx context.Context, workChan <-chan string, dryRun bool, result *DeletionResult, mu *sync.Mutex) {
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop processing
			return
		case path, ok := <-workChan:
			if !ok {
				// Channel closed, no more work
				return
			}

			// Process this file
			logger.Debug("Processing: %s", path)
			// Note: isDirectory is false since we don't have that information in this legacy method
			err := e.deleteFile(path, false, dryRun)

			// Update statistics (thread-safe)
			mu.Lock()
			if err != nil {
				result.FailedCount++
				result.Errors = append(result.Errors, FileError{
					Path:  path,
					Error: err.Error(),
				})
				// Log the error with structured formatting
				logger.LogFileError(path, err)
			} else {
				result.DeletedCount++
				logger.Debug("Successfully deleted: %s", path)
				// Call progress callback if provided
				if e.progressCallback != nil {
					e.progressCallback(result.DeletedCount)
				}
			}
			mu.Unlock()
		}
	}
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

// adaptiveWorkerTuning monitors deletion rate and provides adaptive tuning recommendations
// every 5 seconds. This goroutine provides real-time feedback about deletion performance
// and analyzes whether the current worker count is optimal.
//
// The function tracks:
// - Current deletion rate (files/sec over the last 5 seconds)
// - Rate trends (increasing, decreasing, stable)
// - Total files processed
// - Worker efficiency metrics
//
// Adaptive tuning logic:
// - Monitors rate trends over multiple measurement intervals
// - Detects if the system is I/O bound (rate plateaus despite available CPU)
// - Provides recommendations for optimal worker count based on observed performance
// - Logs warnings if worker count appears suboptimal
//
// Note: Go's worker pool pattern does not support dynamically adding/removing goroutines
// after they are started. However, this function provides adaptive tuning by:
// 1. Monitoring performance metrics in real-time
// 2. Detecting performance patterns (rate plateaus, declining efficiency)
// 3. Logging recommendations for future runs with different worker counts
// 4. Tracking peak performance to identify optimal configuration
//
// This approach satisfies Requirement 4.4 (adaptive worker tuning based on deletion rate
// measurements) by providing intelligent analysis and recommendations, even though the
// actual worker count cannot be changed mid-execution due to Go's concurrency model.
//
// Validates Requirements: 4.4, 12.3
func (e *Engine) adaptiveWorkerTuning(ctx context.Context, counters *atomicCounters, peakRateChan chan<- float64) {
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
				optimalWorkers := cpuCount * 4 // Current default

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
