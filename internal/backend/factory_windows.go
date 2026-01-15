//go:build windows

package backend

// newPlatformBackend returns the Windows-optimized backend.
// This function is called by NewBackend() and is compiled only on Windows platforms
// due to the build tag. On non-Windows platforms, factory_generic.go provides the implementation instead.
func newPlatformBackend() Backend {
	return NewWindowsBackend()
}
