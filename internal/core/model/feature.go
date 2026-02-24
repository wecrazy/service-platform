package model

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

// Feature represents a feature or permission in the system, which can be used for role-based access control (RBAC).
// Each feature can have a parent-child relationship to create a hierarchical structure of features, which can be used to organize permissions in the application.
// The Feature model includes fields for the title, path, menu order, status, level, and icon, which can be used to manage and display features in the application's user interface.
// The TableName method specifies the database table name for this model, which is defined in the configuration file under Database.TbFeature.
type Feature struct {
	ID uint `json:"id" gorm:"column:id;primary_key;autoincrement"`
	gorm.Model

	ParentID  uint   `json:"parent_id" gorm:"column:parent_id"`
	Title     string `json:"title" gorm:"column:title"`
	Path      string `json:"path" gorm:"column:path"`
	MenuOrder uint   `json:"menu_order" gorm:"column:menu_order"`
	Status    uint   `json:"status" gorm:"column:status"`
	Level     uint   `json:"level" gorm:"column:level"`
	Icon      string `json:"icon" gorm:"column:icon"`
}

// TableName specifies the database table name for the Feature model, which is defined in the configuration file under Database.TbFeature.
func (Feature) TableName() string {
	return config.ServicePlatform.Get().Database.TbFeature
}
