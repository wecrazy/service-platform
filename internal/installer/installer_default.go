//go:build !windows && !linux && !darwin

package installer

import (
	"fmt"
	"runtime"
	"service-platform/internal/config"
)

func EnsureAdminPrivileges() {
	fmt.Println("⚠️  Privilege check not implemented for this OS.")
}

func Install(yamlCfg *config.YamlConfig) {
	fmt.Printf("⚠️ Unsupported OS: %s\n", runtime.GOOS)
}
