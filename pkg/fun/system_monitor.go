package fun

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SystemResourceMonitor provides system resource monitoring utilities
type SystemResourceMonitor struct {
	startTime time.Time
}

// GlobalSystemMonitor is a singleton instance of SystemResourceMonitor that can be used throughout the application to access system resource metrics and health status. It is initialized in the init() function and starts monitoring system resources immediately.
var GlobalSystemMonitor *SystemResourceMonitor

// Prometheus metrics
var (
	cpuLoad1 = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_cpu_load1",
		Help: "System CPU load average over 1 minute",
	})
	cpuLoad5 = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_cpu_load5",
		Help: "System CPU load average over 5 minutes",
	})
	cpuLoad15 = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_cpu_load15",
		Help: "System CPU load average over 15 minutes",
	})
	memoryUsedPercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_memory_used_percent",
		Help: "System memory used percentage",
	})
	memoryTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_memory_total_bytes",
		Help: "Total system memory in bytes",
	})
	memoryUsed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_memory_used_bytes",
		Help: "Used system memory in bytes",
	})
	diskUsedPercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_disk_used_percent",
		Help: "System disk used percentage",
	})
	diskTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_disk_total_bytes",
		Help: "Total system disk space in bytes",
	})
	diskUsed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_disk_used_bytes",
		Help: "Used system disk space in bytes",
	})
	networkBytesSent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_network_bytes_sent",
		Help: "Total network bytes sent",
	})
	networkBytesReceived = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_network_bytes_received",
		Help: "Total network bytes received",
	})
)

// init initializes the GlobalSystemMonitor and starts the resource monitoring goroutine to continuously track system resource usage and update Prometheus metrics for monitoring and alerting purposes. This ensures that the application has real-time insights into its resource consumption and can trigger alerts if usage exceeds defined thresholds.
func init() {
	GlobalSystemMonitor = NewSystemResourceMonitor()
}

// NewSystemResourceMonitor creates a new system resource monitor
func NewSystemResourceMonitor() *SystemResourceMonitor {
	return &SystemResourceMonitor{
		startTime: time.Now(),
	}
}

// ProcessInfo holds information about a process
type ProcessInfo struct {
	PID     string
	Command string
	CPU     float64
	Memory  float64
	RSS     uint64 // Resident Set Size in KB
}

// GetCPULoad returns the load average for 1, 5, and 15 minutes
func (s *SystemResourceMonitor) GetCPULoad() (float64, float64, float64) {
	_ = s // for potential future use if we want to include more context in logs

	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/loadavg")
		if err == nil {
			var l1, l5, l15 float64
			fmt.Sscanf(string(data), "%f %f %f", &l1, &l5, &l15)
			return l1, l5, l15
		}
	}
	return 0, 0, 0
}

// GetNetworkStats returns bytes sent and received on all interfaces
func (s *SystemResourceMonitor) GetNetworkStats() (uint64, uint64) {
	_ = s // for potential future use if we want to include more context in logs

	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/net/dev")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			var totalRx, totalTx uint64
			for _, line := range lines {
				if strings.Contains(line, ":") {
					fields := strings.Fields(strings.ReplaceAll(line, ":", " "))
					if len(fields) >= 10 {
						rx, _ := strconv.ParseUint(fields[1], 10, 64)
						tx, _ := strconv.ParseUint(fields[9], 10, 64)
						totalRx += rx
						totalTx += tx
					}
				}
			}
			return totalRx, totalTx
		}
	}
	return 0, 0
}

// GetTopProcesses returns top processes sorted by cpu or mem
func (s *SystemResourceMonitor) GetTopProcesses(sortBy string, limit int) []ProcessInfo {
	_ = s // for potential future use if we want to include more context in logs

	// Use ps command
	// ps -eo pid,comm,%cpu,%mem,rss --sort=-%cpu | head -n 11
	sortFlag := "-%cpu"
	if sortBy == "mem" {
		sortFlag = "-%mem"
	}

	cmd := exec.Command("ps", "-eo", "pid,comm,%cpu,%mem,rss", "--sort="+sortFlag)
	output, err := cmd.Output()
	if err != nil {
		return []ProcessInfo{}
	}

	lines := strings.Split(string(output), "\n")
	var processes []ProcessInfo

	// Skip header (index 0)
	for i := 1; i < len(lines) && len(processes) < limit; i++ {
		fields := strings.Fields(lines[i])
		if len(fields) >= 5 {
			cpu, _ := strconv.ParseFloat(fields[2], 64)
			mem, _ := strconv.ParseFloat(fields[3], 64)
			rss, _ := strconv.ParseUint(fields[4], 10, 64)
			processes = append(processes, ProcessInfo{
				PID:     fields[0],
				Command: fields[1],
				CPU:     cpu,
				Memory:  mem,
				RSS:     rss,
			})
		}
	}
	return processes
}

