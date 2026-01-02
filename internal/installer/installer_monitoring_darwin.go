//go:build darwin

package installer

import (
	"fmt"
	"service-platform/internal/config"
)

func InstallMonitoring(yamlCfg *config.YamlConfig) {
	fmt.Println("🍎 macOS detected — but we are sorry, we don't have macOS monitoring installer yet")
}

func UninstallMonitoring(yamlCfg *config.YamlConfig) {
	fmt.Println("🍎 macOS detected — but we are sorry, we don't have macOS monitoring uninstaller yet")
}
