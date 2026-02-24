package fun

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"service-platform/internal/config"
	"strings"
	"time"

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

// mysqlBinaryLog represents a MySQL binary log entry returned by SHOW BINARY LOGS.
type mysqlBinaryLog struct {
	LogName  string `gorm:"column:Log_name"`
	FileSize int64  `gorm:"column:File_size"`
}

// getMySQLLogDirectory resolves the MySQL binary log directory.
// It starts with an OS default then overrides with the value from MySQL's datadir variable.
func getMySQLLogDirectory(db *gorm.DB, osType string) string {
	var logDir string
	switch strings.ToLower(osType) {
	case "linux":
		logDir = "/var/lib/mysql"
	case "windows":
		logDir = "C:\\ProgramData\\MySQL\\MySQL Server 8.0\\Data"
	case "darwin":
		logDir = "/usr/local/mysql/data"
	default:
		logrus.Warnf("Unknown OS type: %s, attempting default purge", osType)
	}

	var result struct{ Value string }
	if err := db.Raw("SHOW VARIABLES LIKE 'datadir'").Scan(&result).Error; err == nil && result.Value != "" {
		logDir = result.Value
	}
	return logDir
}

// analyzeMySQLBinaryLogStats returns the number and sizes of binary logs eligible for deletion.
func analyzeMySQLBinaryLogStats(logDir string, logs []mysqlBinaryLog, cutoffTime time.Time) (logsToDelete int, totalSize, sizeToDelete int64) {
	for i, log := range logs {
		logPath := filepath.Join(logDir, log.LogName)
		fileInfo, err := os.Stat(logPath)
		if err != nil {
			logrus.Warnf("Cannot access binary log file %s: %v", logPath, err)
			totalSize += log.FileSize // fallback to reported size
			continue
		}
		totalSize += fileInfo.Size()
		if i == len(logs)-1 { // skip currently active log
			logrus.Infof("Keeping active binary log: %s (size: %s)", log.LogName, formatBytes(fileInfo.Size()))
			continue
		}
		if fileInfo.ModTime().Before(cutoffTime) {
			logsToDelete++
			sizeToDelete += fileInfo.Size()
		}
	}
	return
}

// purgeMySQLBinaryLogs purges MySQL binary logs
func purgeMySQLBinaryLogs(db *gorm.DB, olderThan string, osType string) {
	cutoffTime, err := calculateCutoffTime(olderThan)
	if err != nil {
		logrus.Errorf("Failed to parse duration '%s': %v", olderThan, err)
		return
	}
	logrus.Infof("Purging MySQL binary logs older than %s (cutoff: %s)", olderThan, cutoffTime.Format(config.DateYYYYMMDDHHMMSS))

	var result struct{ Value string }
	if err = db.Raw("SHOW VARIABLES LIKE 'log_bin'").Scan(&result).Error; err != nil {
		logrus.Errorf("Failed to check binary log status: %v", err)
		return
	}
	if strings.ToLower(result.Value) != "on" {
		logrus.Info("MySQL binary logging is not enabled, skipping purge")
		return
	}

	var logs []mysqlBinaryLog
	if err = db.Raw("SHOW BINARY LOGS").Scan(&logs).Error; err != nil {
		logrus.Errorf("Failed to get binary logs list: %v", err)
		return
	}
	if len(logs) == 0 {
		logrus.Info("No binary logs found")
		return
	}

	logDir := getMySQLLogDirectory(db, osType)
	logsToDelete, totalSizeBefore, sizeToDelete := analyzeMySQLBinaryLogStats(logDir, logs, cutoffTime)

	if logsToDelete == 0 {
		logrus.Infof("No old MySQL binary logs to purge. Total size: %s", formatBytes(totalSizeBefore))
		return
	}

	sizeRemaining := totalSizeBefore - sizeToDelete
	logrus.Infof("📊 MySQL Binary Logs - Before deletion: %s", formatBytes(totalSizeBefore))
	logrus.Infof("🗑️  Will delete: %s (%d logs older than %s)", formatBytes(sizeToDelete), logsToDelete, cutoffTime.Format(config.DateYYYYMMDD))
	logrus.Infof("💾 Will remain: %s", formatBytes(sizeRemaining))

	purgeSQL := fmt.Sprintf("PURGE BINARY LOGS BEFORE '%s'", cutoffTime.Format(config.DateYYYYMMDDHHMMSS))
	if err = db.Exec(purgeSQL).Error; err != nil {
		logrus.Errorf("Failed to purge MySQL binary logs: %v", err)
		return
	}

	logrus.Infof("✅ Successfully deleted %d MySQL binary logs", logsToDelete)
	logrus.Infof("📉 Freed space: %s", formatBytes(sizeToDelete))
	logrus.Infof("💿 Remaining size: %s", formatBytes(sizeRemaining))
}

