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

func EnsureAdminPrivileges() {
	cmd := exec.Command("net", "session")
	// Hide output
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		log.Fatalln("❌ This operation requires administrator privileges. Please run this program as Administrator.")
	}
}

func Install(yamlCfg *config.YamlConfig) {
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
