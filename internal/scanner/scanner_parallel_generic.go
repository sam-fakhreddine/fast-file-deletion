//go:build !windows

package scanner

import (
	"time"
)

// Scan performs directory traversal using the sequential scanner as a fallback.
// This is the generic implementation for non-Windows platforms.
// Windows has a platform-specific implementation with parallel scanning.
//
// Returns ScanResult with file list and statistics, or an error if scanning fails.
//
// Validates Requirements: 4.5
func (ps *ParallelScanner) Scan() (*ScanResult, error) {
	startTime := time.Now()

	// Use the sequential scanner as a fallback on non-Windows platforms
	scanner := NewScanner(ps.rootPath, ps.keepDays)
	result, err := scanner.Scan()
	if err != nil {
		return nil, err
	}

	// Record scan duration
	result.ScanDuration = time.Since(startTime)

	// Initialize UTF-16 slice (empty on non-Windows platforms)
	result.FilesUTF16 = make([]*uint16, 0)

	return result, nil
}
