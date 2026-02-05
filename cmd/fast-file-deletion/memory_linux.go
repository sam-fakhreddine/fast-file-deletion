//go:build linux

package main

import (
	"os"
	"strconv"
	"strings"
)

// getTotalSystemMemory returns the total physical memory in bytes on Linux.
func getTotalSystemMemory() int64 {
	return getTotalMemoryLinux()
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
