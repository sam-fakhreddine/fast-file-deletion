//go:build !windows

package backend

// newPlatformBackend returns the generic cross-platform backend.
// This function is called by NewBackend() and is compiled only on non-Windows platforms
// due to the build tag. On Windows, factory_windows.go provides the implementation instead.
func newPlatformBackend() Backend {
	return NewGenericBackend()
}
