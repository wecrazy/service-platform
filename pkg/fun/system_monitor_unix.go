//go:build !windows

package fun

import (
	"os"
	"syscall"
)

// GetDiskUsage returns used and total disk space in bytes
func (s *SystemResourceMonitor) GetDiskUsage() (uint64, uint64) {
	_ = s // for now we don't need the receiver, but we could log it if needed

	// Get current working directory
	if wd, err := os.Getwd(); err == nil {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(wd, &stat); err == nil {
			total := stat.Blocks * uint64(stat.Bsize)
			free := stat.Bavail * uint64(stat.Bsize)
			used := total - free
			return used, total
		}
	}
	return 0, 0
}
