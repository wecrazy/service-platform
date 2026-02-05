//go:build darwin

package installer

import (
	"fmt"
	"log"
	"os"
	"service-platform/internal/config"
)

func EnsureAdminPrivileges() {
	if os.Geteuid() != 0 {
		log.Fatalln("❌ This operation requires root privileges. Please run this program with sudo.")
	}
}

func Install(yamlCfg *config.TypeConfig) {
	fmt.Println("🍎 macOS detected — but we are sorry, we don't have macOS installer yet")
}
