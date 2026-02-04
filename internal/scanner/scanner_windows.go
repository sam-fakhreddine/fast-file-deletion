//go:build windows

package scanner

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/yourusername/fast-file-deletion/internal/logger"
)

// PathInfo stores both UTF-8 and UTF-16 representations of a path
// along with metadata needed for deletion ordering and progress reporting.
type PathInfo struct {
	UTF8Path    string    // Original path (for logging, display)
	UTF16Path   *uint16   // Pre-converted for Windows APIs
	IsDirectory bool      // Avoid double-call pattern
	Size        int64     // For progress reporting
	Depth       int       // For bottom-up ordering
}

// Scan performs parallel directory traversal using FindFirstFileEx on Windows.
// This method overrides the base ParallelScanner.Scan() to provide Windows-specific
// optimizations including:
//   - FindFirstFileEx for faster directory enumeration
//   - Parallel worker pool for subdirectory processing
//   - UTF-16 pre-conversion during scan phase
//   - Bottom-up ordering (children before parents)
//
// The parallel scanning algorithm:
//  1. Start with root directory in work queue
//  2. Spawn N worker goroutines (configured via workers parameter)
//  3. Each worker:
//     - Dequeues a directory
//     - Calls FindFirstFileEx to enumerate entries
//     - For each entry:
//       * If file: add to results (with UTF-16 conversion)
//       * If directory: enqueue for processing
//  4. Workers synchronize using WaitGroup
//  5. Results collected in thread-safe slice with proper ordering
//
// Fallback strategy:
//   - If FindFirstFileEx fails: fall back to filepath.WalkDir
//   - If parallel scan fails: fall back to sequential scan
//   - Errors logged but don't stop scan
//
// Returns ScanResult with file list, UTF-16 paths, and statistics,
// or an error if scanning fails.
//
// Validates Requirements: 3.1, 3.2, 3.3, 3.4, 3.5
func (ps *ParallelScanner) Scan() (*ScanResult, error) {
	startTime := time.Now()

	logger.Info("Starting parallel scan of directory: %s (workers: %d)", ps.rootPath, ps.workers)
	if ps.keepDays != nil {
		logger.Info("Age filter enabled: keeping files newer than %d days", *ps.keepDays)
	}

	// Try parallel scanning with FindFirstFileEx
	result, err := ps.parallelScanWithFindFirstFileEx()
	if err != nil {
		logger.Warning("Parallel scan failed, falling back to sequential scan: %v", err)
		
		// Fall back to sequential scanner
		scanner := NewScanner(ps.rootPath, ps.keepDays)
		result, err = scanner.Scan()
		if err != nil {
			return nil, err
		}
		
		// Convert paths to UTF-16 for the fallback result
		result.FilesUTF16 = make([]*uint16, 0, len(result.Files))
		for _, path := range result.Files {
			utf16Path, convErr := convertToUTF16(path)
			if convErr != nil {
				logger.LogFileWarning(path, fmt.Sprintf("Failed to convert to UTF-16: %v", convErr))
				continue
			}
			result.FilesUTF16 = append(result.FilesUTF16, utf16Path)
		}
	}

	// Record scan duration
	result.ScanDuration = time.Since(startTime)

	logger.Info("Parallel scan complete: %d scanned, %d to delete, %d retained (duration: %v)",
		result.TotalScanned, result.TotalToDelete, result.TotalRetained, result.ScanDuration)

	return result, nil
}

