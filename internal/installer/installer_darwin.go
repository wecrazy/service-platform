//go:build darwin

package installer

import (
	"fmt"
	"log"
	"os"
	"service-platform/internal/config"
)

// EnsureAdminPrivileges checks if the program is running with root privileges and exits if not.
func EnsureAdminPrivileges() {
	if os.Geteuid() != 0 {
		log.Fatalln("❌ This operation requires root privileges. Please run this program with sudo.")
	}
}

// Install performs the installation process for macOS. Currently, it only prints a message indicating that the installer is not available for macOS.
func Install(yamlCfg *config.TypeServicePlatform) {
	_ = yamlCfg // for now we are not using the yamlCfg, but we keep it as a parameter for future implementation
	fmt.Println("🍎 macOS detected — but we are sorry, we don't have macOS installer yet")
}
