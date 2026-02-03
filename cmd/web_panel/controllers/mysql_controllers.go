package controllers

import (
	"fmt"
	"service-platform/cmd/web_panel/internal/gormdb"

	"github.com/sirupsen/logrus"
)

func RemoveLogMySQL(dayInt int) error {
	dbWeb := gormdb.Databases.Web

	// Get binary log information before purging
	type BinaryLogInfo struct {
		LogName   string `gorm:"column:Log_name"`
		FileSize  int64  `gorm:"column:File_size"`
		Encrypted string `gorm:"column:Encrypted"`
	}

	var allLogs []BinaryLogInfo
	if err := dbWeb.Raw("SHOW BINARY LOGS").Scan(&allLogs).Error; err != nil {
		logrus.Errorf("Failed to get binary logs list: %v", err)
		return fmt.Errorf("failed to get binary logs list: %v", err)
	}

	// Count and calculate size of logs before purging
	var totalSizeToDelete int64
	for _, log := range allLogs {
		totalSizeToDelete += log.FileSize
	}

	totalLogsCount := len(allLogs)
	totalSizeMB := float64(totalSizeToDelete) / 1024 / 1024
	totalSizeGB := totalSizeMB / 1024

	logrus.Infof("Current binary logs: %d files, total size: %.2f MB (%.2f GB)",
		totalLogsCount, totalSizeMB, totalSizeGB)

	// PURGE BINARY LOGS using interval - MySQL supports this syntax
	query := fmt.Sprintf("PURGE BINARY LOGS BEFORE NOW() - INTERVAL %d DAY", dayInt)

	logrus.Infof("Attempting to purge MySQL binary logs older than %d days", dayInt)

	result := dbWeb.Exec(query)
	if result.Error != nil {
		logrus.Errorf("Failed to purge MySQL binary logs: %v", result.Error)
		return result.Error
	}

	// Get remaining binary logs after purge
	var remainingLogs []BinaryLogInfo
	if err := dbWeb.Raw("SHOW BINARY LOGS").Scan(&remainingLogs).Error; err != nil {
		logrus.Warnf("Failed to get remaining binary logs: %v", err)
	} else {
		var remainingSizeTotal int64
		for _, log := range remainingLogs {
			remainingSizeTotal += log.FileSize
		}

		deletedCount := totalLogsCount - len(remainingLogs)
		deletedSize := totalSizeToDelete - remainingSizeTotal
		deletedSizeMB := float64(deletedSize) / 1024 / 1024
		deletedSizeGB := deletedSizeMB / 1024

		remainingSizeMB := float64(remainingSizeTotal) / 1024 / 1024
		remainingSizeGB := remainingSizeMB / 1024

		logrus.Infof("Successfully purged %d binary log file(s), freed %.2f MB (%.2f GB) of disk space",
			deletedCount, deletedSizeMB, deletedSizeGB)
		logrus.Infof("Remaining binary logs: %d files, total size: %.2f MB (%.2f GB)",
			len(remainingLogs), remainingSizeMB, remainingSizeGB)
	}

	return nil
}
