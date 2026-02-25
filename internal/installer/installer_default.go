//go:build !windows && !linux && !darwin

package installer

import (
	"fmt"
	"runtime"
	"service-platform/internal/config"
)

// EnsureAdminPrivileges checks if the program is running with root privileges and exits if not. Currently, it only prints a message indicating that the privilege check is not implemented for this OS.
func EnsureAdminPrivileges() {
	fmt.Println("⚠️  Privilege check not implemented for this OS.")
}

// Install performs the installation process for unsupported OS. It currently only prints a message indicating that the installer is not available for this OS.
func Install(yamlCfg *config.TypeServicePlatform) {
	_ = yamlCfg // for now we are not using the yamlCfg, but we keep it as a parameter for future implementation
	fmt.Printf("⚠️ Unsupported OS: %s\n", runtime.GOOS)
}