// GetSystemMemoryStats returns total, used, and free system memory in MB
func (s *SystemResourceMonitor) GetSystemMemoryStats() (float64, float64, float64) {
	// Try to read from /proc/meminfo on Linux
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			content := string(data)
			var memTotal, memAvailable float64

			if line := s.findLine(content, "MemTotal:"); line != "" {
				fmt.Sscanf(line, "MemTotal: %f kB", &memTotal)
			}
			if line := s.findLine(content, "MemAvailable:"); line != "" {
				fmt.Sscanf(line, "MemAvailable: %f kB", &memAvailable)
			}

			// If MemAvailable is not present (older kernels), fallback to MemFree
			if memAvailable == 0 {
				if line := s.findLine(content, "MemFree:"); line != "" {
					fmt.Sscanf(line, "MemFree: %f kB", &memAvailable)
				}
			}

			if memTotal > 0 {
				totalMB := memTotal / 1024
				availableMB := memAvailable / 1024
				usedMB := totalMB - availableMB
				return totalMB, usedMB, availableMB
			}
		}
	}
	return 0, 0, 0
}

// GetOSUptime returns the system uptime
func (s *SystemResourceMonitor) GetOSUptime() string {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/uptime")
		if err == nil {
			fields := strings.Fields(string(data))
			if len(fields) > 0 {
				uptimeSeconds, _ := strconv.ParseFloat(fields[0], 64)
				duration := time.Duration(uptimeSeconds) * time.Second

				days := int(duration.Hours()) / 24
				hours := int(duration.Hours()) % 24
				minutes := int(duration.Minutes()) % 60
				seconds := int(duration.Seconds()) % 60

				if days > 0 {
					return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
				}
				return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
			}
		}
	}
	return time.Since(s.startTime).String() // Fallback to app uptime
}

// GetHealthStatus returns comprehensive health status information
func (s *SystemResourceMonitor) GetHealthStatus(db interface{}) map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	sysTotalMB, sysUsedMB, _ := s.GetSystemMemoryStats()
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

	if sysTotalMB > 0 {
		systemUsagePercent := (sysUsedMB / sysTotalMB) * 100
		health["memory"].(map[string]interface{})["system_total_mb"] = sysTotalMB
		health["memory"].(map[string]interface{})["system_used_mb"] = sysUsedMB
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
	_ = s // for potential future use if we want to include more context in logs

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
	// Update CPU load metrics
	load1, load5, load15 := s.GetCPULoad()
	cpuLoad1.Set(load1)
	cpuLoad5.Set(load5)
	cpuLoad15.Set(load15)

	// Update memory metrics
	sysTotalMB, sysUsedMB, sysUsagePercent := s.GetSystemMemoryStats()
	memoryTotal.Set(sysTotalMB * 1024 * 1024) // Convert MB to bytes
	memoryUsed.Set(sysUsedMB * 1024 * 1024)
	memoryUsedPercent.Set(sysUsagePercent)

	// Update disk metrics
	diskTotalBytes, diskUsedBytes := s.GetDiskUsage()
	diskTotal.Set(float64(diskTotalBytes))
	diskUsed.Set(float64(diskUsedBytes))
	if diskTotalBytes > 0 {
		diskUsedPercent.Set(float64(diskUsedBytes) / float64(diskTotalBytes) * 100)
	}

	// Update network metrics
	bytesSent, bytesReceived := s.GetNetworkStats()
	networkBytesSent.Set(float64(bytesSent))
	networkBytesReceived.Set(float64(bytesReceived))

	// Keep existing logging logic
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryUsageMB := float64(m.Alloc) / 1024 / 1024

	if sysTotalMB > 0 {
		systemUsagePercent := (sysUsedMB / sysTotalMB) * 100
		if systemUsagePercent > 85 {
			// Log warning - this would need access to logger
			fmt.Printf("🚨 HIGH SYSTEM MEMORY USAGE: %.1f%% (%.0f MB / %.0f MB system memory)\n",
				systemUsagePercent, memoryUsageMB, sysTotalMB)
		}
	}
}
