//go:build linux

package installer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"service-platform/internal/config"
	"strings"
)

func InstallMonitoring(yamlCfg *config.YamlConfig) {
	fmt.Println("🐧 Linux detected — installing monitoring service...")

	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	serviceName := yamlCfg.Monitoring.ServiceName
	if len(serviceName) == 0 || strings.TrimSpace(serviceName) == "" {
		log.Fatalf("Service name cannot be empty or whitespace only")
	}

	workingDir := filepath.Dir(execPath)

	// If running from "bin" directory, set working dir to project root
	if filepath.Base(workingDir) == "bin" {
		workingDir = filepath.Dir(workingDir)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
Type=oneshot
WorkingDirectory=%s
ExecStart=%s --ensure-running
ExecStop=%s --stop
RemainAfterExit=yes
`, yamlCfg.Monitoring.Description, workingDir, execPath, execPath)

	isRoot := os.Geteuid() == 0
	var servicePath string
	var enableCmd, startCmd, daemonReloadCmd *exec.Cmd

	if isRoot {
		// Install as a system-wide service
		serviceContent += "User=root\n\n[Install]\nWantedBy=multi-user.target\n"
		servicePath = fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
		daemonReloadCmd = exec.Command("systemctl", "daemon-reexec")
		enableCmd = exec.Command("systemctl", "enable", serviceName)
		startCmd = exec.Command("systemctl", "start", serviceName)
	} else {
		// Install as a user-level service
		userServiceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
		err := os.MkdirAll(userServiceDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create user systemd directory: %v", err)
		}
		serviceContent += "\n[Install]\nWantedBy=default.target\n"
		servicePath = filepath.Join(userServiceDir, fmt.Sprintf("%s.service", serviceName))
		daemonReloadCmd = exec.Command("systemctl", "--user", "daemon-reexec")
		enableCmd = exec.Command("systemctl", "--user", "enable", serviceName)
		startCmd = exec.Command("systemctl", "--user", "start", serviceName)
	}

	// Check if service is already installed
	checkServiceCmd := exec.Command("systemctl", "status", serviceName)
	if !isRoot {
		checkServiceCmd = exec.Command("systemctl", "--user", "status", serviceName)
	}

	if err := checkServiceCmd.Run(); err == nil {
		fmt.Printf("⚠️  '%s' is already installed and active. Skipping install.\n", serviceName)
		if isRoot {
			fmt.Printf("🔗 To check status: systemctl status %s\n", serviceName)
		} else {
			fmt.Printf("🔗 To check status: systemctl --user status %s\n", serviceName)
		}
		return
	}

	// Write the service file
	err = os.WriteFile(servicePath, []byte(serviceContent), 0644)
	if err != nil {
		log.Fatalf("Failed to write service file: %v", err)
	}

	// Run systemctl commands
	for _, cmd := range []*exec.Cmd{daemonReloadCmd, enableCmd, startCmd} {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to execute command: %v", err)
		}
	}

	fmt.Printf("📦 %s installed and started successfully.\n", serviceName)
	if isRoot {
		fmt.Printf("🔗 Check status using: systemctl status %s\n", serviceName)
	} else {
		fmt.Printf("🔗 Check status using: systemctl --user status %s\n", serviceName)
		fmt.Println("💡 Optional: Run `loginctl enable-linger $(whoami)` to enable service after reboot")
	}
}

func UninstallMonitoring(yamlCfg *config.YamlConfig) {
	fmt.Println("🐧 Linux detected — uninstalling monitoring service...")

	serviceName := yamlCfg.Monitoring.ServiceName
	if len(serviceName) == 0 || strings.TrimSpace(serviceName) == "" {
		log.Fatalf("Service name cannot be empty or whitespace only")
	}

	isRoot := os.Geteuid() == 0
	var servicePath string
	var stopCmd, disableCmd, daemonReloadCmd *exec.Cmd

	if isRoot {
		// System-wide service
		servicePath = fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
		daemonReloadCmd = exec.Command("systemctl", "daemon-reexec")
		disableCmd = exec.Command("systemctl", "disable", serviceName)
		stopCmd = exec.Command("systemctl", "stop", serviceName)
	} else {
		// User-level service
		userServiceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
		servicePath = filepath.Join(userServiceDir, fmt.Sprintf("%s.service", serviceName))
		daemonReloadCmd = exec.Command("systemctl", "--user", "daemon-reexec")
		disableCmd = exec.Command("systemctl", "--user", "disable", serviceName)
		stopCmd = exec.Command("systemctl", "--user", "stop", serviceName)
	}

	// Check if service exists
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		fmt.Printf("⚠️  '%s' is not installed.\n", serviceName)
		return
	}

	// Stop the service if running
	fmt.Printf("🛑 Stopping '%s'...\n", serviceName)
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr
	if err := stopCmd.Run(); err != nil {
		fmt.Printf("⚠️  Failed to stop '%s' (might not be running): %v\n", serviceName, err)
	}

	// Disable the service
	fmt.Printf("🚫 Disabling '%s'...\n", serviceName)
	disableCmd.Stdout = os.Stdout
	disableCmd.Stderr = os.Stderr
	if err := disableCmd.Run(); err != nil {
		fmt.Printf("⚠️  Failed to disable service: %v\n", err)
	}

	// Reload systemd daemon
	daemonReloadCmd.Stdout = os.Stdout
	daemonReloadCmd.Stderr = os.Stderr
	if err := daemonReloadCmd.Run(); err != nil {
		fmt.Printf("⚠️  Failed to reload systemd daemon: %v\n", err)
	}

	// Remove the service file
	if err := os.Remove(servicePath); err != nil {
		log.Fatalf("Failed to remove service file: %v", err)
	}

	fmt.Printf("🗑️  %s uninstalled successfully.\n", serviceName)
	if isRoot {
		fmt.Printf("🔗 Service '%s' has been removed.\n", serviceName)
	} else {
		fmt.Printf("🔗 Service '%s' has been removed.\n", serviceName)
	}
}
