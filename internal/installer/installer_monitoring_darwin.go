//go:build darwin

package installer

import (
	"fmt"
	"service-platform/internal/config"
)

// EnsureAdminPrivileges checks if the program is running with root privileges and exits if not.
func InstallMonitoring(yamlCfg *config.TypeServicePlatform) {
	_ = yamlCfg // for now we are not using the yamlCfg, but we keep it as a parameter for future implementation
	fmt.Println("🍎 macOS detected — but we are sorry, we don't have macOS monitoring installer yet")
}

// UninstallMonitoring performs the uninstallation process for monitoring on macOS. Currently, it only prints a message indicating that the uninstaller is not available for macOS.
func UninstallMonitoring(yamlCfg *config.TypeServicePlatform) {
	_ = yamlCfg // for now we are not using the yamlCfg, but we keep it as a parameter for future implementation
	fmt.Println("🍎 macOS detected — but we are sorry, we don't have macOS monitoring uninstaller yet")
}
