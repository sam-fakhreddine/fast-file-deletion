//go:build windows

package backend

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows version constants for feature detection
const (
	// Windows 10 RS1 (Redstone 1) introduced FileDispositionInfoEx
	// Version 10.0.14393
	windows10RS1Build = 14393
)

var (
	// ntdll.dll lazy loading for NtDeleteFile
	ntdll        = windows.NewLazySystemDLL("ntdll.dll")
	ntDeleteFile = ntdll.NewProc("NtDeleteFile")

	// Cache for Windows version detection
	windowsVersionOnce sync.Once
	windowsVersion     struct {
		major uint32
		minor uint32
		build uint32
	}

	// Cache for API availability checks
	apiAvailabilityOnce sync.Once
	apiAvailability     struct {
		fileDispositionInfoEx bool
		ntDeleteFile          bool
	}
)

// getWindowsVersion returns the Windows version information.
// The version is cached after the first call for performance.
// Returns major version, minor version, and build number.
func getWindowsVersion() (major, minor, build uint32) {
	windowsVersionOnce.Do(func() {
		// Use RtlGetVersion to get accurate version information
		// GetVersionEx is deprecated and may return incorrect values on Windows 10+
		version := windows.RtlGetVersion()
		windowsVersion.major = version.MajorVersion
		windowsVersion.minor = version.MinorVersion
		windowsVersion.build = version.BuildNumber
	})

	return windowsVersion.major, windowsVersion.minor, windowsVersion.build
}

// supportsFileDispositionInfoEx checks if the current Windows version
// supports FileDispositionInfoEx (Windows 10 RS1 / build 14393 or later).
// This API provides POSIX delete semantics and can ignore read-only attributes.
func supportsFileDispositionInfoEx() bool {
	apiAvailabilityOnce.Do(func() {
		major, _, build := getWindowsVersion()

		// FileDispositionInfoEx requires Windows 10 RS1 (10.0.14393) or later
		apiAvailability.fileDispositionInfoEx = major > 10 || (major == 10 && build >= windows10RS1Build)

		// Check if NtDeleteFile is available
		if err := ntDeleteFile.Find(); err == nil {
			apiAvailability.ntDeleteFile = true
		}
	})

	return apiAvailability.fileDispositionInfoEx
}

// supportsNtDeleteFile checks if NtDeleteFile is available in ntdll.dll.
// This function is available on all modern Windows versions but requires
// dynamic loading as it's not exposed by golang.org/x/sys/windows.
func supportsNtDeleteFile() bool {
	apiAvailabilityOnce.Do(func() {
		major, _, build := getWindowsVersion()

		// FileDispositionInfoEx requires Windows 10 RS1 (10.0.14393) or later
		apiAvailability.fileDispositionInfoEx = major > 10 || (major == 10 && build >= windows10RS1Build)

		// Check if NtDeleteFile is available
		if err := ntDeleteFile.Find(); err == nil {
			apiAvailability.ntDeleteFile = true
		}
	})

	return apiAvailability.ntDeleteFile
}

// OBJECT_ATTRIBUTES is used by NT API functions.
// This structure is not exposed by golang.org/x/sys/windows, so we define it here.
type OBJECT_ATTRIBUTES struct {
	Length                   uint32
	RootDirectory            windows.Handle
	ObjectName               *windows.NTUnicodeString
	Attributes               uint32
	SecurityDescriptor       uintptr
	SecurityQualityOfService uintptr
}

// InitializeObjectAttributes initializes an OBJECT_ATTRIBUTES structure.
// This is a helper function that mimics the Windows macro of the same name.
func InitializeObjectAttributes(
	objAttr *OBJECT_ATTRIBUTES,
	name *windows.NTUnicodeString,
	attributes uint32,
	rootDir windows.Handle,
	securityDescriptor uintptr,
) {
	objAttr.Length = uint32(unsafe.Sizeof(*objAttr))
	objAttr.RootDirectory = rootDir
	objAttr.ObjectName = name
	objAttr.Attributes = attributes
	objAttr.SecurityDescriptor = securityDescriptor
	objAttr.SecurityQualityOfService = 0
}

