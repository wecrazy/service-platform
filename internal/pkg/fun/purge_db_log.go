package fun

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// PurgeOldDatabaseLogs removes old database logs (MySQL binary logs or PostgreSQL WAL files)
// based on the database type and operating system
func PurgeOldDatabaseLogs(db *gorm.DB, olderThan string, dbType string) {
	osType := runtime.GOOS
	logrus.Infof("Purging old database logs for %s on %s", dbType, osType)

	switch strings.ToLower(dbType) {
	case "mysql":
		purgeMySQLBinaryLogs(db, olderThan, osType)
	case "postgresql", "postgres":
		purgePostgreSQLWALLogs(db, olderThan, osType)
	default:
		logrus.Warnf("Unsupported database type for log purging: %s", dbType)
	}
}

// purgeMySQLBinaryLogs purges MySQL binary logs
func purgeMySQLBinaryLogs(db *gorm.DB, olderThan string, osType string) {
	// Calculate cutoff time
	cutoffTime, err := calculateCutoffTime(olderThan)
	if err != nil {
		logrus.Errorf("Failed to parse duration '%s': %v", olderThan, err)
		return
	}

	logrus.Infof("Purging MySQL binary logs older than %s (cutoff: %s)", olderThan, cutoffTime.Format("2006-01-02 15:04:05"))

	// Check if binary logging is enabled
	var result struct {
		Value string
	}
	err = db.Raw("SHOW VARIABLES LIKE 'log_bin'").Scan(&result).Error
	if err != nil {
		logrus.Errorf("Failed to check binary log status: %v", err)
		return
	}

	if strings.ToLower(result.Value) != "on" {
		logrus.Info("MySQL binary logging is not enabled, skipping purge")
		return
	}

	// Get the list of binary logs
	var logs []struct {
		LogName  string `gorm:"column:Log_name"`
		FileSize int64  `gorm:"column:File_size"`
	}
	err = db.Raw("SHOW BINARY LOGS").Scan(&logs).Error
	if err != nil {
		logrus.Errorf("Failed to get binary logs list: %v", err)
		return
	}

	if len(logs) == 0 {
		logrus.Info("No binary logs found")
		return
	}

	// Get binary log directory based on OS
	var logDir string
	switch strings.ToLower(osType) {
	case "linux":
		logDir = "/var/lib/mysql" // Default for Linux
	case "windows":
		logDir = "C:\\ProgramData\\MySQL\\MySQL Server 8.0\\Data" // Default for Windows
	case "darwin":
		logDir = "/usr/local/mysql/data" // Default for macOS
	default:
		logrus.Warnf("Unknown OS type: %s, attempting default purge", osType)
	}

	// Try to get actual log directory from MySQL
	var logDirResult struct {
		Value string
	}
	err = db.Raw("SHOW VARIABLES LIKE 'datadir'").Scan(&logDirResult).Error
	if err == nil && logDirResult.Value != "" {
		logDir = logDirResult.Value
	}

	// Calculate sizes and count logs to purge
	logsToDelete := 0
	var totalSizeBefore int64 = 0
	var sizeToDelete int64 = 0

	for i, log := range logs {
		logPath := filepath.Join(logDir, log.LogName)
		fileInfo, err := os.Stat(logPath)
		if err != nil {
			logrus.Warnf("Cannot access binary log file %s: %v", logPath, err)
			// Use size from SHOW BINARY LOGS as fallback
			totalSizeBefore += log.FileSize
			continue
		}

		totalSizeBefore += fileInfo.Size()

		// Skip the last log (currently active)
		if i == len(logs)-1 {
			logrus.Infof("Keeping active binary log: %s (size: %s)", log.LogName, formatBytes(fileInfo.Size()))
			continue
		}

		if fileInfo.ModTime().Before(cutoffTime) {
			logsToDelete++
			sizeToDelete += fileInfo.Size()
		}
	}

	if logsToDelete == 0 {
		logrus.Infof("No old MySQL binary logs to purge. Total size: %s", formatBytes(totalSizeBefore))
		return
	}

	sizeRemaining := totalSizeBefore - sizeToDelete
	logrus.Infof("📊 MySQL Binary Logs - Before deletion: %s", formatBytes(totalSizeBefore))
	logrus.Infof("🗑️  Will delete: %s (%d logs older than %s)", formatBytes(sizeToDelete), logsToDelete, cutoffTime.Format("2006-01-02"))
	logrus.Infof("💾 Will remain: %s", formatBytes(sizeRemaining))

	// Use MySQL PURGE BINARY LOGS command (safer than deleting files directly)
	purgeSQL := fmt.Sprintf("PURGE BINARY LOGS BEFORE '%s'", cutoffTime.Format("2006-01-02 15:04:05"))
	err = db.Exec(purgeSQL).Error
	if err != nil {
		logrus.Errorf("Failed to purge MySQL binary logs: %v", err)
		return
	}

	logrus.Infof("✅ Successfully deleted %d MySQL binary logs", logsToDelete)
	logrus.Infof("📉 Freed space: %s", formatBytes(sizeToDelete))
	logrus.Infof("💿 Remaining size: %s", formatBytes(sizeRemaining))
}

