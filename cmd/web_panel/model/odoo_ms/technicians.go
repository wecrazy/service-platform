package odooms

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ODOOMSTechnicianData struct {
	ID        uint `gorm:"column:id;primaryKey;autoIncrement"`
	CreatedAt time.Time
	gorm.Model

	Technician      string     `gorm:"column:technician;type:varchar(255)" json:"technician"`
	Name            string     `gorm:"column:name;type:varchar(255)" json:"name"`
	TechnicianGroup string     `gorm:"column:technician_group;type:varchar(255)" json:"technician_group"`
	SPL             string     `gorm:"column:spl;type:varchar(255)" json:"spl"`
	SAC             string     `gorm:"column:sac;type:varchar(255)" json:"sac"`
	Email           string     `gorm:"column:email;type:varchar(255)" json:"email"`
	NoHP            string     `gorm:"column:no_hp;type:varchar(100)" json:"no_hp"`
	JobGroupID      int        `gorm:"column:job_group_id" json:"job_group_id"`
	NIK             string     `gorm:"column:nik;type:varchar(100)" json:"nik"`
	Address         string     `gorm:"column:address;type:text" json:"address"`
	Area            string     `gorm:"column:area;type:varchar(255)" json:"area"`
	BirthStatus     string     `gorm:"column:birth_status;type:varchar(255)" json:"birth_status"`
	UserCreatedOn   *time.Time `gorm:"column:user_created_on" json:"user_created_on"`
	EmployeeCode    string     `gorm:"column:employee_code;type:varchar(255)" json:"employee_code"`

	LastLogin      *time.Time `gorm:"column:last_login" json:"last_login"`
	LastDownloadJO *time.Time `gorm:"column:last_download_jo" json:"last_download_jo"`
	FirstUpload    *time.Time `gorm:"column:first_upload" json:"first_upload"`
	LastVisit      *time.Time `gorm:"column:last_visit" json:"last_visit"`

	// Using JSON datatype to store array of values -> JO Planned based on created_at
	WONumber      datatypes.JSON `gorm:"column:wo_number;type:json" json:"wo_number"`
	TicketSubject datatypes.JSON `gorm:"column:ticket_subject;type:json" json:"ticket_subject"`
	WOLinkPhotos  datatypes.JSON `gorm:"column:wo_link_photos;type:json" json:"wo_link_photos"`
	WOStages      datatypes.JSON `gorm:"column:wo_stages;type:json" json:"wo_stages"`

	WONumberVisited      datatypes.JSON `gorm:"column:wo_number_visited;type:json" json:"wo_number_visited"`
	TicketSubjectVisited datatypes.JSON `gorm:"column:ticket_subject_visited;type:json" json:"ticket_subject_visited"`
	WOVisitedLinkPhotos  datatypes.JSON `gorm:"column:wo_visited_link_photos;type:json" json:"wo_visited_link_photos"`
	WOVisitedStages      datatypes.JSON `gorm:"column:wo_visited_stages;type:json" json:"wo_visited_stages"`
}

func (ODOOMSTechnicianData) TableName() string {
	return config.GetConfig().Database.TbODOOMSDataTech
}
