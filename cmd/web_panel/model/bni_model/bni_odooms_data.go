package bnimodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type BNIOdooMSData struct { // Field service / project.task data
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	TaskType            string     `gorm:"column:task_type;type:text" json:"task_type"`
	Stage               string     `gorm:"column:stage;type:text" json:"stage"`
	WONumber            string     `gorm:"column:wo_number;type:text" json:"wo_number"`
	Technician          string     `gorm:"column:technician;type:text" json:"technician"`
	Mid                 string     `gorm:"column:mid;type:text" json:"mid"`
	Tid                 string     `gorm:"column:tid;type:text" json:"tid"`
	MerchantName        string     `gorm:"column:merchant_name;type:text" json:"merchant_name"`
	MerchantCity        string     `gorm:"column:merchant_city;type:text" json:"merchant_city"`
	MerchantZip         string     `gorm:"column:merchant_zip;type:text" json:"merchant_zip"`
	PicMerchant         string     `gorm:"column:pic_merchant;type:text" json:"pic_merchant"`
	PicPhone            string     `gorm:"column:pic_phone;type:text" json:"pic_phone"`
	MerchantAddress     string     `gorm:"column:merchant_address;type:text" json:"merchant_address"`
	Description         string     `gorm:"column:description;type:text" json:"description"`
	Source              string     `gorm:"column:source;type:text" json:"source"`
	MessageCC           string     `gorm:"column:message_cc;type:text" json:"-"`
	StatusMerchant      string     `gorm:"column:status_merchant;type:text" json:"status_merchant"`
	WoRemarkTiket       string     `gorm:"column:wo_remark_tiket;type:text" json:"wo_remark_tiket"`
	Longitude           string     `gorm:"column:longitude;type:text" json:"longitude"`
	Latitude            string     `gorm:"column:latitude;type:text" json:"latitude"`
	Location            string     `gorm:"-" json:"location"`
	LinkPhoto           string     `gorm:"column:link_photo;type:text" json:"link_photo"`
	TicketType          string     `gorm:"column:ticket_type;type:text" json:"ticket_type"`
	WorksheetTemplate   string     `gorm:"column:worksheet_template;type:text" json:"worksheet_template"`
	TicketSubject       string     `gorm:"column:ticket_subject;type:text" json:"ticket_subject"`
	SNEDC               string     `gorm:"column:sn_edc;type:text" json:"sn_edc"`
	EDCType             string     `gorm:"column:edc_type;type:text" json:"edc_type"`
	ReasonCode          string     `gorm:"column:reason_code;type:text" json:"reason_code"`
	SlaDeadline         *time.Time `gorm:"column:sla_deadline" json:"sla_deadline"`
	CreateDate          *time.Time `gorm:"column:create_date" json:"-"`
	ReceivedDatetimeSpk *time.Time `gorm:"column:received_datetime_spk" json:"received_datetime_spk"`
	PlanDate            *time.Time `gorm:"column:plan_date" json:"plan_date"`
	TimesheetLastStop   *time.Time `gorm:"column:timesheet_last_stop" json:"timesheet_last_stop"`
	DateLastStageUpdate *time.Time `gorm:"column:date_last_stage_update" json:"-"`
}

func (BNIOdooMSData) TableName() string {
	return config.WebPanel.Get().BNI.TBDataODOOMS
}
