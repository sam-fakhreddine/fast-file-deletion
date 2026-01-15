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
	progressCallback func(int)
}

// DeletionResult contains statistics and errors from a deletion operation.
// It provides comprehensive information about the deletion process including
// success/failure counts, detailed error information, and timing data.
type DeletionResult struct {
	DeletedCount    int         // Number of files successfully deleted
	FailedCount     int         // Number of files that failed to delete
	Errors          []FileError // List of errors encountered during deletion
	DurationSeconds float64     // Total time taken for deletion
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
// count based on CPU cores using the formula: NumCPU * 2. This provides good
// parallelism without overwhelming the system.
//
// Returns a configured Engine ready to perform parallel deletion.
func NewEngine(backend backend.Backend, workers int, progressCallback func(int)) *Engine {
	// Auto-detect optimal worker count if not specified
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}

	return &Engine{
		backend:          backend,
		workers:          workers,
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
	startTime := time.Now()

	logger.Info("Starting deletion of %d files with %d workers", len(files), e.workers)
	if dryRun {
		logger.Info("Running in DRY-RUN mode - no files will be deleted")
	}

	result := &DeletionResult{
		Errors: make([]FileError, 0),
	}

	// Mutex for thread-safe access to result statistics
	var mu sync.Mutex

	// Group files by depth (number of path separators) to process level by level
	// This ensures children are deleted before parents, avoiding "directory not empty" errors
	depthMap := make(map[int][]string)
	maxDepth := 0
	
	for _, file := range files {
		depth := countPathSeparators(file)
		depthMap[depth] = append(depthMap[depth], file)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	// Create buffered channel for work distribution
	workChan := make(chan string, 100)

	// WaitGroup to track worker completion
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < e.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.worker(ctx, workChan, dryRun, result, &mu)
		}()
	}

	// Process files level by level, starting from deepest (highest depth number)
	// This ensures all children are deleted before attempting to delete parent directories
	go func() {
		defer close(workChan)
		
		for depth := maxDepth; depth >= 0; depth-- {
			filesAtDepth := depthMap[depth]
			
			// Send all files at this depth level
			for _, file := range filesAtDepth {
				select {
				case workChan <- file:
				case <-ctx.Done():
					// Context cancelled, stop sending work
					logger.Warning("Deletion interrupted by user")
					return
				}
			}
			
			// Wait for all files at this depth to be processed before moving to next level
			// This is done by waiting for the channel to be drained
			for len(workChan) > 0 {
				select {
				case <-ctx.Done():
					logger.Warning("Deletion interrupted by user")
					return
				case <-time.After(time.Millisecond):
					// Small delay to allow workers to process
				}
			}
			
			// Additional synchronization: ensure workers have finished processing
			// by checking if all files at this depth have been accounted for
			mu.Lock()
			processedCount := result.DeletedCount + result.FailedCount
			mu.Unlock()
			
			expectedCount := 0
			for d := maxDepth; d >= depth; d-- {
				expectedCount += len(depthMap[d])
			}
			
			// Wait until all files up to this depth have been processed
			for processedCount < expectedCount {
				select {
				case <-ctx.Done():
					logger.Warning("Deletion interrupted by user")
					mu.Unlock()
					return
				case <-time.After(time.Millisecond):
					mu.Lock()
					processedCount = result.DeletedCount + result.FailedCount
					mu.Unlock()
				}
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()

	// Calculate duration
	result.DurationSeconds = time.Since(startTime).Seconds()

	logger.Info("Deletion completed: %d succeeded, %d failed in %.2f seconds",
		result.DeletedCount, result.FailedCount, result.DurationSeconds)

	return result, nil
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
			err := e.deleteFile(path, dryRun)

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

// deleteFile deletes a single file or directory using the backend.
// It first attempts to delete as a file, and if that fails, tries as a directory.
// In dry-run mode, it skips actual deletion and returns success immediately.
// This dual approach handles both files and directories with a single function.
func (e *Engine) deleteFile(path string, dryRun bool) error {
	if dryRun {
		// In dry-run mode, don't actually delete
		return nil
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
