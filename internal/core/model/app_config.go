package model

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type AppConfig struct {
	gorm.Model
	RoleID      uint   `json:"role_id" gorm:"column:role_id;not null"`
	AppName     string `json:"app_name" gorm:"column:app_name;size:100"`
	AppLogo     string `json:"app_logo" gorm:"column:app_logo;size:255"`
	AppVersion  string `json:"app_version" gorm:"column:app_version;size:20"`
	VersionNo   string `json:"version_no" gorm:"column:version_no;size:20"`
	VersionCode string `json:"version_code" gorm:"column:version_code;size:20"`
	VersionName string `json:"version_name" gorm:"column:version_name;size:50"`
	IsActive    bool   `json:"is_active" gorm:"column:is_active;default:true"`

	Description string `json:"description" gorm:"column:description;size:255"`

	// Relationship
	Role Role `gorm:"foreignKey:RoleID;references:ID"`
}

func (AppConfig) TableName() string {
	return config.GetConfig().Database.TbWebAppConfig
}
