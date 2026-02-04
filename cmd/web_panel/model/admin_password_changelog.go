package model

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type AdminPasswordChangeLog struct {
	gorm.Model
	Email    string `json:"email" gorm:"column:email"`
	Password string `json:"password" gorm:"column:password"`
}

func (AdminPasswordChangeLog) TableName() string {
	return config.WebPanel.Get().Database.TbAdminPwdChangelog
}
