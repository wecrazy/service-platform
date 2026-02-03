package fun

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"service-platform/cmd/web_panel/config"
)

// IsWSLRunning checks if any WSL instance is currently running
func IsWSLRunning() (bool, error) {
	if !IsWindows() {
		return false, errors.New("WSL check is only available on Windows")
	}

	cmd := exec.Command("wsl", "--list", "--running")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		// WSL returns non-zero if no instance is running, so this is expected
		return false, nil
	}

	output := out.String()
	return strings.TrimSpace(output) != "", nil
}

// OpenWSL opens WSL only if not already running
func OpenWSL() error {
	if !IsWindows() {
		return errors.New("WSL launch is only available on Windows")
	}

	running, err := IsWSLRunning()
	if err != nil {
		return err
	}
	if running {
		fmt.Println("✅ WSL is already running.")
		return nil
	}

	cmd := exec.Command("powershell", "-Command", "Start-Process wsl")
	fmt.Println("🚀 Launching WSL...")
	return cmd.Start()
}

// IsWindows checks if the current OS is Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsMySQLRunning checks if the XAMPP MySQL process (mysqld.exe) is already running
func IsMySQLRunning() (bool, error) {
	if !IsWindows() {
		return false, fmt.Errorf("%v", "This MySQL check is only available on Windows")
	}

	cmd := exec.Command("tasklist")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return false, fmt.Errorf("failed to run tasklist: %w", err)
	}

	output := out.String()
	return strings.Contains(output, "mysqld.exe"), nil
}

// StartMySQL attempts to start XAMPP's MySQL by running the appropriate .bat or .exe
func StartMySQL() error {
	if !IsWindows() {
		return errors.New("XAMPP MySQL start is only available on Windows")
	}

	running, err := IsMySQLRunning()
	if err != nil {
		return fmt.Errorf("error checking MySQL status: %w", err)
	}
	if running {
		fmt.Println("✅ XAMPP MySQL is already running")
		return nil
	}

	// Normalize the path from config (forward slashes → Windows-safe path)
	mysqlPath := filepath.FromSlash(config.GetConfig().Default.XAMPPMySQLPath)

	cmd := exec.Command("cmd", "/C", mysqlPath)
	return cmd.Start()
}
