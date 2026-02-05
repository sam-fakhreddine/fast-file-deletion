// Package scanner provides directory traversal and file discovery functionality
// with optional age-based filtering for selective deletion.
package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/yourusername/fast-file-deletion/internal/logger"
)

// Scanner handles directory traversal and file discovery with optional age filtering.
// It efficiently walks directory trees and builds a list of files to delete,
// ordered bottom-up (files before their parent directories) for safe deletion.
type Scanner struct {
	rootPath string
	keepDays *int
}

// ScanResult contains the results of a directory scan.
// It includes statistics about files scanned, files to delete, files retained,
// and the total size of files to be deleted.
type ScanResult struct {
	ScannedPath    string        // Absolute path that was scanned (for TOCTOU protection)
	Files          []string      // List of files to delete (bottom-up order)
	FilesUTF16     []*uint16     // Pre-converted UTF-16 paths (Windows only)
	IsDirectory    []bool        // Flags indicating if each path is a directory
	TotalScanned   int           // Total number of files and directories scanned
	TotalToDelete  int           // Number of files and directories marked for deletion
	TotalRetained  int           // Number of files retained due to age filtering
	TotalSizeBytes int64         // Total size of files to delete (in bytes)
	ScanDuration   time.Duration // Time taken to complete the scan
}

// NewScanner creates a new Scanner instance.
//
// Parameters:
//   - rootPath: The directory to scan
//   - keepDays: Optional age filter - only delete files older than this many days
//     (nil = delete all files, 0 = delete all files)
//
// Returns a configured Scanner ready to perform directory traversal.
func NewScanner(rootPath string, keepDays *int) *Scanner {
	return &Scanner{
		rootPath: rootPath,
		keepDays: keepDays,
	}
}