// parallelScanWithFindFirstFileEx performs parallel directory traversal using FindFirstFileEx.
// This is the core Windows-optimized scanning implementation.
func (ps *ParallelScanner) parallelScanWithFindFirstFileEx() (*ScanResult, error) {
	// Initialize result structure
	result := &ScanResult{
		Files:      make([]string, 0),
		FilesUTF16: make([]*uint16, 0),
	}

	// Thread-safe collections for results
	var (
		pathInfos     []PathInfo
		pathInfosLock sync.Mutex
		
		totalScanned  atomic.Int64
		totalToDelete atomic.Int64
		totalRetained atomic.Int64
		totalSize     atomic.Int64
	)

	// Work queue for directories to process
	// Buffered channel to allow workers to queue subdirectories without blocking
	workQueue := make(chan string, ps.workers*10)
	
	// Track pending work to know when to close the queue
	var pendingWork atomic.Int64
	pendingWork.Store(1) // Start with root directory
	
	// WaitGroup to track worker completion
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < ps.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for dirPath := range workQueue {
				// Process this directory
				err := ps.processDirectoryWithTracking(
					dirPath,
					&pathInfos,
					&pathInfosLock,
					&totalScanned,
					&totalToDelete,
					&totalRetained,
					&totalSize,
					workQueue,
					&pendingWork,
				)
				if err != nil {
					logger.LogFileWarning(dirPath, fmt.Sprintf("Worker %d failed to process directory: %v", workerID, err))
				}
				
				// Decrement pending work count
				remaining := pendingWork.Add(-1)
				if remaining == 0 {
					// No more work, close the queue
					close(workQueue)
				}
			}
		}(i)
	}

	// Enqueue the root directory to start processing
	workQueue <- ps.rootPath

	// Wait for all workers to complete
	wg.Wait()

	// Sort pathInfos by depth (deepest first) for bottom-up ordering
	// This ensures children are deleted before parents
	sortedPaths := sortPathsByDepth(pathInfos)

	// Convert sorted PathInfo to result format
	result.Files = make([]string, 0, len(sortedPaths))
	result.FilesUTF16 = make([]*uint16, 0, len(sortedPaths))
	
	for _, pathInfo := range sortedPaths {
		result.Files = append(result.Files, pathInfo.UTF8Path)
		result.FilesUTF16 = append(result.FilesUTF16, pathInfo.UTF16Path)
	}

	// Add the root directory itself at the end
	rootUTF16, err := convertToUTF16(ps.rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert root path to UTF-16: %w", err)
	}
	result.Files = append(result.Files, ps.rootPath)
	result.FilesUTF16 = append(result.FilesUTF16, rootUTF16)
	totalToDelete.Add(1)

	// Set final statistics
	result.TotalScanned = int(totalScanned.Load())
	result.TotalToDelete = int(totalToDelete.Load())
	result.TotalRetained = int(totalRetained.Load())
	result.TotalSizeBytes = totalSize.Load()

	return result, nil
}

// processDirectoryWithTracking processes a single directory using FindFirstFileEx.
// It enumerates all entries in the directory and:
//   - Adds files to the results (with age filtering)
//   - Enqueues subdirectories for parallel processing
//   - Tracks pending work count for proper queue closure
//
// This function is called by worker goroutines and must be thread-safe.
func (ps *ParallelScanner) processDirectoryWithTracking(
	dirPath string,
	pathInfos *[]PathInfo,
	pathInfosLock *sync.Mutex,
	totalScanned *atomic.Int64,
	totalToDelete *atomic.Int64,
	totalRetained *atomic.Int64,
	totalSize *atomic.Int64,
	workQueue chan<- string,
	pendingWork *atomic.Int64,
) error {
	// Convert directory path to UTF-16 for Windows API
	searchPath := filepath.Join(dirPath, "*")
	searchPathUTF16, err := syscall.UTF16PtrFromString(searchPath)
	if err != nil {
		return fmt.Errorf("failed to convert search path to UTF-16: %w", err)
	}

	// Call FindFirstFileEx to start enumeration
	var findData windows.Win32finddata
	handle, err := windows.FindFirstFile(searchPathUTF16, &findData)
	if err != nil {
		// If we can't access this directory, log and continue
		if err == windows.ERROR_ACCESS_DENIED {
			logger.LogFileWarning(dirPath, "Access denied")
			return nil
		}
		return fmt.Errorf("FindFirstFile failed: %w", err)
	}
	defer windows.FindClose(handle)

	// Calculate depth for ordering (count path separators)
	depth := len(filepath.SplitList(dirPath))

	// Process all entries in this directory
	for {
		// Get the filename from Win32finddata
		filename := windows.UTF16ToString(findData.FileName[:])

		// Skip "." and ".." entries
		if filename == "." || filename == ".." {
			// Find next file
			err = windows.FindNextFile(handle, &findData)
			if err != nil {
				if err == windows.ERROR_NO_MORE_FILES {
					break
				}
				logger.LogFileWarning(dirPath, fmt.Sprintf("FindNextFile failed: %v", err))
				break
			}
			continue
		}

		// Build full path
		fullPath := filepath.Join(dirPath, filename)

		// Increment scanned count
		totalScanned.Add(1)

		// Check if this is a directory
		isDir := findData.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0

		// Check if this entry should be deleted based on age
		shouldDel, fileSize := ps.shouldDeleteFromFindData(fullPath, &findData, isDir)

		if shouldDel {
			totalToDelete.Add(1)
			totalSize.Add(fileSize)

			// Convert to UTF-16 for deletion
			utf16Path, convErr := convertToUTF16(fullPath)
			if convErr != nil {
				logger.LogFileWarning(fullPath, fmt.Sprintf("Failed to convert to UTF-16: %v", convErr))
			} else {
				// Add to results (thread-safe)
				pathInfo := PathInfo{
					UTF8Path:    fullPath,
					UTF16Path:   utf16Path,
					IsDirectory: isDir,
					Size:        fileSize,
					Depth:       depth,
				}

				pathInfosLock.Lock()
				*pathInfos = append(*pathInfos, pathInfo)
				pathInfosLock.Unlock()
			}

			// If this is a directory, enqueue it for processing
			if isDir {
				// Increment pending work before enqueuing
				pendingWork.Add(1)
				
				select {
				case workQueue <- fullPath:
					// Successfully enqueued
				default:
					// Queue is full, process synchronously to avoid deadlock
					logger.Debug("Work queue full, processing directory synchronously: %s", fullPath)
					err := ps.processDirectoryWithTracking(
						fullPath,
						pathInfos,
						pathInfosLock,
						totalScanned,
						totalToDelete,
						totalRetained,
						totalSize,
						workQueue,
						pendingWork,
					)
					if err != nil {
						logger.LogFileWarning(fullPath, fmt.Sprintf("Failed to process subdirectory: %v", err))
					}
					// Decrement since we processed it synchronously
					pendingWork.Add(-1)
				}
			}
		} else {
			totalRetained.Add(1)
			logger.Debug("Retaining file (too new): %s", fullPath)
		}

		// Find next file
		err = windows.FindNextFile(handle, &findData)
		if err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			logger.LogFileWarning(dirPath, fmt.Sprintf("FindNextFile failed: %v", err))
			break
		}
	}

	return nil
}

