//go:build !windows

package monitor

// WindowsMonitor is a stub for non-Windows platforms.
// On non-Windows systems, it behaves identically to Monitor.
type WindowsMonitor struct {
	*Monitor
}

// NewWindowsMonitor creates a new monitor (generic version for non-Windows).
func NewWindowsMonitor() *WindowsMonitor {
	return &WindowsMonitor{
		Monitor: NewMonitor(),
	}
}
