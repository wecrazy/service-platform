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

// RemoveExistingDir removes subdirectories within the specified directory (dirPath) whose names,
// when parsed as dates using dirDateFormat, are older than a calculated threshold date.
// The threshold date is determined by subtracting the given dateRange (e.g., "-2day", "-4month", "-1year", "-1week")
// from the current time in the "timezone". Only directories whose parsed date is before the threshold
// will be removed, along with all their contents.
//
// Parameters:
//   - dirPath:        The path to the parent directory containing date-named subdirectories.
//   - dateRange:      A string specifying the age threshold (e.g., "-2day", "-1month").
//   - dirDateFormat:  The Go time format string used to parse subdirectory names as dates.
//
// Returns an error if reading the directory or removing a subdirectory fails. Logs detailed debug information
// about the removal process and skips directories whose names cannot be parsed as dates.
//
// Example usage:
//   err := RemoveExistingDir("/data/backups", "-7days", "2006-01-02")

func RemoveExistingDirectory(dirPath, dateRange, dirDateFormat string) error {
	// Support flexible dateRange: -2day, -4month, -1year, -1week, etc.
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	threshold := now

	logrus.Debugf("🗂️ [RemoveExistingDir] Checking directory: %s", dirPath)

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
	logrus.Debugf("[RemoveExistingDir] Parsed dateRange num: %d, unit: %s", num, unit)
	if num == 0 || unit == "" {
		logrus.Warnf("[RemoveExistingDir] Invalid or missing dateRange unit/number, aborting directory removal to prevent data loss.")
		return nil
	}
	if !negative {
		num = -num // always go to the past
	} else {
		num = -num
	}
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
		logrus.Warnf("[RemoveExistingDir] Unrecognized dateRange unit '%s', aborting directory removal to prevent data loss.", unit)
		return nil
	}

	logrus.Debugf("⏳ [RemoveExistingDir] Threshold date: %s (dateRange: %s)", threshold.Format(time.RFC3339), dateRange)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		dirDate, err := time.Parse(dirDateFormat, dirName)
		if err != nil {
			logrus.Debugf("❓ [RemoveExistingDir] Skipping dir: %s (cannot parse as date: %v)", dirName, err)
			continue
		}
		logrus.Debugf("📁 [RemoveExistingDir] Checking dir: %s (parsed date: %s, threshold: %s)", dirName, dirDate.Format(time.RFC3339), threshold.Format(time.RFC3339))
		if dirDate.Before(threshold) {
			removePath := filepath.Join(dirPath, dirName)
			logrus.Debugf("🗑️ [RemoveExistingDir] Removing directory: %s", removePath)

			// List files before removing
			fileList := []string{}
			filepath.Walk(removePath, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					fileList = append(fileList, path)
				}
				return nil
			})
			for _, f := range fileList {
				logrus.Debugf("🗃️ [RemoveExistingDir] Will remove file: %s", f)
			}

			if err := os.RemoveAll(removePath); err != nil {
				return fmt.Errorf("failed to remove %s: %v", removePath, err)
			}
		}
	}
	return nil
}
