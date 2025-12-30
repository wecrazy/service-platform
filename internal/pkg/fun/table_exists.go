package fun

import (
	"fmt"
	"service-platform/internal/config"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// TableExists checks if a table exists in the database
func TableExists(db *gorm.DB, tableName string) bool {
	dbType := config.GetConfig().Database.Type
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		var exists bool
		query := fmt.Sprintf("SELECT to_regclass('%s') IS NOT NULL", tableName)
		err := db.Raw(query).Scan(&exists).Error
		if err != nil {
			logrus.Errorf("TableExists: failed to check if table %s exists: %v", tableName, err)
			return false
		}
		return exists
	default:
		logrus.Warnf("TableExists: unsupported database type %s", dbType)
		return false
	}
}