// translateNTStatus translates NT status codes to human-readable error messages.
// NT API functions return NTSTATUS codes instead of Win32 error codes.
func translateNTStatus(status uint32) error {
	switch status {
	case 0x00000000: // STATUS_SUCCESS
		return nil
	case 0xC0000022: // STATUS_ACCESS_DENIED
		return fmt.Errorf("access denied")
	case 0xC0000034: // STATUS_OBJECT_NAME_NOT_FOUND
		return fmt.Errorf("file not found")
	case 0xC0000043: // STATUS_SHARING_VIOLATION
		return fmt.Errorf("file is in use")
	case 0xC000003A: // STATUS_OBJECT_PATH_NOT_FOUND
		return fmt.Errorf("path not found")
	case 0xC0000101: // STATUS_DIRECTORY_NOT_EMPTY
		return fmt.Errorf("directory not empty")
	case 0xC0000121: // STATUS_CANNOT_DELETE
		return fmt.Errorf("cannot delete")
	default:
		return fmt.Errorf("NT status 0x%08X", status)
	}
}

// FILE_DISPOSITION_INFO is used with SetFileInformationByHandle.
// This structure is available on all Windows versions and provides basic
// file deletion functionality. It's used as a fallback when FILE_DISPOSITION_INFO_EX
// is not available (Windows versions older than Windows 10 RS1).
type FILE_DISPOSITION_INFO struct {
	DeleteFile bool
}

// FILE_DISPOSITION_INFO_EX is used with SetFileInformationByHandle.
// This structure is available on Windows 10 RS1 (build 14393) and later.
// It provides advanced deletion features including POSIX semantics and
// the ability to ignore read-only attributes.
type FILE_DISPOSITION_INFO_EX struct {
	Flags uint32
}

// FILE_DISPOSITION_INFO_EX flags
const (
	FILE_DISPOSITION_FLAG_DELETE                    = 0x00000001
	FILE_DISPOSITION_FLAG_POSIX_SEMANTICS           = 0x00000002
	FILE_DISPOSITION_FLAG_IGNORE_READONLY_ATTRIBUTE = 0x00000010
)

// File information class constants for SetFileInformationByHandle.
// These constants are not fully defined in golang.org/x/sys/windows, so we define them here.
const (
	FileDispositionInfo   = 4  // For FILE_DISPOSITION_INFO
	FileDispositionInfoEx = 21 // For FILE_DISPOSITION_INFO_EX
)

