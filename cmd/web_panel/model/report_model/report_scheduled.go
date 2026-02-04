package reportmodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type ReportScheduled struct {
	ID string `gorm:"column:id;primaryKey;type:varchar(255)" json:"id"`
	gorm.Model

	FilePath            string     `gorm:"column:file_path;type:text;not null" json:"file_path"`
	ScheduledAt         *time.Time `gorm:"column:scheduled_at;type:datetime" json:"scheduled_at"`
	ScheduledBy         string     `gorm:"column:scheduled_by;type:varchar(255)" json:"scheduled_by"`
	AlreadySentViaEmail bool       `gorm:"column:already_sent_via_email;type:boolean;not null;default:false" json:"already_sent_via_email"`
}

func (ReportScheduled) TableName() string {
	return config.WebPanel.Get().Database.TbReportScheduled
}
