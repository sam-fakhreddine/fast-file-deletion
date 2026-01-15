// Package backend provides platform-specific deletion implementations.
// It defines the Backend interface and provides factory functions to create
// the appropriate backend for the current operating system.
package backend

// Backend defines the interface for platform-specific deletion implementations.
// Different backends can optimize deletion for specific operating systems.
// On Windows, the WindowsBackend uses direct Win32 API calls for better performance.
// On other platforms, the GenericBackend uses standard Go file operations.
type Backend interface {
	// DeleteFile deletes a single file at the specified path.
	// This method should only be used for files, not directories.
	// Returns an error if the file cannot be deleted (e.g., permission denied, file locked).
	DeleteFile(path string) error

	// DeleteDirectory deletes an empty directory at the specified path.
	// The directory must be empty before calling this method.
	// Returns an error if the directory cannot be deleted or is not empty.
	DeleteDirectory(path string) error
}

// NewBackend creates and returns the appropriate backend for the current platform.
// The backend selection is done at compile time using build tags:
//   - On Windows: Returns WindowsBackend (uses Win32 API for optimized performance)
//   - On other platforms: Returns GenericBackend (uses standard Go file operations)
//
// This function delegates to newPlatformBackend(), which is implemented differently
// for each platform using build tags (see factory_windows.go and factory_generic.go).
func NewBackend() Backend {
	return newPlatformBackend()
}
