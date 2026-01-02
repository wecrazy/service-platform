package fun

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"service-platform/internal/config"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// PurgeOldLogBackupFiles removes old .gz backup files from the log directory
// that are older than the specified duration (e.g., "3Days", "1Week", "1Month")
func PurgeOldLogBackupFiles(olderThan string) {
	cfg := config.GetConfig()
	logDir := cfg.App.LogDir

	// Validate log directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		logDir2, errFind := FindValidDirectory([]string{
			"log",
			"../log",
			"../../log",
			"../../../log",
			"../../../../log",
		})
		if errFind != nil {
			logrus.Errorf("Log directory not found: %v", errFind)
			return
		}
		logDir = logDir2
	}

	// Check if there's at least one .log file in the directory
	hasLogFile, err := hasLogFileInDir(logDir)
	if err != nil {
		logrus.Errorf("Failed to check for .log files in %s: %v", logDir, err)
		return
	}
	if !hasLogFile {
		logrus.Warnf("No .log files found in %s, skipping purge", logDir)
		return
	}

	// Calculate the cutoff time from olderThan parameter
	cutoffTime, err := calculateCutoffTime(olderThan)
	if err != nil {
		logrus.Errorf("Failed to parse duration '%s': %v", olderThan, err)
		return
	}

	logrus.Infof("Purging .gz backup files older than %s (cutoff: %s)", olderThan, cutoffTime.Format(config.DATE_YYYY_MM_DD_HH_MM_SS))

	// Search for .gz files in log directory
	deletedCount := 0
	err = filepath.Walk(logDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file has .gz extension
		if !strings.HasSuffix(info.Name(), ".gz") {
			return nil
		}

		// Check if file is older than cutoff time
		if info.ModTime().Before(cutoffTime) {
			err := os.Remove(path)
			if err != nil {
				logrus.Errorf("Failed to delete %s: %v", path, err)
			} else {
				logrus.Infof("Deleted old backup file: %s (modified: %s)", path, info.ModTime().Format(config.DATE_YYYY_MM_DD_HH_MM_SS))
				deletedCount++
			}
		}

		return nil
	})

	if err != nil {
		logrus.Errorf("Error walking through log directory: %v", err)
		return
	}

	if deletedCount > 0 {
		logrus.Infof("Purge completed: deleted %d old .gz backup file(s)", deletedCount)
	} else {
		logrus.Info("Purge completed: no old .gz backup files found to delete")
	}
}

// hasLogFileInDir checks if there's at least one .log file in the directory
func hasLogFileInDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			return true, nil
		}
	}

	return false, nil
}

// calculateCutoffTime calculates the cutoff time based on the duration string
// Keeps files from the current period and deletes older files
// Examples: "1d" keeps today, "1w" keeps current week, "1m" keeps current month
func calculateCutoffTime(input string) (time.Time, error) {
	// Convert input to lowercase for easier matching
	input = strings.ToLower(input)

	// Use regex to extract number and unit (supports both long and short formats)
	re := regexp.MustCompile(`^(\d+)(d|day|days|w|week|weeks|m|month|months)$`)
	matches := re.FindStringSubmatch(input)

	if len(matches) != 3 {
		return time.Time{}, fmt.Errorf("invalid duration format: %s (expected format: <number><d|w|m> or <number><Day|Week|Month>, e.g., '3d', '1w', '1m' or '3Days', '1Week', '1Month')", input)
	}

	count, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid number in duration: %s", matches[1])
	}

	unit := strings.TrimSuffix(matches[2], "s") // Normalize to singular form

	now := time.Now()
	var cutoffTime time.Time

	switch unit {
	case "d", "day":
		// Keep last N days: cutoff at start of N days ago
		cutoffTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -count+1)
	case "w", "week":
		// Keep last N weeks: cutoff at start of N weeks ago
		startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		daysFromMonday := int(now.Weekday()) - int(time.Monday)
		if daysFromMonday < 0 {
			daysFromMonday += 7
		}
		startOfWeek := startOfToday.AddDate(0, 0, -daysFromMonday)
		cutoffTime = startOfWeek.AddDate(0, 0, -7*(count-1))
	case "m", "month":
		// Keep last N months: cutoff at start of N months ago
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		cutoffTime = startOfMonth.AddDate(0, -count+1, 0)
	default:
		return time.Time{}, fmt.Errorf("unsupported time unit: %s (supported: d/Day, w/Week, m/Month)", matches[2])
	}

	return cutoffTime, nil
}