// purgePostgreSQLWALLogs purges PostgreSQL WAL (Write-Ahead Log) files
func purgePostgreSQLWALLogs(db *gorm.DB, olderThan string, osType string) {
	// Calculate cutoff time
	cutoffTime, err := calculateCutoffTime(olderThan)
	if err != nil {
		logrus.Errorf("Failed to parse duration '%s': %v", olderThan, err)
		return
	}

	logrus.Infof("Purging PostgreSQL WAL logs older than %s (cutoff: %s)", olderThan, cutoffTime.Format("2006-01-02 15:04:05"))

	// Try to get PostgreSQL data directory using multiple methods
	dataDir := getPostgreSQLDataDir(db, osType)
	if dataDir == "" {
		logrus.Error("❌ Cannot determine PostgreSQL data directory. Please check:")
		logrus.Error("   1. Grant permission: GRANT pg_read_all_settings TO your_user;")
		logrus.Error("   2. Or run: SELECT setting FROM pg_settings WHERE name='data_directory';")
		logrus.Error("   3. Or check your PostgreSQL installation directory")
		return
	}

	logrus.Infof("Using PostgreSQL data directory: %s", dataDir)

	// PostgreSQL WAL files are in pg_wal subdirectory (or pg_xlog in older versions)
	walDir := filepath.Join(dataDir, "pg_wal")
	if _, err := os.Stat(walDir); os.IsNotExist(err) {
		// Try older directory name
		walDir = filepath.Join(dataDir, "pg_xlog")
		if _, err := os.Stat(walDir); os.IsNotExist(err) {
			logrus.Errorf("❌ PostgreSQL WAL directory not found. Tried:")
			logrus.Errorf("   - %s", filepath.Join(dataDir, "pg_wal"))
			logrus.Errorf("   - %s", filepath.Join(dataDir, "pg_xlog"))
			return
		}
	}

	logrus.Infof("Found PostgreSQL WAL directory: %s", walDir)

	// Get current WAL file to avoid deleting it
	var currentWAL string
	err = db.Raw("SELECT pg_walfile_name(pg_current_wal_lsn())").Scan(&currentWAL).Error
	if err != nil {
		logrus.Errorf("Failed to get current WAL file: %v", err)
		logrus.Warn("⚠️ Skipping WAL purge to avoid data corruption")
		return
	}

	logrus.Infof("Current WAL file: %s (will not be deleted)", currentWAL)

	// Count and calculate sizes
	deletedCount := 0
	var totalSizeBefore int64 = 0
	var sizeDeleted int64 = 0

	// First pass: calculate total size and identify files to delete
	filesToDelete := make(map[string]int64)
	err = filepath.Walk(walDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// PostgreSQL WAL files are typically 16MB and have specific naming (24 hex characters)
		// Example: 000000010000000000000001
		if len(info.Name()) != 24 {
			return nil
		}

		// Skip current WAL file and archive_status directory
		if info.Name() == currentWAL || info.Name() == "archive_status" {
			return nil
		}

		totalSizeBefore += info.Size()

		// Check if file is older than cutoff time
		if info.ModTime().Before(cutoffTime) {
			filesToDelete[path] = info.Size()
			sizeDeleted += info.Size()
		}

		return nil
	})

	if err != nil {
		logrus.Errorf("Error walking through WAL directory: %v", err)
		return
	}

	if len(filesToDelete) == 0 {
		logrus.Infof("No old PostgreSQL WAL files to purge. Total size: %s", formatBytes(totalSizeBefore))
		return
	}

	sizeRemaining := totalSizeBefore - sizeDeleted
	logrus.Infof("📊 PostgreSQL WAL Files - Before deletion: %s", formatBytes(totalSizeBefore))
	logrus.Infof("🗑️  Will delete: %s (%d files older than %s)", formatBytes(sizeDeleted), len(filesToDelete), cutoffTime.Format("2006-01-02"))
	logrus.Infof("💾 Will remain: %s", formatBytes(sizeRemaining))

	// Second pass: delete the files
	actualDeleted := int64(0)
	for path, size := range filesToDelete {
		err := os.Remove(path)
		if err != nil {
			logrus.Errorf("Failed to delete WAL file %s: %v", filepath.Base(path), err)
		} else {
			logrus.Debugf("Deleted old WAL file: %s (size: %s)", filepath.Base(path), formatBytes(size))
			deletedCount++
			actualDeleted += size
		}
	}

	if deletedCount > 0 {
		logrus.Infof("✅ Successfully deleted %d PostgreSQL WAL file(s)", deletedCount)
		logrus.Infof("📉 Freed space: %s", formatBytes(actualDeleted))
		logrus.Infof("💿 Remaining size: %s", formatBytes(totalSizeBefore-actualDeleted))
	} else {
		logrus.Warn("Purge completed but no files were successfully deleted")
	}
}

