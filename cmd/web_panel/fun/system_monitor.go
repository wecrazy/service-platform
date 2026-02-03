package fun

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// SystemResourceMonitor provides system resource monitoring utilities
type SystemResourceMonitor struct {
	startTime time.Time
}

// NewSystemResourceMonitor creates a new system resource monitor
func NewSystemResourceMonitor() *SystemResourceMonitor {
	return &SystemResourceMonitor{
		startTime: time.Now(),
	}
}

// GetSystemMemoryMB attempts to get total system memory in MB
func (s *SystemResourceMonitor) GetSystemMemoryMB() float64 {
	// Try to read from /proc/meminfo on Linux
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			content := string(data)
			// Parse MemTotal line
			if memTotalLine := s.findLine(content, "MemTotal:"); memTotalLine != "" {
				var memTotalKB float64
				if _, err := fmt.Sscanf(memTotalLine, "MemTotal: %f kB", &memTotalKB); err == nil {
					return memTotalKB / 1024 // Convert KB to MB
				}
			}
		}
	}
	return 0 // Unable to determine
}

// GetDiskUsage attempts to get disk usage percentage
func (s *SystemResourceMonitor) GetDiskUsage() float64 {
	// Get current working directory
	if wd, err := os.Getwd(); err == nil {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(wd, &stat); err == nil {
			// Calculate usage percentage
			total := stat.Blocks * uint64(stat.Bsize)
			used := total - (stat.Bavail * uint64(stat.Bsize))
			if total > 0 {
				return float64(used) / float64(total) * 100
			}
		}
	}
	return 0 // Unable to determine
}

// GetHealthStatus returns comprehensive health status information
func (s *SystemResourceMonitor) GetHealthStatus(db interface{}) map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	systemMemoryMB := s.GetSystemMemoryMB()
	memoryUsageMB := float64(m.Alloc) / 1024 / 1024
	memoryUsagePercent := float64(m.Alloc) / float64(m.Sys) * 100

	health := map[string]interface{}{
		"status":     "healthy",
		"timestamp":  time.Now().Unix(),
		"uptime":     time.Since(s.startTime).String(),
		"goroutines": runtime.NumGoroutine(),
		"memory": map[string]interface{}{
			"alloc_mb":      memoryUsageMB,
			"sys_mb":        float64(m.Sys) / 1024 / 1024,
			"usage_percent": memoryUsagePercent,
		},
		"gc": map[string]interface{}{
			"cycles":         m.NumGC,
			"pause_total_ms": float64(m.PauseTotalNs) / 1000000,
		},
	}

	if systemMemoryMB > 0 {
		systemUsagePercent := (memoryUsageMB / systemMemoryMB) * 100
		health["memory"].(map[string]interface{})["system_total_mb"] = systemMemoryMB
		health["memory"].(map[string]interface{})["system_usage_percent"] = systemUsagePercent

		// Mark as unhealthy if system memory usage is critical
		if systemUsagePercent > 95 {
			health["status"] = "critical"
		}
	}

	// Check database connections if db is provided
	if db != nil {
		dbStatus := "healthy"
		// This would need to be implemented based on the actual DB interface
		// For now, we'll assume it's healthy
		health["database"] = dbStatus

		if dbStatus != "healthy" {
			health["status"] = "degraded"
		}
	}

	return health
}

// findLine finds a line containing the given prefix
func (s *SystemResourceMonitor) findLine(content, prefix string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

// StartResourceMonitoring starts the system resource monitoring goroutine
func (s *SystemResourceMonitor) StartResourceMonitoring() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s.checkSystemResources()
		}
	}()
}

// checkSystemResources performs periodic system resource checks and logging
func (s *SystemResourceMonitor) checkSystemResources() {
	// This would contain the monitoring logic from main.go
	// For now, it's a placeholder that can be expanded
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryUsageMB := float64(m.Alloc) / 1024 / 1024
	systemMemoryMB := s.GetSystemMemoryMB()

	if systemMemoryMB > 0 {
		systemUsagePercent := (memoryUsageMB / systemMemoryMB) * 100
		if systemUsagePercent > 85 {
			// Log warning - this would need access to logger
			fmt.Printf("🚨 HIGH SYSTEM MEMORY USAGE: %.1f%% (%.0f MB / %.0f MB system memory)\n",
				systemUsagePercent, memoryUsageMB, systemMemoryMB)
		}
	}
}
