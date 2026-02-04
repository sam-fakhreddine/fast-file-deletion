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

// UTF16Backend extends Backend with support for pre-converted UTF-16 paths.
// This interface is optional and provides performance optimization by avoiding
// repeated UTF-16 conversions during deletion. Backends that don't implement
// this interface will fall back to the standard DeleteFile method.
type UTF16Backend interface {
	Backend

	// DeleteFileUTF16 deletes a single file using a pre-converted UTF-16 path.
	// This method avoids the overhead of UTF-16 conversion on every deletion call.
	// The path pointer must remain valid for the duration of the call.
	// Returns an error if the file cannot be deleted.
	DeleteFileUTF16(pathUTF16 *uint16, originalPath string) error

	// DeleteDirectoryUTF16 deletes an empty directory using a pre-converted UTF-16 path.
	// This method avoids the overhead of UTF-16 conversion on every deletion call.
	// The path pointer must remain valid for the duration of the call.
	// Returns an error if the directory cannot be deleted or is not empty.
	DeleteDirectoryUTF16(pathUTF16 *uint16, originalPath string) error
}

// DeletionMethod represents the different deletion methods available on Windows.
// Each method has different performance characteristics and OS version requirements.
type DeletionMethod int

const (
	// MethodAuto automatically selects the best available method with fallback chain.
	// This is the default and recommended mode for most use cases.
	MethodAuto DeletionMethod = iota

	// MethodFileInfo uses SetFileInformationByHandle with FileDispositionInfoEx/FileDispositionInfo.
	// This is the fastest method on Windows 10 RS1+ and supports POSIX delete semantics.
	// Falls back to FileDispositionInfo on older Windows versions.
	MethodFileInfo

	// MethodDeleteOnClose uses FILE_FLAG_DELETE_ON_CLOSE flag with CreateFile.
	// The file is deleted when the handle is closed. Works on all Windows versions.
	MethodDeleteOnClose

	// MethodNtAPI uses the native NtDeleteFile API from ntdll.dll.
	// This bypasses the Win32 layer entirely for lowest overhead.
	// Requires dynamic loading as it's not exposed by golang.org/x/sys/windows.
	MethodNtAPI

	// MethodDeleteAPI uses the standard windows.DeleteFile API (baseline).
	// This is the fallback method used by the original implementation.
	MethodDeleteAPI
)

// String returns the string representation of the deletion method.
func (m DeletionMethod) String() string {
	switch m {
	case MethodAuto:
		return "auto"
	case MethodFileInfo:
		return "fileinfo"
	case MethodDeleteOnClose:
		return "deleteonclose"
	case MethodNtAPI:
		return "ntapi"
	case MethodDeleteAPI:
		return "deleteapi"
	default:
		return "unknown"
	}
}

// DeletionStats tracks statistics about which deletion methods were used
// and their success rates. This is useful for benchmarking and optimization.
type DeletionStats struct {
	// FileInfo method statistics
	FileInfoAttempts  int
	FileInfoSuccesses int

	// DeleteOnClose method statistics
	DeleteOnCloseAttempts  int
	DeleteOnCloseSuccesses int

	// NtAPI method statistics
	NtAPIAttempts  int
	NtAPISuccesses int

	// Fallback (DeleteAPI) method statistics
	FallbackAttempts  int
	FallbackSuccesses int
}

// AdvancedBackend extends the Backend interface with optimization features.
// This interface is implemented by backends that support multiple deletion methods
// and provide detailed statistics about their usage.
type AdvancedBackend interface {
	Backend

	// SetDeletionMethod configures which deletion method to use.
	// When set to MethodAuto, the backend will automatically select the best
	// available method and fall back to alternatives if needed.
	SetDeletionMethod(method DeletionMethod)

	// GetDeletionStats returns statistics about deletion method usage.
	// This includes attempt counts and success rates for each method.
	GetDeletionStats() *DeletionStats
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
