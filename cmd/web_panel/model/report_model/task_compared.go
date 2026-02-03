package reportmodel

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type TaskComparedData struct {
	gorm.Model
	ID uint `gorm:"primarykey"`

	Merchant            string     `gorm:"type:text;column:merchant" json:"merchant"`
	PicMerchant         string     `gorm:"type:text;column:pic_merchant" json:"pic_merchant"`
	PicPhone            string     `gorm:"type:text;column:pic_phone" json:"pic_phone"`
	MerchantAddress     string     `gorm:"type:text;column:merchant_address" json:"merchant_address"`
	Description         string     `gorm:"type:text;column:description" json:"description"`
	TaskType            string     `gorm:"type:text;column:task_type" json:"task_type"`
	WorksheetTemplate   string     `gorm:"type:text;column:worksheet_template" json:"worksheet_template"`
	TicketType2         string     `gorm:"type:text;column:ticket_type2" json:"ticket_type2"`
	Company             string     `gorm:"type:text;column:company" json:"company"`
	Stage               string     `gorm:"type:text;column:stage" json:"stage"`
	TicketSubject       string     `gorm:"type:text;column:ticket_subject" json:"ticket_subject"`
	MID                 string     `gorm:"type:text;column:mid" json:"mid"`
	TID                 string     `gorm:"type:text;column:tid" json:"tid"`
	Source              string     `gorm:"type:text;column:source" json:"source"`
	MessageCallCenter   string     `gorm:"type:text;column:message_call_center" json:"message_call_center"`
	WONumber            string     `gorm:"type:text;column:wo_number" json:"wo_number"`
	StatusMerchant      string     `gorm:"type:text;column:status_merchant" json:"status_merchant"`
	SNEDC               string     `gorm:"type:text;column:sn_edc" json:"sn_edc"`
	EDCType             string     `gorm:"type:text;column:edc_type" json:"edc_type"`
	WORemark            string     `gorm:"type:text;column:wo_remark" json:"wo_remark"`
	Longitude           string     `gorm:"type:text;column:longitude" json:"longitude"`
	Latitude            string     `gorm:"type:text;column:latitude" json:"latitude"`
	Technician          string     `gorm:"type:text;column:technician" json:"technician"`
	ReasonCode          string     `gorm:"type:text;column:reason_code" json:"reason_code"`
	LastUpdateBy        string     `gorm:"type:text;column:last_update_by" json:"last_update_by"`
	SLADeadline         *time.Time `gorm:"column:sla_deadline" json:"sla_deadline"`
	CreateDate          *time.Time `gorm:"column:create_date" json:"create_date"`
	ReceivedDatetimeSpk *time.Time `gorm:"column:received_datetime_spk" json:"received_datetime_spk"`
	PlanDate            *time.Time `gorm:"column:planned_date_begin" json:"planned_date_begin"`
	TimesheetLastStop   *time.Time `gorm:"column:timesheet_timer_last_stop" json:"timesheet_timer_last_stop"`
	DateLastStageUpdate *time.Time `gorm:"column:date_last_stage_update" json:"date_last_stage_update"`
}

func (TaskComparedData) TableName() string {
	return config.GetConfig().Database.TbReportCompared
}
