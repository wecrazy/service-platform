package fun

import (
	"os/exec"
)

// IsPodmanAvailable checks if Podman is installed and available
func IsPodmanAvailable() bool {
	cmd := exec.Command("podman", "--version")
	err := cmd.Run()
	return err == nil
}

// IsContainerRunning checks if a specific container is running
func IsContainerRunning(containerName string) bool {
	cmd := exec.Command("podman", "ps", "-q", "-f", "name="+containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(output) > 0
}

// StopContainer stops and removes a container
func StopContainer(containerName string) error {
	// Stop the container
	stopCmd := exec.Command("podman", "stop", containerName)
	if err := stopCmd.Run(); err != nil {
		return err
	}
	// Remove the container
	rmCmd := exec.Command("podman", "rm", containerName)
	return rmCmd.Run()
}
