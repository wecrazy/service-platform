package app_installer

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

func Init() bool {
	// Define command-line flag for install
	installFlag := flag.Bool("install", false, "Install the service")

	// Parse the flags
	flag.Parse()

	// If the install flag is provided, attempt to install the service
	if *installFlag {
		// Get the current directory
		exePath, errE := os.Executable()
		if errE != nil {
			log.Fatalf("Error getting executable path: %v", errE)
		}

		currentDir := filepath.Dir(exePath)
		errInstall := installService("TA Web Dashboard", currentDir)
		if errInstall != nil {
			fmt.Printf("Error installing service: %v\n", errInstall)
			os.Exit(1)
		}
		return true
	}
	return false
}
func installService(serviceDesc, currentDir string) error {
	// Get the current user
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("error getting current user: %v", err)
	}

	// Derive the service name from the current directory name
	_, folderName := filepath.Split(currentDir)
	folderName = strings.Trim(folderName, "/\\")
	serviceName := folderName
	// serviceName := "mqtt_broker_paxlite"

	// Check if 'main' and '.env' files exist in the current directory
	mainPath := filepath.Join(currentDir, "main")
	envPath := filepath.Join(currentDir, ".env")

	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		return fmt.Errorf("'main' file not found in the current directory")
	}

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("'.env' file not found in the current directory")
	}

	// Define the service file path
	serviceFilePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	// Create the service file content
	serviceFileContent := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=3
EnvironmentFile=%s

[Install]
WantedBy=multi-user.target
`, serviceDesc, currentUser.Username, currentUser.Username, currentDir, mainPath, envPath)

	// Write the service file
	if err := os.WriteFile(serviceFilePath, []byte(serviceFileContent), 0644); err != nil {
		return fmt.Errorf("error writing service file: %v", err)
	}

	fmt.Printf("Service file created at %s\n", serviceFilePath)

	// Reload systemd daemon
	if err := exec.Command("sudo", "systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("error reloading systemd daemon: %v", err)
	}
	fmt.Println("Systemd daemon reloaded.")

	// Enable the service
	if err := exec.Command("sudo", "systemctl", "enable", fmt.Sprintf("%s.service", serviceName)).Run(); err != nil {
		return fmt.Errorf("error enabling service: %v", err)
	}
	fmt.Println("Service enabled.")

	// Restart the service
	if err := exec.Command("sudo", "systemctl", "restart", fmt.Sprintf("%s.service", serviceName)).Run(); err != nil {
		return fmt.Errorf("error restarting service: %v", err)
	}
	fmt.Println("Service restarted.")

	return nil
}
