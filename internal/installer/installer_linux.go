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

func EnsureAdminPrivileges() {
	if os.Geteuid() != 0 {
		log.Fatalln("❌ This operation requires root privileges. Please run this program with sudo.")
	}
}

func Install(yamlCfg *config.YamlConfig) {
	fmt.Println("🐧 Linux detected — running Linux install steps...")

	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	serviceName := yamlCfg.App.Name
	serviceName = strings.ReplaceAll(strings.TrimSpace(serviceName), " ", "")
	if len(serviceName) == 0 {
		log.Fatalf("Service name cannot be empty or whitespace only")
	}

	workingDir := filepath.Dir(execPath)

	// If running from "bin" directory, set working dir to project root
	// so that config/ and web/ folders are accessible.
	if filepath.Base(workingDir) == "bin" {
		workingDir = filepath.Dir(workingDir)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
WorkingDirectory=%s
ExecStart=%s
Restart=always
`, yamlCfg.App.Description, workingDir, execPath)

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
		fmt.Printf("⚠️  Service '%s' is already installed and active. Skipping install.\n", serviceName)
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

	fmt.Println("📦 Linux service installed and started successfully.")
	if isRoot {
		fmt.Printf("🔗 Check status using: systemctl status %s\n", serviceName)
	} else {
		fmt.Printf("🔗 Check status using: systemctl --user status %s\n", serviceName)
		fmt.Println("💡 Optional: Run `loginctl enable-linger $(whoami)` to enable service after reboot")
	}
}
