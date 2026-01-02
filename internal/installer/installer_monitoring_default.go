//go:build !windows && !linux && !darwin

package installer

import (
	"fmt"
	"runtime"
	"service-platform/internal/config"
)

func InstallMonitoring(yamlCfg *config.YamlConfig) {
	fmt.Printf("⚠️ Unsupported OS for monitoring service installation: %s\n", runtime.GOOS)
}

func UninstallMonitoring(yamlCfg *config.YamlConfig) {
	fmt.Printf("⚠️ Unsupported OS for monitoring service uninstallation: %s\n", runtime.GOOS)
}
