package installer

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"service-platform/internal/config"
	"strings"
)

func EnsureAdminPrivileges() {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("net", "session")
		// Hide output
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			log.Fatalln("❌ This operation requires administrator privileges. Please run this program as Administrator.")
		}
	case "linux", "darwin":
		if os.Geteuid() != 0 {
			log.Fatalln("❌ This operation requires root privileges. Please run this program with sudo.")
		}
	default:
		fmt.Println("⚠️  Privilege check not implemented for this OS.")
	}
}

func WindowsService(yamlCfg *config.YamlConfig) {
	fmt.Println("🪟  Windows detected — running Windows install steps...")
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	serviceName := yamlCfg.App.Name
	serviceName = strings.ReplaceAll(strings.TrimSpace(serviceName), " ", "")
	if len(serviceName) == 0 {
		log.Fatalf("Service name cannot be empty or whitespace only")
	}

	nssmPath, err := exec.LookPath("nssm")
	if err != nil {
		nssmPath = yamlCfg.Default.NSSMFullPath
		if _, err := os.Stat(nssmPath); os.IsNotExist(err) {
			log.Fatalf("NSSM not found in PATH and default path is invalid: %v", err)
		}
	}

	// ✅ Check if service exists
	checkCmd := exec.Command(nssmPath, "status", serviceName)
	var outBuf, errBuf bytes.Buffer
	checkCmd.Stdout = &outBuf
	checkCmd.Stderr = &errBuf

	if err := checkCmd.Run(); err == nil {
		fmt.Printf("⚠️ Service '%s' already exists. Status: %s\n", serviceName, strings.TrimSpace(outBuf.String()))
		return
	} else if !strings.Contains(errBuf.String(), "The specified service does not exist") {
		log.Fatalf("Unexpected error checking service status: %v — %s", err, errBuf.String())
	}

	cmd := exec.Command(nssmPath, "install", serviceName, execPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to install service via NSSM: %v", err)
	}

	// If running from "bin", set AppDirectory to project root
	workingDir := filepath.Dir(execPath)
	if filepath.Base(workingDir) == "bin" {
		parentDir := filepath.Dir(workingDir)
		setDirCmd := exec.Command(nssmPath, "set", serviceName, "AppDirectory", parentDir)
		if err := setDirCmd.Run(); err != nil {
			log.Printf("⚠️ Failed to set AppDirectory: %v", err)
		}
	}

	fmt.Println("📦 Windows service installed successfully using NSSM.")
	fmt.Printf("🔗 Try Win + R, and input services.msc and search for the service name: %s\n", serviceName)

	fmt.Printf("🔄 Starting service %s...\n", serviceName)

	cmd = exec.Command(nssmPath, "start", serviceName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to start service via NSSM: %v", err)
	}

	fmt.Printf("✅ Service: %s started successfully.\n", serviceName)
}

func LinuxService(yamlCfg *config.YamlConfig) {
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
`, yamlCfg.App.Name, workingDir, execPath)

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
