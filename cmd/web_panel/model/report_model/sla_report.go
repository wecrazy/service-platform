package reportmodel

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type ODOOMSSLAReport struct {
	ID uint `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	gorm.Model

	TicketNumber              string     `gorm:"column:ticket_number" json:"ticket_number"`
	TicketCreatedAt           *time.Time `gorm:"column:ticket_created_at" json:"ticket_created_at"`
	Stage                     string     `gorm:"column:stage" json:"stage"`
	Technician                string     `gorm:"column:technician" json:"technician"`
	Company                   string     `gorm:"column:company" json:"company"`
	TaskType                  string     `gorm:"column:task_type" json:"task_type"`
	ReceivedDatetimeSPK       *time.Time `gorm:"column:received_datetime_spk" json:"received_datetime_spk"`
	SLADeadline               *time.Time `gorm:"column:sla_deadline" json:"sla_deadline"`
	SLAStatus                 string     `gorm:"column:sla_status" json:"sla_status"`
	SLAExpired                string     `gorm:"column:sla_expired" json:"sla_expired"`
	CompleteDatetimeWO        *time.Time `gorm:"column:complete_datetime_wo" json:"complete_datetime_wo"`
	TechnicianGroup           string     `gorm:"column:technician_group" json:"technician_group"`
	MID                       string     `gorm:"column:mid" json:"mid"`
	TID                       string     `gorm:"column:tid" json:"tid"`
	TaskCount                 int        `gorm:"column:task_count" json:"task_count"`
	Merchant                  string     `gorm:"column:merchant" json:"merchant"`
	MerchantPIC               string     `gorm:"column:merchant_pic" json:"merchant_pic"`
	MerchantPhone             string     `gorm:"column:merchant_phone" json:"merchant_phone"`
	MerchantAddress           string     `gorm:"column:merchant_address" json:"merchant_address"`
	MerchantLongitude         *float64   `gorm:"type:decimal(10,6);column:merchant_longitude" json:"merchant_longitude"`
	MerchantLatitude          *float64   `gorm:"type:decimal(10,6);column:merchant_latitude" json:"merchant_latitude"`
	LinkWO                    string     `gorm:"column:link_wo" json:"link_wo"`
	WORemark                  string     `gorm:"column:wo_remark" json:"wo_remark"`
	WOFirst                   string     `gorm:"column:wo_first" json:"wo_first"`
	WOLast                    string     `gorm:"column:wo_last" json:"wo_last"`
	StatusEDC                 string     `gorm:"column:status_edc" json:"status_edc"`
	KondisiMerchant           string     `gorm:"column:kondisi_merchant" json:"kondisi_merchant"`
	EDCType                   string     `gorm:"column:edc_type" json:"edc_type"`
	EDCSerial                 string     `gorm:"column:edc_serial" json:"edc_serial"`
	Source                    string     `gorm:"column:source" json:"source"`
	ReasonCode                string     `gorm:"column:reason_code" json:"reason_code"`
	FirstTaskCompleteDatetime *time.Time `gorm:"column:first_task_complete_datetime" json:"first_task_complete_datetime"`
	FirstTaskReason           string     `gorm:"column:first_task_reason" json:"first_task_reason"`
	FirstTaskReasonCode       string     `gorm:"column:first_task_reason_code" json:"first_task_reason_code"`
	FirstTaskMessage          string     `gorm:"column:first_task_message" json:"first_task_message"`
	Description               string     `gorm:"column:description" json:"description"`
	TicketType                string     `gorm:"column:ticket_type" json:"ticket_type"`
}

func (ODOOMSSLAReport) TableName() string {
	return config.GetConfig().Database.TbReportSLA
}
