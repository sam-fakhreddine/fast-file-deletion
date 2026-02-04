//go:build !windows

package backend

// newPlatformBackend returns the generic cross-platform backend.
// This function is called by NewBackend() and is compiled only on non-Windows platforms
// due to the build tag. On Windows, factory_windows.go provides the implementation instead.
func newPlatformBackend() Backend {
	return NewGenericBackend()
}

// GetAPIAvailability returns stub information for non-Windows platforms.
// On non-Windows platforms, advanced Windows APIs are not available.
// This function exists to maintain API compatibility across platforms.
//
// Returns:
//   - major: 0 (not applicable on non-Windows)
//   - minor: 0 (not applicable on non-Windows)
//   - build: 0 (not applicable on non-Windows)
//   - hasFileInfoEx: false (Windows-only API)
//   - hasNtDelete: false (Windows-only API)
func GetAPIAvailability() (major, minor, build uint32, hasFileInfoEx, hasNtDelete bool) {
	return 0, 0, 0, false, false
}
