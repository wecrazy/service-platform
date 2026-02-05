//go:build windows

package installer

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"service-platform/internal/config"
	"strings"
)

func InstallMonitoring(yamlCfg *config.TypeConfig) {
	fmt.Println("🪟  Windows detected — installing monitoring service...")

	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	serviceName := yamlCfg.Monitoring.ServiceName
	if len(serviceName) == 0 || strings.TrimSpace(serviceName) == "" {
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
		fmt.Printf("⚠️ '%s' already exists. Status: %s\n", serviceName, strings.TrimSpace(outBuf.String()))
		return
	} else if !strings.Contains(errBuf.String(), "The specified service does not exist") {
		log.Fatalf("Unexpected error checking service status: %v — %s", err, errBuf.String())
	}

	cmd := exec.Command(nssmPath, "install", serviceName, execPath, "--ensure-running")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to install monitoring service via NSSM: %v", err)
	}

	// Set service description
	setDescCmd := exec.Command(nssmPath, "set", serviceName, "Description", yamlCfg.Monitoring.Description)
	setDescCmd.Stdout = os.Stdout
	setDescCmd.Stderr = os.Stderr
	if err := setDescCmd.Run(); err != nil {
		log.Printf("⚠️ Failed to set service description: %v", err)
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

	fmt.Printf("📦 Windows %s installed successfully using NSSM.\n", serviceName)
	fmt.Printf("🔗 Try Win + R, and input services.msc and search for the service name: %s\n", serviceName)

	fmt.Printf("🔄 Starting %s...\n", serviceName)

	cmd = exec.Command(nssmPath, "start", serviceName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to start monitoring service via NSSM: %v", err)
	}

	fmt.Printf("✅ %s started successfully.\n", serviceName)
}

func UninstallMonitoring(yamlCfg *config.TypeConfig) {
	serviceName := yamlCfg.Monitoring.ServiceName
	if len(serviceName) == 0 || strings.TrimSpace(serviceName) == "" {
		log.Fatalf("Service name cannot be empty or whitespace only")
	}

	fmt.Printf("🪟  Windows detected — uninstalling '%s'...\n", serviceName)

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

	if err := checkCmd.Run(); err != nil {
		if strings.Contains(errBuf.String(), "The specified service does not exist") {
			fmt.Printf("⚠️ '%s' is not installed.\n", serviceName)
			return
		} else {
			log.Fatalf("Unexpected error checking service status: %v — %s", err, errBuf.String())
		}
	}

	fmt.Printf("🛑 Stopping '%s'...\n", serviceName)
	stopCmd := exec.Command(nssmPath, "stop", serviceName)
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr
	if err := stopCmd.Run(); err != nil {
		fmt.Printf("⚠️ Failed to stop service (might not be running): %v\n", err)
	}

	fmt.Printf("🗑️ Removing '%s'...\n", serviceName)
	removeCmd := exec.Command(nssmPath, "remove", serviceName, "confirm")
	removeCmd.Stdout = os.Stdout
	removeCmd.Stderr = os.Stderr
	if err := removeCmd.Run(); err != nil {
		log.Fatalf("Failed to remove '%s' via NSSM: %v", serviceName, err)
	}

	fmt.Printf("✅ '%s' uninstalled successfully.\n", serviceName)
}
