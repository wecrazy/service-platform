package fun

import (
	"fmt"
	"os"
)

func FindValidDirectory(paths []string) (string, error) {
	for _, dir := range paths {
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no valid report directory found in: %v", paths)
}
