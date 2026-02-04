//go:build !windows

package main

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// getTotalSystemMemory returns the total physical memory in bytes on Unix-like systems.
// Uses platform-specific APIs (sysctl on macOS, /proc/meminfo on Linux).
func getTotalSystemMemory() int64 {
	switch runtime.GOOS {
	case "darwin":
		return getTotalMemoryDarwin()
	case "linux":
		return getTotalMemoryLinux()
	default:
		return 0
	}
}

// getTotalMemoryDarwin gets total memory on macOS using sysctl.
func getTotalMemoryDarwin() int64 {
	// On macOS, use sysctl hw.memsize
	var memsize int64
	mib := [2]int32{6 /* CTL_HW */, 24 /* HW_MEMSIZE */}
	n := uintptr(8) // size of int64

	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		2,
		uintptr(unsafe.Pointer(&memsize)),
		uintptr(unsafe.Pointer(&n)),
		0,
		0,
	)

	if errno != 0 {
		return 0
	}

	return memsize
}

// getTotalMemoryLinux gets total memory on Linux by reading /proc/meminfo.
func getTotalMemoryLinux() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	// Parse MemTotal from /proc/meminfo
	// Format: "MemTotal:       32940268 kB"
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil {
					return kb * 1024 // Convert KB to bytes
				}
			}
		}
	}

	return 0
}
