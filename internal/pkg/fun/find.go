package fun

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
)

func FindValidDirectory(paths []string) (string, error) {
	for _, dir := range paths {
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no valid directory found in: %v", paths)
}

func FindValidFile(paths []string) (string, error) {
	for _, filePath := range paths {
		if _, err := os.Stat(filePath); err == nil {
			return filePath, nil
		}
	}
	return "", fmt.Errorf("no valid file found in: %v", paths)
}

// GetLogDir resolves the absolute path to the log directory.
// It tries to find the directory relative to the current working directory,
// or falls back to finding the project root (via go.mod) and appending the configLogDir.
func GetLogDir(configLogDir string) (string, error) {
	// 1. Try to find existing log dir in standard locations
	candidates := []string{
		configLogDir,
		filepath.Join("..", configLogDir),
		filepath.Join("..", "..", configLogDir),
	}

	if dir, err := FindValidDirectory(candidates); err == nil {
		return filepath.Abs(dir)
	}

	// 2. Try to find project root by looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Found project root
			logDir := filepath.Join(dir, configLogDir)
			return logDir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// 3. Fallback to just using the config value (will likely create in CWD)
	return filepath.Abs(configLogDir)
}

// GetFileExtension returns the file extension associated with the provided MIME type.
// If the MIME type is recognized, it returns the first corresponding extension (e.g., ".mp4").
// If the MIME type is unknown or an error occurs, it returns a safe default extension ".bin".
func GetFileExtension(mimeType string) string {
	exts, err := mime.ExtensionsByType(mimeType)
	if err == nil && len(exts) > 0 {
		return exts[0] // e.g., ".mp4"
	}
	// fallback to safe default if unknown
	return ".bin"
}
