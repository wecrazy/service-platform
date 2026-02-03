package odooms

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type UploadedExcelToODOOMS struct {
	gorm.Model
	Email            string     `gorm:"column:email;type:varchar(300);not null" json:"email"`
	Password         string     `gorm:"column:pwd;type:varchar(300);not null" json:"pwd"`
	Filename         string     `gorm:"column:filename;type:text;not null" json:"filename"`
	OriginalFilename string     `gorm:"column:ori_filename;type:text;not null" json:"ori_filename"`
	Status           string     `gorm:"column:status;type:varchar(50);not null" json:"status"`
	Template         int        `gorm:"column:template;type:int;not null" json:"template"`
	TotalRow         int        `gorm:"column:total_row;type:int;not null" json:"total_row"`
	TotalSuccess     int        `gorm:"column:total_success;type:int;not null" json:"total_success"`
	TotalFail        int        `gorm:"column:total_fail;type:int;not null" json:"total_fail"`
	CompleteTime     *time.Time `gorm:"column:complete_time;type:timestamp" json:"complete_time"`
	Logs             string     `gorm:"column:log;type:text" json:"log"` // Changed from *string to string for better handling
}

func (UploadedExcelToODOOMS) TableName() string {
	return config.GetConfig().Database.TbODOOMSUploadedExcel
}
