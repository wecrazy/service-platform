// Package main is the entry point for the monitoring management tool.
//
// It orchestrates Prometheus, Grafana, Loki, and Tempo via shell scripts located
// in the scripts/ directory. The tool can also install/uninstall itself as a
// system service.
//
// When invoked without arguments it behaves like --ensure-running.
//
// Commands:
//
//	--install          Install monitoring as a system service (requires root)
//	--uninstall        Uninstall the system service (aliases: --delete, --remove)
//	--ensure-running   Start monitoring if not already running (default)
//	--stop             Stop monitoring services
//	--help, -h         Show help
//
// Usage:
//
//	go run cmd/monitoring/main.go --install
//	make monitoring-start
//	./bin/monitoring --stop
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"service-platform/internal/config"
	"service-platform/internal/installer"
	"service-platform/pkg/logger"
)

// main loads configuration, then dispatches to the appropriate sub-command
// based on CLI arguments. Defaults to ensureMonitoringRunning when no args given.
func main() {
	// Load config
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	yamlCfg := config.ServicePlatform.Get()

	// Init log
	logger.InitLogrus()

	// Handle CLI args
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--install":
			fmt.Println("🔧 Installing monitoring service...")
			installer.EnsureAdminPrivileges()
			installer.InstallMonitoring(&yamlCfg)
			return
		case "--uninstall", "--delete", "--remove":
			fmt.Println("🗑️  Uninstalling monitoring service...")
			installer.EnsureAdminPrivileges()
			installer.UninstallMonitoring(&yamlCfg)
			return
		case "--ensure-running":
			ensureMonitoringRunning()
			return
		case "--stop":
			stopMonitoring()
			return
		case "--help", "-h":
			printHelp()
			return
		default:
			fmt.Printf("Unknown argument: %s\n", os.Args[1])
			printHelp()
			os.Exit(1)
		}
	}

	// If no args, just run ensure-running
	ensureMonitoringRunning()
}

// ensureMonitoringRunning starts monitoring services if they are not already running.
func ensureMonitoringRunning() {
	fmt.Println("🚀 Starting Service Platform Monitoring...")
	startMonitoring()
	fmt.Println("✅ Monitoring started successfully")
}

// cleanupMonitoring runs the cleanup-monitoring.sh script to remove stale data.
func cleanupMonitoring() {
	scriptPath := filepath.Join(getProjectRoot(), "scripts", "cleanup-monitoring.sh")
	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to run cleanup script: %v", err)
	}
}

// startMonitoring executes scripts/start-monitoring.sh to launch all monitoring containers.
func startMonitoring() {
	scriptPath := filepath.Join(getProjectRoot(), "scripts", "start-monitoring.sh")
	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to run start script: %v", err)
	}
}

// stopMonitoring executes scripts/stop-monitoring.sh to gracefully stop all monitoring containers.
func stopMonitoring() {
	fmt.Println("🛑 Stopping Service Platform Monitoring...")
	scriptPath := filepath.Join(getProjectRoot(), "scripts", "stop-monitoring.sh")
	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to run stop script: %v", err)
	}
	fmt.Println("✅ Monitoring stopped successfully")
}

// getProjectRoot returns the project root directory by inspecting the executable
// path. If the binary lives in a "bin" sub-directory, the parent is returned.
func getProjectRoot() string {
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	workingDir := filepath.Dir(execPath)
	// If running from "bin" directory, set working dir to project root
	if filepath.Base(workingDir) == "bin" {
		workingDir = filepath.Dir(workingDir)
	}
	return workingDir
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Println("Service Platform - Monitoring Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  monitoring [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  --install         Install monitoring as a system service")
	fmt.Println("  --uninstall       Uninstall monitoring system service")
	fmt.Println("  --delete          Alias for --uninstall")
	fmt.Println("  --remove          Alias for --uninstall")
	fmt.Println("  --ensure-running  Ensure monitoring services are running")
	fmt.Println("  --stop            Stop monitoring services")
	fmt.Println("  --help, -h        Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  monitoring --install")
	fmt.Println("  monitoring --ensure-running")
	fmt.Println("  monitoring --uninstall")
}
