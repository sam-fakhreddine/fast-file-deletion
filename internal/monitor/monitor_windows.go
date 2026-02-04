//go:build windows

package monitor

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemTimes      = kernel32.NewProc("GetSystemTimes")
	procGetProcessIoCounters = kernel32.NewProc("GetProcessIoCounters")
)

// IO_COUNTERS structure for process I/O statistics
type IO_COUNTERS struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

// WindowsMetrics contains Windows-specific performance metrics.
type WindowsMetrics struct {
	// CPU metrics
	CPUUsagePercent float64 // Actual CPU usage percentage (0-100)

	// Disk I/O metrics
	ReadOpsPerSec  float64 // Disk read operations per second
	WriteOpsPerSec float64 // Disk write operations per second
	ReadMBPerSec   float64 // Disk read throughput in MB/sec
	WriteMBPerSec  float64 // Disk write throughput in MB/sec

	// I/O pressure indicators
	IOSaturated bool // True if I/O operations are very high
}

// WindowsMonitor extends Monitor with Windows-specific metrics.
type WindowsMonitor struct {
	*Monitor
	lastIdleTime   uint64
	lastKernelTime uint64
	lastUserTime   uint64
	lastIOCounters IO_COUNTERS
	lastIOTime     time.Time
}

// NewWindowsMonitor creates a new Windows-specific monitor.
func NewWindowsMonitor() *WindowsMonitor {
	return &WindowsMonitor{
		Monitor:    NewMonitor(),
		lastIOTime: time.Now(),
	}
}

// collectWindowsMetrics gathers Windows-specific performance metrics.
func (m *WindowsMonitor) collectWindowsMetrics() (WindowsMetrics, error) {
	cpuPercent, err := m.getCPUUsage()
	if err != nil {
		cpuPercent = 0 // Fall back to 0 if we can't get CPU usage
	}

	ioMetrics, err := m.getIOMetrics()
	if err != nil {
		// Fall back to empty metrics if we can't get I/O stats
		ioMetrics = WindowsMetrics{}
	}

	ioMetrics.CPUUsagePercent = cpuPercent

	// Detect I/O saturation (>10,000 ops/sec is very high for file deletion)
	ioMetrics.IOSaturated = (ioMetrics.WriteOpsPerSec + ioMetrics.ReadOpsPerSec) > 10000

	return ioMetrics, nil
}

// getCPUUsage returns the current CPU usage percentage using GetSystemTimes.
func (m *WindowsMonitor) getCPUUsage() (float64, error) {
	var idleTime, kernelTime, userTime windows.Filetime

	ret, _, err := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)

	if ret == 0 {
		return 0, fmt.Errorf("GetSystemTimes failed: %v", err)
	}

	// Convert FILETIME to uint64 (100-nanosecond intervals)
	idle := uint64(idleTime.HighDateTime)<<32 | uint64(idleTime.LowDateTime)
	kernel := uint64(kernelTime.HighDateTime)<<32 | uint64(kernelTime.LowDateTime)
	user := uint64(userTime.HighDateTime)<<32 | uint64(userTime.LowDateTime)

	// Calculate deltas
	if m.lastIdleTime == 0 {
		// First measurement, just store values
		m.lastIdleTime = idle
		m.lastKernelTime = kernel
		m.lastUserTime = user
		return 0, nil
	}

	idleDelta := idle - m.lastIdleTime
	kernelDelta := kernel - m.lastKernelTime
	userDelta := user - m.lastUserTime

	// Kernel time includes idle time, so subtract it
	systemDelta := kernelDelta + userDelta - idleDelta

	// Calculate CPU usage percentage
	totalDelta := kernelDelta + userDelta
	cpuPercent := 0.0
	if totalDelta > 0 {
		cpuPercent = (float64(systemDelta) / float64(totalDelta)) * 100.0
	}

	// Update last values
	m.lastIdleTime = idle
	m.lastKernelTime = kernel
	m.lastUserTime = user

	return cpuPercent, nil
}

// getIOMetrics returns disk I/O metrics for the current process.
func (m *WindowsMonitor) getIOMetrics() (WindowsMetrics, error) {
	var ioCounters IO_COUNTERS

	handle, err := windows.GetCurrentProcess()
	if err != nil {
		return WindowsMetrics{}, fmt.Errorf("GetCurrentProcess failed: %v", err)
	}

	ret, _, err := procGetProcessIoCounters.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&ioCounters)),
	)

	if ret == 0 {
		return WindowsMetrics{}, fmt.Errorf("GetProcessIoCounters failed: %v", err)
	}

	now := time.Now()
	elapsed := now.Sub(m.lastIOTime).Seconds()

	metrics := WindowsMetrics{}

	if m.lastIOCounters.ReadOperationCount > 0 && elapsed > 0 {
		// Calculate operations per second
		readOpsDelta := ioCounters.ReadOperationCount - m.lastIOCounters.ReadOperationCount
		writeOpsDelta := ioCounters.WriteOperationCount - m.lastIOCounters.WriteOperationCount

		metrics.ReadOpsPerSec = float64(readOpsDelta) / elapsed
		metrics.WriteOpsPerSec = float64(writeOpsDelta) / elapsed

		// Calculate throughput in MB/sec
		readBytesDelta := ioCounters.ReadTransferCount - m.lastIOCounters.ReadTransferCount
		writeBytesDelta := ioCounters.WriteTransferCount - m.lastIOCounters.WriteTransferCount

		metrics.ReadMBPerSec = (float64(readBytesDelta) / elapsed) / (1024 * 1024)
		metrics.WriteMBPerSec = (float64(writeBytesDelta) / elapsed) / (1024 * 1024)
	}

	// Update last values
	m.lastIOCounters = ioCounters
	m.lastIOTime = now

	return metrics, nil
}

// LogWindowsMetrics logs Windows-specific performance metrics.
func (m *WindowsMonitor) LogWindowsMetrics(metrics WindowsMetrics) {
	if metrics.IOSaturated {
		fmt.Printf("\n⚠️  I/O SATURATION DETECTED:\n")
		fmt.Printf("   CPU Usage:     %.1f%%\n", metrics.CPUUsagePercent)
		fmt.Printf("   Read Ops/sec:  %.0f\n", metrics.ReadOpsPerSec)
		fmt.Printf("   Write Ops/sec: %.0f\n", metrics.WriteOpsPerSec)
		fmt.Printf("   Read MB/sec:   %.2f\n", metrics.ReadMBPerSec)
		fmt.Printf("   Write MB/sec:  %.2f\n\n", metrics.WriteMBPerSec)
	}
}
