//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// MEMORYSTATUSEX structure for GlobalMemoryStatusEx API
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

// getTotalSystemMemory returns the total physical memory in bytes on Windows.
// Uses GlobalMemoryStatusEx API to get accurate system memory information.
func getTotalSystemMemory() int64 {
	var memStatus memoryStatusEx
	memStatus.Length = uint32(unsafe.Sizeof(memStatus))

	// Call GlobalMemoryStatusEx
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		// If API call fails, return 0 to skip memory limit setting
		return 0
	}

	return int64(memStatus.TotalPhys)
}