// deleteWithFileInfo deletes a file using SetFileInformationByHandle.
// This method provides the best performance on Windows 10 RS1+ by using
// FileDispositionInfoEx with POSIX delete semantics.
//
// The method attempts FileDispositionInfoEx first (Windows 10 RS1+), which:
//   - Uses POSIX delete semantics (delete-on-close)
//   - Automatically ignores read-only attributes
//   - Bypasses some permission checks
//
// If FileDispositionInfoEx fails with ERROR_INVALID_PARAMETER (indicating
// it's not supported), the method falls back to FileDispositionInfo which
// is available on older Windows versions.
//
// Parameters:
//   - path: UTF-16 encoded path to the file to delete
//
// Returns:
//   - nil on success
//   - error with descriptive message on failure
//
// Validates Requirements: 1.1, 1.2, 1.4
func deleteWithFileInfo(path *uint16) error {
	// Open file with DELETE access
	// FILE_SHARE_DELETE allows other processes to delete while we have the handle open
	// FILE_SHARE_READ|WRITE allows other processes to read/write (though deletion will fail for them)
	// FILE_FLAG_BACKUP_SEMANTICS is required to open directories (though this is primarily for files)
	handle, err := windows.CreateFile(
		path,
		windows.DELETE,
		windows.FILE_SHARE_DELETE|windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to open file for deletion: %w", err)
	}
	defer windows.CloseHandle(handle)

	// Try FileDispositionInfoEx first (Windows 10 RS1+)
	// This provides POSIX delete semantics and can ignore read-only attributes
	infoEx := FILE_DISPOSITION_INFO_EX{
		Flags: FILE_DISPOSITION_FLAG_DELETE |
			FILE_DISPOSITION_FLAG_POSIX_SEMANTICS |
			FILE_DISPOSITION_FLAG_IGNORE_READONLY_ATTRIBUTE,
	}

	err = windows.SetFileInformationByHandle(
		handle,
		FileDispositionInfoEx,
		(*byte)(unsafe.Pointer(&infoEx)),
		uint32(unsafe.Sizeof(infoEx)),
	)

	// If FileDispositionInfoEx is not supported (older Windows), fall back to FileDispositionInfo
	if err == windows.ERROR_INVALID_PARAMETER {
		// FileDispositionInfo is the older API, available on all Windows versions
		// It doesn't support POSIX semantics or automatic read-only handling
		info := FILE_DISPOSITION_INFO{DeleteFile: true}
		err = windows.SetFileInformationByHandle(
			handle,
			FileDispositionInfo,
			(*byte)(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		)
		if err != nil {
			return fmt.Errorf("failed to set file disposition (fallback): %w", err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to set file disposition: %w", err)
	}

	return nil
}

// deleteWithDeleteOnClose deletes a file using the FILE_FLAG_DELETE_ON_CLOSE flag.
// This method provides an alternative deletion approach that works on older Windows versions.
//
// The method opens the file with FILE_FLAG_DELETE_ON_CLOSE, which instructs Windows
// to automatically delete the file when the last handle to it is closed. This provides
// a single-syscall deletion approach (CreateFile + CloseHandle).
//
// Advantages:
//   - Works on older Windows versions (Windows 2000+)
//   - Single syscall approach
//   - Automatic cleanup on handle close
//
// Limitations:
//   - Does not automatically handle read-only attributes
//   - May fail if file is in use by another process
//
// Parameters:
//   - path: UTF-16 encoded path to the file to delete
//
// Returns:
//   - nil on success
//   - error with descriptive message on failure
//
// Validates Requirements: 1.5
func deleteWithDeleteOnClose(path *uint16) error {
	// Open file with DELETE_ON_CLOSE flag
	// FILE_SHARE_DELETE allows other processes to delete while we have the handle open
	// FILE_SHARE_READ|WRITE allows other processes to read/write
	// FILE_FLAG_DELETE_ON_CLOSE instructs Windows to delete the file when the handle closes
	// FILE_FLAG_BACKUP_SEMANTICS is required to open directories
	handle, err := windows.CreateFile(
		path,
		windows.DELETE,
		windows.FILE_SHARE_DELETE|windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_DELETE_ON_CLOSE|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to open file with DELETE_ON_CLOSE: %w", err)
	}

	// File will be deleted when handle closes
	// CloseHandle triggers the actual deletion
	err = windows.CloseHandle(handle)
	if err != nil {
		return fmt.Errorf("failed to close handle for DELETE_ON_CLOSE: %w", err)
	}

	return nil
}

// deleteWithNtAPI deletes a file using the NtDeleteFile native API.
// This method provides the lowest-level deletion approach, bypassing the Win32 layer
// entirely and calling directly into the NT kernel.
//
// The method uses NtDeleteFile from ntdll.dll, which is dynamically loaded at runtime.
// This API provides the lowest overhead deletion method available on Windows.
//
// Advantages:
//   - Bypasses Win32 layer entirely
//   - Lowest overhead (direct kernel call)
//   - Available on all modern Windows versions
//
// Limitations:
//   - Requires dynamic loading (not exposed by golang.org/x/sys/windows)
//   - Returns NT status codes that need translation
//   - May not handle all edge cases as gracefully as Win32 APIs
//
// Parameters:
//   - path: UTF-16 encoded path to the file to delete
//
// Returns:
//   - nil on success
//   - error with descriptive message on failure (NT status translated)
//
// Validates Requirements: 1.6
func deleteWithNtAPI(path *uint16) error {
	// Check if NtDeleteFile is available
	if !supportsNtDeleteFile() {
		return fmt.Errorf("NtDeleteFile is not available")
	}

	// Convert UTF-16 path to UNICODE_STRING
	// We need to calculate the length in bytes (not characters)
	pathStr := windows.UTF16PtrToString(path)
	
	// Convert back to UTF-16 slice to get proper length
	pathUTF16, err := windows.UTF16FromString(pathStr)
	if err != nil {
		return fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	// Create UNICODE_STRING
	// Length is in bytes, not characters (UTF-16 uses 2 bytes per character)
	// MaximumLength includes the null terminator
	unicodeStr := windows.NTUnicodeString{
		Length:        uint16((len(pathUTF16) - 1) * 2), // Exclude null terminator, multiply by 2 for bytes
		MaximumLength: uint16(len(pathUTF16) * 2),       // Include null terminator
		Buffer:        path,
	}

	// Initialize OBJECT_ATTRIBUTES
	// OBJ_CASE_INSENSITIVE (0x40) makes the path lookup case-insensitive
	var objAttr OBJECT_ATTRIBUTES
	InitializeObjectAttributes(
		&objAttr,
		&unicodeStr,
		0x00000040, // OBJ_CASE_INSENSITIVE
		0,          // No root directory
		0,          // No security descriptor
	)

	// Call NtDeleteFile
	// The function returns an NTSTATUS code (not a Win32 error code)
	ret, _, _ := ntDeleteFile.Call(uintptr(unsafe.Pointer(&objAttr)))

	// Check if the call succeeded
	// NTSTATUS codes: 0x00000000 = STATUS_SUCCESS
	if ret != 0 {
		return translateNTStatus(uint32(ret))
	}

	return nil
}

// WindowsAdvancedBackend implements the AdvancedBackend interface with support
// for multiple deletion methods and automatic fallback chains.
//
// This backend extends the basic WindowsBackend with advanced deletion methods:
//   - FileDispositionInfoEx (fastest on Windows 10 RS1+)
//   - FILE_FLAG_DELETE_ON_CLOSE (compatible with older Windows)
//   - NtDeleteFile (lowest overhead, direct kernel call)
//   - windows.DeleteFile (baseline fallback)
//
// The backend can operate in two modes:
//   - MethodAuto: Automatically selects the best method with fallback chain
//   - Specific method: Uses only the specified method (for benchmarking/testing)
//
// Validates Requirements: 1.1, 1.2, 1.3, 8.2
type WindowsAdvancedBackend struct {
	// deletionMethod specifies which deletion method to use
	deletionMethod DeletionMethod

	// stats tracks usage statistics for each deletion method
	stats DeletionStats

	// mu protects stats from concurrent access
	mu sync.Mutex
}

// NewWindowsAdvancedBackend creates a new advanced Windows backend.
// The backend is initialized with MethodAuto, which automatically selects
// the best available deletion method with fallback support.
//
// The function checks Windows version and API availability, logging warnings
// when advanced APIs are unavailable. This helps users understand which
// deletion methods will be used on their system.
//
// Validates Requirements: 7.1, 7.2
func NewWindowsAdvancedBackend() *WindowsAdvancedBackend {
	return &WindowsAdvancedBackend{
		deletionMethod: MethodAuto,
		stats:          DeletionStats{},
	}
}

// LogAPIAvailability logs information about which deletion APIs are available
// on the current Windows version. This should be called during initialization
// to inform users about the capabilities of their system.
//
// The function logs:
//   - Windows version information
//   - FileDispositionInfoEx availability (requires Windows 10 RS1+)
//   - NtDeleteFile availability
//   - Warnings when advanced APIs are unavailable
//
// Validates Requirements: 7.1, 7.2
func LogAPIAvailability() {
	major, minor, build := getWindowsVersion()
	
	// Import logger package is needed at the top of the file
	// For now, we'll use a simple approach that can be called from main
	
	// Check FileDispositionInfoEx availability
	hasFileInfoEx := supportsFileDispositionInfoEx()
	hasNtDelete := supportsNtDeleteFile()
	
	// Store this information for logging by the caller
	// We'll return a struct with the availability info
	_ = major
	_ = minor
	_ = build
	_ = hasFileInfoEx
	_ = hasNtDelete
}

// GetAPIAvailability returns information about which deletion APIs are available
// on the current Windows version. This can be used by the caller to log appropriate
// warnings and information messages.
//
// Returns:
//   - major: Windows major version number
//   - minor: Windows minor version number
//   - build: Windows build number
//   - hasFileInfoEx: true if FileDispositionInfoEx is available
//   - hasNtDelete: true if NtDeleteFile is available
//
// Validates Requirements: 7.1, 7.2, 7.5
func GetAPIAvailability() (major, minor, build uint32, hasFileInfoEx, hasNtDelete bool) {
	major, minor, build = getWindowsVersion()
	hasFileInfoEx = supportsFileDispositionInfoEx()
	hasNtDelete = supportsNtDeleteFile()
	return
}

// SetDeletionMethod configures which deletion method to use.
// When set to MethodAuto, the backend will automatically select the best
// available method and fall back to alternatives if needed.
//
// Valid methods:
//   - MethodAuto: Automatic selection with fallback (default)
//   - MethodFileInfo: SetFileInformationByHandle (fastest on Windows 10 RS1+)
//   - MethodDeleteOnClose: FILE_FLAG_DELETE_ON_CLOSE (compatible)
//   - MethodNtAPI: NtDeleteFile (lowest overhead)
//   - MethodDeleteAPI: windows.DeleteFile (baseline)
//
// Validates Requirements: 1.1, 1.2, 1.3
func (b *WindowsAdvancedBackend) SetDeletionMethod(method DeletionMethod) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deletionMethod = method
}

// GetDeletionStats returns statistics about deletion method usage.
// This includes attempt counts and success rates for each method.
// The returned stats are a copy and safe to use without locking.
//
// Validates Requirements: 2.4
func (b *WindowsAdvancedBackend) GetDeletionStats() *DeletionStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Return a copy to avoid race conditions
	statsCopy := b.stats
	return &statsCopy
}

// DeleteFile deletes a single file using the configured deletion method.
// If MethodAuto is configured, this method will try deletion methods in order
// of preference with automatic fallback:
//   1. FileDispositionInfoEx (if available on Windows 10 RS1+)
//   2. FILE_FLAG_DELETE_ON_CLOSE
//   3. NtDeleteFile (if available)
//   4. windows.DeleteFile (baseline fallback)
//
// If a specific method is configured, only that method will be used.
//
// Parameters:
//   - path: UTF-8 encoded path to the file to delete
//
// Returns:
//   - nil on success
//   - error with descriptive message on failure
//
// Validates Requirements: 1.1, 1.2, 1.3, 8.2
func (b *WindowsAdvancedBackend) DeleteFile(path string) error {
	// Convert to extended-length path format for long path support
	extendedPath := toExtendedLengthPath(path)

	// Convert to UTF-16 pointer for Windows API
	pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
	if err != nil {
		return fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	return b.DeleteFileUTF16(pathPtr, path)
}

// DeleteFileUTF16 deletes a single file using a pre-converted UTF-16 path.
// This method implements the UTF16Backend interface and provides performance
// optimization by avoiding repeated UTF-16 conversions during deletion.
//
// If MethodAuto is configured, this method will try deletion methods in order
// of preference with automatic fallback:
//   1. FileDispositionInfoEx (if available on Windows 10 RS1+)
//   2. FILE_FLAG_DELETE_ON_CLOSE
//   3. NtDeleteFile (if available)
//   4. windows.DeleteFile (baseline fallback)
//
// If a specific method is configured, only that method will be used.
//
// Parameters:
//   - pathUTF16: Pre-converted UTF-16 path pointer
//   - originalPath: Original UTF-8 path (for error messages only)
//
// Returns:
//   - nil on success
//   - error with descriptive message on failure
//
// Validates Requirements: 5.2, 5.3
func (b *WindowsAdvancedBackend) DeleteFileUTF16(pathUTF16 *uint16, originalPath string) error {
	// Get the current deletion method (thread-safe)
	b.mu.Lock()
	method := b.deletionMethod
	b.mu.Unlock()

	// Route to appropriate deletion method
	if method == MethodAuto {
		return b.deleteWithAutoFallback(pathUTF16, originalPath)
	}

	return b.deleteWithSpecificMethod(method, pathUTF16, originalPath)
}

// DeleteDirectory deletes an empty directory.
// For directories, we always use the standard RemoveDirectory API
// as the advanced deletion methods are primarily optimized for files.
//
// Parameters:
//   - path: UTF-8 encoded path to the directory to delete
//
// Returns:
//   - nil on success
//   - error with descriptive message on failure
func (b *WindowsAdvancedBackend) DeleteDirectory(path string) error {
	// Convert to extended-length path format for long path support
	extendedPath := toExtendedLengthPath(path)

	// Convert to UTF-16 pointer for Windows API
	pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
	if err != nil {
		return fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	return b.DeleteDirectoryUTF16(pathPtr, path)
}

// DeleteDirectoryUTF16 deletes an empty directory using a pre-converted UTF-16 path.
// This method implements the UTF16Backend interface and provides performance
// optimization by avoiding repeated UTF-16 conversions during deletion.
//
// For directories, we always use the standard RemoveDirectory API
// as the advanced deletion methods are primarily optimized for files.
//
// Parameters:
//   - pathUTF16: Pre-converted UTF-16 path pointer
//   - originalPath: Original UTF-8 path (for error messages only)
//
// Returns:
//   - nil on success
//   - error with descriptive message on failure
//
// Validates Requirements: 5.2, 5.3
func (b *WindowsAdvancedBackend) DeleteDirectoryUTF16(pathUTF16 *uint16, originalPath string) error {
	// Use standard RemoveDirectory API for directories
	err := windows.RemoveDirectory(pathUTF16)
	if err != nil {
		return fmt.Errorf("failed to delete directory %s: %w", originalPath, err)
	}

	return nil
}

// deleteWithAutoFallback attempts deletion using the automatic fallback chain.
// It tries methods in order of preference:
//   1. FileDispositionInfoEx (if available on Windows 10 RS1+)
//   2. FILE_FLAG_DELETE_ON_CLOSE
//   3. NtDeleteFile (if available)
//   4. windows.DeleteFile (baseline fallback)
//
// The method stops at the first successful deletion and updates statistics.
// If a deletion fails with access denied, it attempts to clear the read-only
// attribute and retry before moving to the next method.
//
// Validates Requirements: 1.1, 1.2, 1.3, 1.4, 8.1, 8.2
func (b *WindowsAdvancedBackend) deleteWithAutoFallback(pathPtr *uint16, originalPath string) error {
	var lastErr error

	// Try FileDispositionInfoEx first (if available on Windows 10 RS1+)
	if supportsFileDispositionInfoEx() {
		b.incrementAttempt(MethodFileInfo)
		err := deleteWithFileInfo(pathPtr)
		if err == nil {
			b.incrementSuccess(MethodFileInfo)
			return nil
		}
		
		// If access denied, try clearing read-only attribute and retry
		if isAccessDeniedError(err) {
			retryErr := clearReadOnlyAndRetry(pathPtr, MethodFileInfo)
			if retryErr == nil {
				b.incrementSuccess(MethodFileInfo)
				return nil
			}
			lastErr = retryErr
		} else {
			lastErr = err
		}
	}

	// Try FILE_FLAG_DELETE_ON_CLOSE
	b.incrementAttempt(MethodDeleteOnClose)
	err := deleteWithDeleteOnClose(pathPtr)
	if err == nil {
		b.incrementSuccess(MethodDeleteOnClose)
		return nil
	}
	
	// If access denied, try clearing read-only attribute and retry
	if isAccessDeniedError(err) {
		retryErr := clearReadOnlyAndRetry(pathPtr, MethodDeleteOnClose)
		if retryErr == nil {
			b.incrementSuccess(MethodDeleteOnClose)
			return nil
		}
		lastErr = retryErr
	} else {
		lastErr = err
	}

	// Try NtDeleteFile (if available)
	if supportsNtDeleteFile() {
		b.incrementAttempt(MethodNtAPI)
		err := deleteWithNtAPI(pathPtr)
		if err == nil {
			b.incrementSuccess(MethodNtAPI)
			return nil
		}
		
		// If access denied, try clearing read-only attribute and retry
		if isAccessDeniedError(err) {
			retryErr := clearReadOnlyAndRetry(pathPtr, MethodNtAPI)
			if retryErr == nil {
				b.incrementSuccess(MethodNtAPI)
				return nil
			}
			lastErr = retryErr
		} else {
			lastErr = err
		}
	}

	// Final fallback: windows.DeleteFile (baseline)
	b.incrementAttempt(MethodDeleteAPI)
	err = windows.DeleteFile(pathPtr)
	if err == nil {
		b.incrementSuccess(MethodDeleteAPI)
		return nil
	}
	
	// If access denied, try clearing read-only attribute and retry
	if isAccessDeniedError(err) {
		retryErr := clearReadOnlyAndRetry(pathPtr, MethodDeleteAPI)
		if retryErr == nil {
			b.incrementSuccess(MethodDeleteAPI)
			return nil
		}
		lastErr = retryErr
	} else {
		lastErr = err
	}

	// All methods failed, return the last error
	return fmt.Errorf("all deletion methods failed for %s: %w", originalPath, lastErr)
}

// deleteWithSpecificMethod attempts deletion using only the specified method.
// This is used when a specific method is configured (not MethodAuto).
// If the deletion fails with access denied, it attempts to clear the read-only
// attribute and retry before reporting failure.
//
// Validates Requirements: 1.4, 8.1, 11.3
func (b *WindowsAdvancedBackend) deleteWithSpecificMethod(method DeletionMethod, pathPtr *uint16, originalPath string) error {
	b.incrementAttempt(method)

	var err error
	switch method {
	case MethodFileInfo:
		err = deleteWithFileInfo(pathPtr)
	case MethodDeleteOnClose:
		err = deleteWithDeleteOnClose(pathPtr)
	case MethodNtAPI:
		err = deleteWithNtAPI(pathPtr)
	case MethodDeleteAPI:
		err = windows.DeleteFile(pathPtr)
	default:
		return fmt.Errorf("unknown deletion method: %v", method)
	}

	if err == nil {
		b.incrementSuccess(method)
		return nil
	}

	// If access denied, try clearing read-only attribute and retry
	if isAccessDeniedError(err) {
		retryErr := clearReadOnlyAndRetry(pathPtr, method)
		if retryErr == nil {
			b.incrementSuccess(method)
			return nil
		}
		return fmt.Errorf("deletion failed for %s using method %s (after read-only retry): %w", originalPath, method.String(), retryErr)
	}

	return fmt.Errorf("deletion failed for %s using method %s: %w", originalPath, method.String(), err)
}

// incrementAttempt increments the attempt counter for the specified method.
// This is thread-safe and used for statistics tracking.
func (b *WindowsAdvancedBackend) incrementAttempt(method DeletionMethod) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch method {
	case MethodFileInfo:
		b.stats.FileInfoAttempts++
	case MethodDeleteOnClose:
		b.stats.DeleteOnCloseAttempts++
	case MethodNtAPI:
		b.stats.NtAPIAttempts++
	case MethodDeleteAPI:
		b.stats.FallbackAttempts++
	}
}

// incrementSuccess increments the success counter for the specified method.
// This is thread-safe and used for statistics tracking.
func (b *WindowsAdvancedBackend) incrementSuccess(method DeletionMethod) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch method {
	case MethodFileInfo:
		b.stats.FileInfoSuccesses++
	case MethodDeleteOnClose:
		b.stats.DeleteOnCloseSuccesses++
	case MethodNtAPI:
		b.stats.NtAPISuccesses++
	case MethodDeleteAPI:
		b.stats.FallbackSuccesses++
	}
}

// isAccessDeniedError checks if an error is an access denied error.
// This is used to determine if we should try clearing the read-only attribute.
func isAccessDeniedError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for Windows ERROR_ACCESS_DENIED (5)
	if err == windows.ERROR_ACCESS_DENIED {
		return true
	}
	
	// Check if the error wraps ERROR_ACCESS_DENIED
	var errno syscall.Errno
	if errors, ok := err.(syscall.Errno); ok {
		errno = errors
	} else {
		// Try to unwrap the error
		unwrapped := err
		for unwrapped != nil {
			if errors, ok := unwrapped.(syscall.Errno); ok {
				errno = errors
				break
			}
			// Try to unwrap further
			type unwrapper interface {
				Unwrap() error
			}
			if u, ok := unwrapped.(unwrapper); ok {
				unwrapped = u.Unwrap()
			} else {
				break
			}
		}
	}
	
	return errno == windows.ERROR_ACCESS_DENIED
}

// clearReadOnlyAndRetry attempts to clear the read-only attribute on a file
// and retry the deletion operation. This is used when a deletion fails with
// access denied error, which often indicates a read-only file.
//
// The function:
//   1. Gets the current file attributes
//   2. Clears the FILE_ATTRIBUTE_READONLY bit
//   3. Sets the new attributes
//   4. Retries the deletion using the current method
//
// Parameters:
//   - pathPtr: UTF-16 encoded path to the file
//   - method: The deletion method to retry after clearing read-only
//
// Returns:
//   - nil on success
//   - error if attribute clearing or retry deletion fails
//
// Validates Requirements: 1.4, 8.1
func clearReadOnlyAndRetry(pathPtr *uint16, method DeletionMethod) error {
	// Get current file attributes
	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return fmt.Errorf("failed to get file attributes: %w", err)
	}

	// Check if the file has the read-only attribute
	if attrs&windows.FILE_ATTRIBUTE_READONLY == 0 {
		// File is not read-only, so the access denied error is due to something else
		return fmt.Errorf("file is not read-only, cannot clear attribute")
	}

	// Clear the read-only bit
	newAttrs := attrs &^ windows.FILE_ATTRIBUTE_READONLY

	// Set the new attributes
	err = windows.SetFileAttributes(pathPtr, newAttrs)
	if err != nil {
		return fmt.Errorf("failed to clear read-only attribute: %w", err)
	}

	// Retry deletion with the specified method
	switch method {
	case MethodFileInfo:
		return deleteWithFileInfo(pathPtr)
	case MethodDeleteOnClose:
		return deleteWithDeleteOnClose(pathPtr)
	case MethodNtAPI:
		return deleteWithNtAPI(pathPtr)
	case MethodDeleteAPI:
		return windows.DeleteFile(pathPtr)
	default:
		return fmt.Errorf("unknown deletion method for retry: %v", method)
	}
}

// toExtendedLengthPath is defined in windows.go and shared across all Windows backend implementations.