// getPostgreSQLDataDir tries multiple methods to find PostgreSQL data directory
func getPostgreSQLDataDir(db *gorm.DB, osType string) string {
	// Method 1: Try SHOW data_directory (requires permissions)
	var dataDir string
	err := db.Raw("SHOW data_directory").Scan(&dataDir).Error
	if err == nil && dataDir != "" {
		// Verify directory exists
		if _, err := os.Stat(dataDir); err == nil {
			return dataDir
		}
		logrus.Warnf("data_directory returned '%s' but directory doesn't exist", dataDir)
	}

	// Method 2: Try pg_settings (alternative method)
	err = db.Raw("SELECT setting FROM pg_settings WHERE name='data_directory'").Scan(&dataDir).Error
	if err == nil && dataDir != "" {
		if _, err := os.Stat(dataDir); err == nil {
			logrus.Info("Found data directory via pg_settings")
			return dataDir
		}
	}

	// Method 3: Try common Linux paths
	if strings.ToLower(osType) == "linux" {
		commonPaths := []string{
			"/var/lib/postgresql/data",
			"/var/lib/pgsql/data",
			"/usr/local/pgsql/data",
			"/opt/postgresql/data",
			"/var/lib/postgresql/16/main", // Debian/Ubuntu style
			"/var/lib/postgresql/15/main",
			"/var/lib/postgresql/14/main",
			"/var/lib/postgresql/13/main",
			"/var/lib/pgsql/16/data", // RedHat/CentOS style
			"/var/lib/pgsql/15/data",
			"/var/lib/pgsql/14/data",
		}

		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				logrus.Infof("Found data directory at: %s", path)
				return path
			}
		}
	}

	// Method 4: Try Windows paths
	if strings.ToLower(osType) == "windows" {
		commonPaths := []string{
			"C:\\Program Files\\PostgreSQL\\16\\data",
			"C:\\Program Files\\PostgreSQL\\15\\data",
			"C:\\Program Files\\PostgreSQL\\14\\data",
			"C:\\PostgreSQL\\data",
		}

		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				logrus.Infof("Found data directory at: %s", path)
				return path
			}
		}
	}

	// Method 5: Try macOS paths
	if strings.ToLower(osType) == "darwin" {
		commonPaths := []string{
			"/usr/local/var/postgres",
			"/opt/homebrew/var/postgres",
			"/Library/PostgreSQL/16/data",
			"/Library/PostgreSQL/15/data",
		}

		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				logrus.Infof("Found data directory at: %s", path)
				return path
			}
		}
	}

	return ""
}
