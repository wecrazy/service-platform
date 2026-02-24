//go:build windows

package fun

import (
	"os"

	"golang.org/x/sys/windows"
)

// GetDiskUsage returns used and total disk space in bytes
func (s *SystemResourceMonitor) GetDiskUsage() (uint64, uint64) {
	wd, err := os.Getwd()
	if err != nil {
		return 0, 0
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64

	ptr, err := windows.UTF16PtrFromString(wd)
	if err != nil {
		return 0, 0
	}

	err = windows.GetDiskFreeSpaceEx(ptr, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
	if err != nil {
		return 0, 0
	}

	return totalBytes - totalFreeBytes, totalBytes
}
