//go:build darwin

package main

import (
	"syscall"
	"unsafe"
)

// getTotalSystemMemory returns the total physical memory in bytes on macOS.
func getTotalSystemMemory() int64 {
	return getTotalMemoryDarwin()
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
