//go:build windows

// Package backend provides Windows-optimized file deletion using direct Win32 API calls.
package backend

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
	"github.com/yourusername/fast-file-deletion/internal/logger"
)

// WindowsBackend provides Windows-optimized file deletion using direct syscalls.
// It uses the Windows API (DeleteFile and RemoveDirectory) for better performance
// compared to standard Go file operations. It also handles long paths correctly
// by using the extended-length path prefix (\\?\).
//
// Performance benefits:
//   - Direct syscalls bypass Go's file handling overhead
//   - Extended-length path support for paths > 260 characters
//   - Better error handling with Windows-specific error codes
type WindowsBackend struct{}

// NewWindowsBackend creates a new Windows-optimized backend.
func NewWindowsBackend() *WindowsBackend {
	return &WindowsBackend{}
}

// DeleteFile deletes a single file using the Windows DeleteFile API.
// It automatically handles long paths by adding the extended-length path prefix (\\?\).
//
// The Windows DeleteFile API provides better performance than Go's os.Remove because:
//   - It's a direct syscall with minimal overhead
//   - It handles Windows-specific file attributes correctly
//   - It provides detailed Windows error codes for better error handling
//
// Returns an error if the file cannot be deleted (e.g., permission denied, file locked).
func (b *WindowsBackend) DeleteFile(path string) error {
	// Convert to extended-length path format for long path support
	extendedPath := toExtendedLengthPath(path)

	// Convert to UTF-16 pointer for Windows API
	pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
	if err != nil {
		logger.Debug("Failed to convert path to UTF-16: %s", path)
		return fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	// Call Windows DeleteFile API
	err = windows.DeleteFile(pathPtr)
	if err != nil {
		logger.Debug("Windows DeleteFile API failed for: %s (error: %v)", path, err)
		return fmt.Errorf("failed to delete file %s: %w", path, err)
	}

	return nil
}

// DeleteDirectory deletes an empty directory using the Windows RemoveDirectory API.
// The directory must be empty before calling this method.
//
// Like DeleteFile, this uses direct Windows API calls for better performance.
// It also handles long paths correctly using the extended-length path prefix.
//
// Returns an error if the directory cannot be deleted or is not empty.
func (b *WindowsBackend) DeleteDirectory(path string) error {
	// Convert to extended-length path format for long path support
	extendedPath := toExtendedLengthPath(path)

	// Convert to UTF-16 pointer for Windows API
	pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
	if err != nil {
		logger.Debug("Failed to convert directory path to UTF-16: %s", path)
		return fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	// Call Windows RemoveDirectory API
	err = windows.RemoveDirectory(pathPtr)
	if err != nil {
		logger.Debug("Windows RemoveDirectory API failed for: %s (error: %v)", path, err)
		return fmt.Errorf("failed to delete directory %s: %w", path, err)
	}

	return nil
}

// toExtendedLengthPath converts a regular path to Windows extended-length path format.
// Extended-length paths use the \\?\ prefix and support paths longer than 260 characters,
// which is the traditional Windows path length limit (MAX_PATH).
//
// Path conversion rules:
//   - Already extended (\\?\...): Return as-is
//   - UNC path (\\server\share): Convert to \\?\UNC\server\share
//   - Absolute path (C:\path): Convert to \\?\C:\path
//
// This function is critical for handling deeply nested directory structures
// that exceed the 260-character limit, which is common in node_modules and
// similar directories with many levels of nesting.
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
	// Note: This assumes the path is already absolute (e.g., C:\path)
	return `\\?\` + path
}