// findWALDirectory returns the pg_wal (or legacy pg_xlog) path under dataDir.
func findWALDirectory(dataDir string) (string, error) {
	if walDir := filepath.Join(dataDir, "pg_wal"); dirExists(walDir) {
		return walDir, nil
	}
	if walDir := filepath.Join(dataDir, "pg_xlog"); dirExists(walDir) {
		return walDir, nil
	}
	return "", fmt.Errorf("WAL directory not found (tried pg_wal and pg_xlog under %s)", dataDir)
}

func dirExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// collectWALFilesToDelete walks walDir and collects WAL files older than cutoffTime.
// It skips the currentWAL file and any non-WAL entries.
func collectWALFilesToDelete(walDir, currentWAL string, cutoffTime time.Time) (filesToDelete map[string]int64, totalSize, sizeToDelete int64, err error) {
	filesToDelete = make(map[string]int64)
	err = filepath.Walk(walDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// WAL files are non-directories with exactly 24-char hex names
		if info.IsDir() || len(info.Name()) != 24 {
			return nil
		}
		if info.Name() == currentWAL || info.Name() == "archive_status" {
			return nil
		}
		totalSize += info.Size()
		if info.ModTime().Before(cutoffTime) {
			filesToDelete[path] = info.Size()
			sizeToDelete += info.Size()
		}
		return nil
	})
	return
}

// purgePostgreSQLWALLogs purges PostgreSQL WAL (Write-Ahead Log) files
func purgePostgreSQLWALLogs(db *gorm.DB, olderThan string, osType string) {
	cutoffTime, err := calculateCutoffTime(olderThan)
	if err != nil {
		logrus.Errorf("Failed to parse duration '%s': %v", olderThan, err)
		return
	}
	logrus.Infof("Purging PostgreSQL WAL logs older than %s (cutoff: %s)", olderThan, cutoffTime.Format(config.DateYYYYMMDDHHMMSS))

	dataDir := getPostgreSQLDataDir(db, osType)
	if dataDir == "" {
		logrus.Error("❌ Cannot determine PostgreSQL data directory. Please check:")
		logrus.Error("   1. Grant permission: GRANT pg_read_all_settings TO your_user;")
		logrus.Error("   2. Or run: SELECT setting FROM pg_settings WHERE name='data_directory';")
		logrus.Error("   3. Or check your PostgreSQL installation directory")
		return
	}
	logrus.Infof("Using PostgreSQL data directory: %s", dataDir)

	walDir, err := findWALDirectory(dataDir)
	if err != nil {
		logrus.Errorf("❌ %v", err)
		return
	}
	logrus.Infof("Found PostgreSQL WAL directory: %s", walDir)

	var currentWAL string
	if err = db.Raw("SELECT pg_walfile_name(pg_current_wal_lsn())").Scan(&currentWAL).Error; err != nil {
		logrus.Errorf("Failed to get current WAL file: %v", err)
		logrus.Warn("⚠️ Skipping WAL purge to avoid data corruption")
		return
	}
	logrus.Infof("Current WAL file: %s (will not be deleted)", currentWAL)

	filesToDelete, totalSizeBefore, sizeDeleted, err := collectWALFilesToDelete(walDir, currentWAL, cutoffTime)
	if err != nil {
		logrus.Errorf("Error walking through WAL directory: %v", err)
		return
	}
	if len(filesToDelete) == 0 {
		logrus.Infof("No old PostgreSQL WAL files to purge. Total size: %s", formatBytes(totalSizeBefore))
		return
	}

	logrus.Infof("📊 PostgreSQL WAL Files - Before deletion: %s", formatBytes(totalSizeBefore))
	logrus.Infof("🗑️  Will delete: %s (%d files older than %s)", formatBytes(sizeDeleted), len(filesToDelete), cutoffTime.Format(config.DateYYYYMMDD))
	logrus.Infof("💾 Will remain: %s", formatBytes(totalSizeBefore-sizeDeleted))

	deletedCount, actualDeleted := deleteFiles(filesToDelete)
	if deletedCount > 0 {
		logrus.Infof("✅ Successfully deleted %d PostgreSQL WAL file(s)", deletedCount)
		logrus.Infof("📉 Freed space: %s", formatBytes(actualDeleted))
		logrus.Infof("💿 Remaining size: %s", formatBytes(totalSizeBefore-actualDeleted))
	} else {
		logrus.Warn("Purge completed but no files were successfully deleted")
	}
}

