//go:build !windows && !linux && !darwin

package installer

import (
	"fmt"
	"runtime"
	"service-platform/internal/config"
)

// EnsureAdminPrivileges checks if the program is running with root privileges and exits if not. Currently, it only prints a message indicating that the privilege check is not implemented for this OS.
func InstallMonitoring(yamlCfg *config.TypeServicePlatform) {
	_ = yamlCfg // for now we are not using the yamlCfg, but we keep it as a parameter for future implementation
	fmt.Printf("⚠️ Unsupported OS for monitoring service installation: %s\n", runtime.GOOS)
}

// UninstallMonitoring performs the uninstallation process for monitoring on unsupported OS. Currently, it only prints a message indicating that the uninstaller is not available for this OS.
func UninstallMonitoring(yamlCfg *config.TypeServicePlatform) {
	_ = yamlCfg // for now we are not using the yamlCfg, but we keep it as a parameter for future implementation
	fmt.Printf("⚠️ Unsupported OS for monitoring service uninstallation: %s\n", runtime.GOOS)
}
