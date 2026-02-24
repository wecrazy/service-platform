//go:build !windows

package fun

import (
	"fmt"
	"syscall"
)

// IncreaseFileDescriptorLimit increases the file descriptor limit to the hard limit for better resource handling.
func IncreaseFileDescriptorLimit() {
	var rLimit syscall.Rlimit
	fmt.Println("🔍 Checking current file descriptor limit...")
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(fmt.Errorf("❌ Failed to get rlimit: %w", err))
	}
	fmt.Printf("📊 Current limit: Soft = %d, Hard = %d\n", rLimit.Cur, rLimit.Max)

	rLimit.Cur = rLimit.Max

	fmt.Printf("🔧 Increasing soft limit to match hard limit (%d)...\n", rLimit.Max)
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(fmt.Errorf("❌ Failed to set rlimit: %w", err))
	}
	fmt.Println("✅ File descriptor limit successfully increased.")
}