// deleteFiles removes all files in the map and returns the count and total size deleted.
func deleteFiles(files map[string]int64) (count int, totalDeleted int64) {
	for path, size := range files {
		if err := os.Remove(path); err != nil {
			logrus.Errorf("Failed to delete file %s: %v", filepath.Base(path), err)
			continue
		}
		logrus.Debugf("Deleted file: %s (size: %s)", filepath.Base(path), formatBytes(size))
		count++
		totalDeleted += size
	}
	return
}

// getCommonPostgreSQLPaths returns well-known PostgreSQL data directory paths for the given OS.
func getCommonPostgreSQLPaths(osType string) []string {
	switch strings.ToLower(osType) {
	case "linux":
		return []string{
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
	case "windows":
		return []string{
			"C:\\Program Files\\PostgreSQL\\16\\data",
			"C:\\Program Files\\PostgreSQL\\15\\data",
			"C:\\Program Files\\PostgreSQL\\14\\data",
			"C:\\PostgreSQL\\data",
		}
	case "darwin":
		return []string{
			"/usr/local/var/postgres",
			"/opt/homebrew/var/postgres",
			"/Library/PostgreSQL/16/data",
			"/Library/PostgreSQL/15/data",
		}
	}
	return nil
}

// getPostgreSQLDataDir tries multiple methods to find the PostgreSQL data directory.
func getPostgreSQLDataDir(db *gorm.DB, osType string) string {
	var dataDir string

	// Method 1: SHOW data_directory (requires superuser or pg_read_all_settings)
	if err := db.Raw("SHOW data_directory").Scan(&dataDir).Error; err == nil && dataDir != "" {
		if dirExists(dataDir) {
			return dataDir
		}
		logrus.Warnf("data_directory returned '%s' but directory doesn't exist", dataDir)
	}

	// Method 2: pg_settings (alternative for restricted users)
	if err := db.Raw("SELECT setting FROM pg_settings WHERE name='data_directory'").Scan(&dataDir).Error; err == nil && dataDir != "" {
		if dirExists(dataDir) {
			logrus.Info("Found data directory via pg_settings")
			return dataDir
		}
	}

	// Method 3: well-known OS-specific paths
	for _, p := range getCommonPostgreSQLPaths(osType) {
		if dirExists(p) {
			logrus.Infof("Found data directory at: %s", p)
			return p
		}
	}

	return ""
}
