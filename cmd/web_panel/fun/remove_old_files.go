package fun

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"service-platform/cmd/web_panel/config"

	"github.com/sirupsen/logrus"
)

// RemoveOldFiles removes files within the specified directory (dirPath) that are older
// than the calculated threshold based on their modification time (not filename).
// The threshold is determined by subtracting the given dateRange (e.g., "-2day", "-1week", "-2month")
// from the current time in the configured timezone. Only files whose modification time
// is before the threshold will be removed.
//
// Parameters:
//   - dirPath:    The path to the directory containing files to clean up.
//   - dateRange:  A string specifying the age threshold (e.g., "-2day", "-1week", "-1month").
//
// Returns an error if reading the directory or removing files fails. Logs detailed debug information
// about the removal process.
//
// Example usage:
//
//	err := RemoveOldFiles("/path/to/web/file/uploaded_excel_to_odoo_ms", "-1week")
//	err := RemoveOldFiles("/path/to/web/file/uploaded_excel_to_odoo_ms", "-2days")
func RemoveOldFiles(dirPath, dateRange string) error {
	// Load timezone
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	threshold := now

	logrus.Debugf("🗂️ [RemoveOldFiles] Checking directory: %s", dirPath)

	// Normalize and parse dateRange
	dateRange = strings.TrimSpace(dateRange)
	negative := false
	if strings.HasPrefix(dateRange, "-") {
		negative = true
		dateRange = strings.TrimPrefix(dateRange, "-")
	}

	// Find number and unit robustly
	var num int
	var unit string
	for i, r := range dateRange {
		if r < '0' || r > '9' {
			numPart := dateRange[:i]
			unit = strings.ToLower(strings.TrimSpace(dateRange[i:]))
			fmt.Sscanf(numPart, "%d", &num)
			break
		}
	}

	logrus.Debugf("📊 [RemoveOldFiles] Parsed dateRange num: %d, unit: %s", num, unit)

	if num == 0 || unit == "" {
		logrus.Warnf("⚠️ [RemoveOldFiles] Invalid or missing dateRange unit/number, aborting file removal to prevent data loss.")
		return nil
	}

	if !negative {
		num = -num // always go to the past
	} else {
		num = -num
	}

	// Calculate threshold based on unit
	switch unit {
	case "day", "days", "d":
		threshold = now.AddDate(0, 0, num)
	case "week", "weeks", "w":
		threshold = now.AddDate(0, 0, num*7)
	case "month", "months", "m":
		threshold = now.AddDate(0, num, 0)
	case "year", "years", "y":
		threshold = now.AddDate(num, 0, 0)
	default:
		logrus.Warnf("⚠️ [RemoveOldFiles] Unrecognized dateRange unit '%s', aborting file removal to prevent data loss.", unit)
		return nil
	}

	logrus.Debugf("⏳ [RemoveOldFiles] Threshold date: %s (dateRange: %s)", threshold.Format(time.RFC3339), dateRange)

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		logrus.Warnf("⚠️ [RemoveOldFiles] Directory does not exist: %s", dirPath)
		return nil
	}

	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read dir: %v", err)
	}

	removedCount := 0
	skippedCount := 0

	for _, entry := range entries {
		// Skip directories, only process files
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		filePath := filepath.Join(dirPath, fileName)

		// Get file info to check modification time
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			logrus.Warnf("⚠️ [RemoveOldFiles] Failed to stat file: %s (error: %v)", filePath, err)
			continue
		}

		modTime := fileInfo.ModTime()

		logrus.Debugf("📄 [RemoveOldFiles] Checking file: %s (modified: %s, threshold: %s)",
			fileName, modTime.Format(time.RFC3339), threshold.Format(time.RFC3339))

		// Remove file if it's older than threshold
		if modTime.Before(threshold) {
			logrus.Debugf("🗑️ [RemoveOldFiles] Removing old file: %s (age: %s)",
				filePath, now.Sub(modTime).Round(time.Second))

			if err := os.Remove(filePath); err != nil {
				logrus.Errorf("❌ [RemoveOldFiles] Failed to remove file: %s (error: %v)", filePath, err)
				return fmt.Errorf("failed to remove %s: %v", filePath, err)
			}
			removedCount++
			logrus.Infof("✅ [RemoveOldFiles] Successfully removed: %s", fileName)
		} else {
			skippedCount++
			logrus.Debugf("⏭️ [RemoveOldFiles] Keeping file: %s (not old enough)", fileName)
		}
	}

	logrus.Infof("🎯 [RemoveOldFiles] Cleanup complete in %s - Removed: %d files, Kept: %d files",
		dirPath, removedCount, skippedCount)

	return nil
}