// Scan traverses the directory tree and builds a list of files to delete.
// Files are ordered bottom-up (files before their parent directories) for safe deletion.
// This ordering ensures that directories are empty when we attempt to delete them.
//
// The scan process:
//  1. Walks the directory tree using filepath.WalkDir for efficiency
//  2. Applies age filtering if keepDays is set
//  3. Separates files and directories
//  4. Orders directories deepest-first for bottom-up deletion
//  5. Calculates total size for progress reporting
//  6. Tracks directory flags for skip-double-call optimization
//
// Returns ScanResult with file list and statistics, or an error if scanning fails.
//
// Validates Requirements: 4.5
func (s *Scanner) Scan() (*ScanResult, error) {
	logger.Info("Starting scan of directory: %s", s.rootPath)
	if s.keepDays != nil {
		logger.Info("Age filter enabled: keeping files newer than %d days", *s.keepDays)
	}

	// Validate that the root path exists before scanning
	if _, err := os.Stat(s.rootPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory does not exist: %s", s.rootPath)
		}
		return nil, fmt.Errorf("cannot access directory: %w", err)
	}

	// Get absolute path for TOCTOU protection
	absPath, err := filepath.Abs(s.rootPath)
	if err != nil {
		return nil, fmt.Errorf("cannot get absolute path: %w", err)
	}

	result := &ScanResult{
		ScannedPath: absPath,
		Files:       make([]string, 0),
		IsDirectory: make([]bool, 0),
	}

	// Track directories separately to add them after files (bottom-up)
	directories := make([]string, 0)

	// Walk the directory tree
	err = filepath.WalkDir(s.rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// If we can't access a path, log it but continue
			logger.LogFileWarning(path, fmt.Sprintf("Cannot access: %v", err))
			return nil
		}

		// Skip the root directory itself (we'll handle it separately)
		if path == s.rootPath {
			return nil
		}

		result.TotalScanned++

		// Check if this file/directory should be deleted based on age
		shouldDel, fileSize, err := s.shouldDelete(path, d)
		if err != nil {
			// If we can't determine age, skip this file but continue
			logger.LogFileWarning(path, fmt.Sprintf("Cannot determine age: %v", err))
			return nil
		}

		if shouldDel {
			result.TotalToDelete++
			result.TotalSizeBytes += fileSize

			if d.IsDir() {
				// Store directories separately to add them after files
				directories = append(directories, path)
			} else {
				// Add files immediately
				result.Files = append(result.Files, path)
				result.IsDirectory = append(result.IsDirectory, false)
			}
		} else {
			result.TotalRetained++
			logger.Debug("Retaining file (too new): %s", path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	// Add directories in reverse order (deepest first) for bottom-up deletion
	for i := len(directories) - 1; i >= 0; i-- {
		result.Files = append(result.Files, directories[i])
		result.IsDirectory = append(result.IsDirectory, true)
	}

	// Finally, add the root directory itself if we're deleting everything
	// Only add root directory when no age filter is set (deleting all files)
	// Don't add it when doing partial deletion with age filtering
	if s.keepDays == nil || *s.keepDays == 0 {
		result.Files = append(result.Files, s.rootPath)
		result.IsDirectory = append(result.IsDirectory, true)
		result.TotalToDelete++
	}

	logger.Info("Scan complete: %d scanned, %d to delete, %d retained",
		result.TotalScanned, result.TotalToDelete, result.TotalRetained)

	return result, nil
}

// shouldDelete determines if a file or directory should be deleted based on age filtering.
// Returns (shouldDelete, fileSize, error).
//
// Age filtering logic:
//   - If keepDays is nil or 0, all files are marked for deletion
//   - Otherwise, only files older than keepDays are marked for deletion
//   - Age is calculated using the file's modification time (mtime)
//
// The function returns the file size for progress reporting (0 for directories).
func (s *Scanner) shouldDelete(path string, d fs.DirEntry) (bool, int64, error) {
	// If no age filter is set, delete everything
	if s.keepDays == nil {
		return true, s.getFileSize(path, d), nil
	}

	// If keepDays is 0, delete everything (edge case)
	if *s.keepDays == 0 {
		return true, s.getFileSize(path, d), nil
	}

	// Get file info to check modification time
	info, err := d.Info()
	if err != nil {
		return false, 0, fmt.Errorf("failed to get file info: %w", err)
	}

	// Calculate file age based on modification time
	fileAge := time.Since(info.ModTime())
	keepDuration := time.Duration(*s.keepDays) * 24 * time.Hour

	// Delete if file is older than the retention period
	shouldDel := fileAge > keepDuration

	return shouldDel, s.getFileSize(path, d), nil
}

// getFileSize returns the size of a file or 0 for directories.
// This is used for progress reporting and statistics.
// If the file info cannot be retrieved, returns 0.
func (s *Scanner) getFileSize(path string, d fs.DirEntry) int64 {
	if d.IsDir() {
		return 0
	}

	info, err := d.Info()
	if err != nil {
		return 0
	}

	return info.Size()
}

// ParallelScanner extends Scanner with concurrent directory traversal capabilities.
// It provides Windows-optimized parallel scanning using FindFirstFileEx and worker pools
// for improved performance on large directory trees.
type ParallelScanner struct {
	rootPath        string
	keepDays        *int
	workers         int  // Number of parallel scan workers
	useWinAPI       bool // Use FindFirstFileEx vs filepath.WalkDir
	preConvertUTF16 bool // Pre-convert paths to UTF-16 during scan
}

// NewParallelScanner creates a new ParallelScanner instance with parallel scanning capabilities.
//
// Parameters:
//   - rootPath: The directory to scan
//   - keepDays: Optional age filter - only delete files older than this many days
//     (nil = delete all files, 0 = delete all files)
//   - workers: Number of parallel worker goroutines for scanning
//     (0 = auto-detect based on runtime.NumCPU())
//
// The parallel scanner uses a worker pool to process subdirectories concurrently,
// significantly improving scan performance on large directory trees. On Windows,
// it can optionally use FindFirstFileEx for faster directory enumeration and
// pre-convert paths to UTF-16 to avoid repeated conversions during deletion.
//
// Returns a configured ParallelScanner ready to perform parallel directory traversal.
func NewParallelScanner(rootPath string, keepDays *int, workers int) *ParallelScanner {
	// Auto-detect worker count if not specified
	if workers <= 0 {
		workers = 4 // Default to 4 workers for reasonable parallelism
	}

	return &ParallelScanner{
		rootPath:        rootPath,
		keepDays:        keepDays,
		workers:         workers,
		useWinAPI:       false, // Will be set to true on Windows in platform-specific code
		preConvertUTF16: false, // Will be set to true on Windows in platform-specific code
	}
}
