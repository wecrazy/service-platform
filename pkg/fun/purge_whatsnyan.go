package fun

import (
	"service-platform/internal/config"
	whatsnyanmodel "service-platform/internal/core/model/whatsnyan_model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// PurgeOldWhatsnyanMessages permanently deletes messages (both soft-deleted and active) older than specified duration based on created_at
func PurgeOldWhatsnyanMessages(db *gorm.DB, olderThan string) {
	// Calculate cutoff time
	cutoffTime, err := calculateCutoffTime(olderThan)
	if err != nil {
		logrus.Errorf("Failed to parse duration '%s': %v", olderThan, err)
		return
	}

	logrus.Infof("Purging Whatsnyan messages older than %s (cutoff: %s)", olderThan, cutoffTime.Format(config.DateYYYYMMDDHHMMSS))

	// Get table name
	tableName := whatsnyanmodel.WhatsAppMsg{}.TableName()

	// Count total records (including soft-deleted)
	var totalCount int64
	db.Unscoped().Model(&whatsnyanmodel.WhatsAppMsg{}).Count(&totalCount)

	// Count active records
	var activeCount int64
	db.Model(&whatsnyanmodel.WhatsAppMsg{}).Count(&activeCount)

	// Count soft-deleted records
	var softDeletedCount int64
	db.Unscoped().Model(&whatsnyanmodel.WhatsAppMsg{}).
		Where("deleted_at IS NOT NULL").
		Count(&softDeletedCount)

	// Count records older than cutoff (both active and soft-deleted)
	var toDeleteCount int64
	db.Unscoped().Model(&whatsnyanmodel.WhatsAppMsg{}).
		Where("created_at < ?", cutoffTime).
		Count(&toDeleteCount)

	if toDeleteCount == 0 {
		logrus.Infof("No old Whatsnyan messages to purge")
		logrus.Infof("📊 Total records: %d (Active: %d | Soft-deleted: %d)", totalCount, activeCount, softDeletedCount)
		return
	}

	remainingRecords := totalCount - toDeleteCount

	logrus.Infof("📊 Whatsnyan Messages (%s)", tableName)
	logrus.Infof("   Total records: %d (Active: %d | Soft-deleted: %d)", totalCount, activeCount, softDeletedCount)
	logrus.Infof("🗑️  Will permanently delete: %d messages (created before %s)", toDeleteCount, cutoffTime.Format(config.DateYYYYMMDD))
	logrus.Infof("💾 Will remain: %d records", remainingRecords)

	// Permanently delete old records based on created_at using Unscoped()
	result := db.Unscoped().
		Where("created_at < ?", cutoffTime).
		Delete(&whatsnyanmodel.WhatsAppMsg{})

	if result.Error != nil {
		logrus.Errorf("Failed to purge old Whatsnyan messages: %v", result.Error)
		return
	}

	actualDeleted := result.RowsAffected

	if actualDeleted > 0 {
		logrus.Infof("✅ Successfully purged %d old Whatsnyan messages", actualDeleted)
		logrus.Infof("📉 Freed %d records from table %s", actualDeleted, tableName)

		// Count remaining records after deletion
		var remainingAfter int64
		db.Unscoped().Model(&whatsnyanmodel.WhatsAppMsg{}).Count(&remainingAfter)

		var activeAfter int64
		db.Model(&whatsnyanmodel.WhatsAppMsg{}).Count(&activeAfter)

		var softDeletedAfter int64
		db.Unscoped().Model(&whatsnyanmodel.WhatsAppMsg{}).
			Where("deleted_at IS NOT NULL").
			Count(&softDeletedAfter)

		logrus.Infof("💿 Remaining records: %d (Active: %d | Soft-deleted: %d)", remainingAfter, activeAfter, softDeletedAfter)
	} else {
		logrus.Warn("Purge completed but no records were deleted")
	}
}