// RemoveOldFilesRecursive recursively removes files within the specified directory and its subdirectories
// that are older than the calculated threshold based on their modification time.
//
// Parameters:
//   - dirPath:    The path to the directory to scan recursively.
//   - dateRange:  A string specifying the age threshold (e.g., "-2day", "-1week", "-1month").
//
// Returns an error if walking the directory tree or removing files fails.
//
// Example usage:
//
//	err := RemoveOldFilesRecursive("/path/to/uploads", "-1week")
func RemoveOldFilesRecursive(dirPath, dateRange string) error {
	// Load timezone
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	threshold := now

	logrus.Debugf("🗂️ [RemoveOldFilesRecursive] Checking directory recursively: %s", dirPath)

	// Normalize and parse dateRange
	dateRange = strings.TrimSpace(dateRange)
	negative := false
	if strings.HasPrefix(dateRange, "-") {
		negative = true
		dateRange = strings.TrimPrefix(dateRange, "-")
	}

	// Find number and unit robustly
	var num int
	var unit string
	for i, r := range dateRange {
		if r < '0' || r > '9' {
			numPart := dateRange[:i]
			unit = strings.ToLower(strings.TrimSpace(dateRange[i:]))
			fmt.Sscanf(numPart, "%d", &num)
			break
		}
	}

	logrus.Debugf("📊 [RemoveOldFilesRecursive] Parsed dateRange num: %d, unit: %s", num, unit)

	if num == 0 || unit == "" {
		logrus.Warnf("⚠️ [RemoveOldFilesRecursive] Invalid or missing dateRange unit/number, aborting file removal to prevent data loss.")
		return nil
	}

	if !negative {
		num = -num // always go to the past
	} else {
		num = -num
	}

	// Calculate threshold based on unit
	switch unit {
	case "day", "days", "d":
		threshold = now.AddDate(0, 0, num)
	case "week", "weeks", "w":
		threshold = now.AddDate(0, 0, num*7)
	case "month", "months", "m":
		threshold = now.AddDate(0, num, 0)
	case "year", "years", "y":
		threshold = now.AddDate(num, 0, 0)
	default:
		logrus.Warnf("⚠️ [RemoveOldFilesRecursive] Unrecognized dateRange unit '%s', aborting file removal to prevent data loss.", unit)
		return nil
	}

	logrus.Debugf("⏳ [RemoveOldFilesRecursive] Threshold date: %s (dateRange: %s)", threshold.Format(time.RFC3339), dateRange)

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		logrus.Warnf("⚠️ [RemoveOldFilesRecursive] Directory does not exist: %s", dirPath)
		return nil
	}

	removedCount := 0
	skippedCount := 0

	// Walk through directory tree
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.Warnf("⚠️ [RemoveOldFilesRecursive] Error accessing path %s: %v", path, err)
			return nil // Continue walking
		}

		// Skip directories, only process files
		if info.IsDir() {
			return nil
		}

		modTime := info.ModTime()

		logrus.Debugf("📄 [RemoveOldFilesRecursive] Checking file: %s (modified: %s, threshold: %s)",
			path, modTime.Format(time.RFC3339), threshold.Format(time.RFC3339))

		// Remove file if it's older than threshold
		if modTime.Before(threshold) {
			logrus.Debugf("🗑️ [RemoveOldFilesRecursive] Removing old file: %s (age: %s)",
				path, now.Sub(modTime).Round(time.Second))

			if err := os.Remove(path); err != nil {
				logrus.Errorf("❌ [RemoveOldFilesRecursive] Failed to remove file: %s (error: %v)", path, err)
				return nil // Continue walking even if one file fails
			}
			removedCount++
			logrus.Infof("✅ [RemoveOldFilesRecursive] Successfully removed: %s", path)
		} else {
			skippedCount++
			logrus.Debugf("⏭️ [RemoveOldFilesRecursive] Keeping file: %s (not old enough)", info.Name())
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %v", err)
	}

	logrus.Infof("🎯 [RemoveOldFilesRecursive] Cleanup complete in %s - Removed: %d files, Kept: %d files",
		dirPath, removedCount, skippedCount)

	return nil
}