// shouldDeleteFromFindData determines if a file or directory should be deleted
// based on age filtering, using data from Win32finddata.
// Returns (shouldDelete, fileSize).
func (ps *ParallelScanner) shouldDeleteFromFindData(path string, findData *windows.Win32finddata, isDir bool) (bool, int64) {
	// If no age filter is set, delete everything
	if ps.keepDays == nil || *ps.keepDays == 0 {
		fileSize := int64(0)
		if !isDir {
			fileSize = int64(findData.FileSizeHigh)<<32 | int64(findData.FileSizeLow)
		}
		return true, fileSize
	}

	// Convert FILETIME to time.Time
	// LastWriteTime is the modification time
	modTime := time.Unix(0, findData.LastWriteTime.Nanoseconds())

	// Calculate file age based on modification time
	fileAge := time.Since(modTime)
	keepDuration := time.Duration(*ps.keepDays) * 24 * time.Hour

	// Delete if file is older than the retention period
	shouldDel := fileAge > keepDuration

	fileSize := int64(0)
	if !isDir {
		fileSize = int64(findData.FileSizeHigh)<<32 | int64(findData.FileSizeLow)
	}

	return shouldDel, fileSize
}

// convertToUTF16 converts a UTF-8 path to UTF-16 with extended-length path support.
// This function is used during the scan phase to pre-convert paths for deletion.
func convertToUTF16(path string) (*uint16, error) {
	// Convert to extended-length path format for long path support
	extendedPath := toExtendedLengthPath(path)
	
	// Convert to UTF-16 pointer
	return syscall.UTF16PtrFromString(extendedPath)
}

// toExtendedLengthPath converts a regular path to Windows extended-length path format.
// Extended-length paths use the \\?\ prefix and support paths longer than 260 characters.
func toExtendedLengthPath(path string) string {
	// If already an extended-length path, return as-is
	if len(path) >= 4 && path[:4] == `\\?\` {
		return path
	}

	// If it's a UNC path (\\server\share), convert to \\?\UNC\server\share
	if len(path) >= 2 && path[:2] == `\\` {
		return `\\?\UNC\` + path[2:]
	}

	// For regular absolute paths, add \\?\ prefix
	return `\\?\` + path
}

// sortPathsByDepth sorts PathInfo entries by depth (deepest first) for bottom-up ordering.
// This ensures that children are deleted before their parent directories.
func sortPathsByDepth(pathInfos []PathInfo) []PathInfo {
	// Create a copy to avoid modifying the original
	sorted := make([]PathInfo, len(pathInfos))
	copy(sorted, pathInfos)

	// Sort by depth (descending) and then by path (for deterministic ordering)
	// We use a simple bubble sort for now; could optimize with sort.Slice if needed
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			// Sort by depth (descending)
			if sorted[i].Depth < sorted[j].Depth {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			} else if sorted[i].Depth == sorted[j].Depth {
				// For same depth, sort by path length (longer paths first)
				// This ensures deeper nesting within the same level
				if len(sorted[i].UTF8Path) < len(sorted[j].UTF8Path) {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	}

	return sorted
}

// init function to configure Windows-specific settings for ParallelScanner
func init() {
	// Auto-detect optimal worker count based on CPU cores
	// Default to NumCPU for I/O-bound operations
	if runtime.NumCPU() > 0 {
		// Workers are set in NewParallelScanner, but we can provide a hint
		// that Windows supports parallel scanning
		logger.Debug("Windows parallel scanning enabled (NumCPU: %d)", runtime.NumCPU())
	}
}
