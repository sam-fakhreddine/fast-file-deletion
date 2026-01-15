//go:build !windows

// Package backend provides cross-platform file deletion using standard Go operations.
package backend

import (
	"fmt"
	"os"

	"github.com/yourusername/fast-file-deletion/internal/logger"
)

// GenericBackend provides cross-platform file deletion using standard Go operations.
// This backend works on all platforms (Linux, macOS, BSD, etc.) but may not be
// as optimized as platform-specific backends like WindowsBackend.
//
// It uses Go's standard library functions (os.Remove) which provide:
//   - Cross-platform compatibility
//   - Reliable operation on all supported platforms
//   - Simpler implementation without platform-specific syscalls
type GenericBackend struct{}

// NewGenericBackend creates a new generic cross-platform backend.
func NewGenericBackend() *GenericBackend {
	return &GenericBackend{}
}

// DeleteFile deletes a single file using os.Remove.
// This is the standard Go approach to file deletion and works reliably
// across all platforms. Returns an error if the file cannot be deleted.
func (b *GenericBackend) DeleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		logger.Debug("os.Remove failed for file: %s (error: %v)", path, err)
		return fmt.Errorf("failed to delete file %s: %w", path, err)
	}
	return nil
}

// DeleteDirectory deletes an empty directory using os.Remove.
// The directory must be empty before calling this method.
// Returns an error if the directory cannot be deleted or is not empty.
func (b *GenericBackend) DeleteDirectory(path string) error {
	err := os.Remove(path)
	if err != nil {
		logger.Debug("os.Remove failed for directory: %s (error: %v)", path, err)
		return fmt.Errorf("failed to delete directory %s: %w", path, err)
	}
	return nil
}
