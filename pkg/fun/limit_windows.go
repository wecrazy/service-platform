//go:build windows

package fun

import "fmt"

// IncreaseFileDescriptorLimit is a no-op on Windows as it handles file descriptors differently.
func IncreaseFileDescriptorLimit() {
	fmt.Println("ℹ️  Skipping file descriptor limit check (Windows detected).")
}
