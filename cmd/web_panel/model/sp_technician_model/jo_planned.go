package sptechnicianmodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type JOPlannedForTechnicianODOOMS struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Technician               string         `gorm:"column:technician;type:varchar(255)" json:"technician"`
	Name                     string         `gorm:"column:name;type:varchar(255)" json:"name"`
	SPL                      string         `gorm:"column:spl;type:varchar(255)" json:"spl"`
	SAC                      string         `gorm:"column:sac;type:varchar(255)" json:"sac"`
	EmailTechnician          string         `gorm:"column:email_technician;type:text" json:"email_technician"`
	NoHPTechnician           string         `gorm:"column:no_hp_technician;type:text" json:"no_hp_technician"`
	WONumber                 datatypes.JSON `gorm:"column:wo_number;type:json" json:"wo_number"`
	TicketSubject            datatypes.JSON `gorm:"column:ticket_subject;type:json" json:"ticket_subject"`
	WONumberVisited          datatypes.JSON `gorm:"column:wo_number_visited;type:json" json:"wo_number_visited"`
	TicketSubjectVisited     datatypes.JSON `gorm:"column:ticket_subject_visited;type:json" json:"ticket_subject_visited"`
	TechnicianLastLogin      *time.Time     `gorm:"column:technician_last_login" json:"technician_last_login"`
	TechnicianLastDownloadJO *time.Time     `gorm:"column:technician_last_download_jo" json:"technician_last_download_jo"`
	TechnicianFirstUpload    *time.Time     `gorm:"column:technician_first_upload" json:"technician_first_upload"`
	TechnicianLastVisit      *time.Time     `gorm:"column:technician_last_visit" json:"technician_last_visit"`
}

func (JOPlannedForTechnicianODOOMS) TableName() string {
	return config.WebPanel.Get().SPTechnician.TBJoPlannedOdooMS
}

type JOPlannedForTechnicianODOOATM struct {
	JOPlannedForTechnicianODOOMS `gorm:"embedded"`
}

func (JOPlannedForTechnicianODOOATM) TableName() string {
	return config.WebPanel.Get().SPTechnician.TBJoPlannedOdooATM
}
