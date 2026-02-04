package reportmodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type MonitoringTicketODOOMS struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Technician              string     `gorm:"type:text;column:technician" json:"technician"`
	TicketNumber            string     `gorm:"type:text;column:ticket_number" json:"ticket_number"`
	Stage                   string     `gorm:"type:text;column:stage" json:"stage"`
	Company                 string     `gorm:"type:text;column:company" json:"company"`
	TaskType                string     `gorm:"type:text;column:task_type" json:"task_type"`
	ReceivedSPKAt           *time.Time `gorm:"column:received_spk_at" json:"received_spk_at"`
	SLADeadline             *time.Time `gorm:"column:sla_deadline" json:"sla_deadline"`
	CompleteWO              *time.Time `gorm:"column:complete_wo" json:"complete_wo"`
	SLAStatus               string     `gorm:"type:text;column:sla_status" json:"sla_status"`
	SLAExpired              string     `gorm:"type:text;column:sla_expired" json:"sla_expired"`
	MID                     string     `gorm:"type:text;column:mid" json:"mid"`
	TID                     string     `gorm:"type:text;column:tid" json:"tid"`
	Merchant                string     `gorm:"type:text;column:merchant" json:"merchant"`
	MerchantPIC             string     `gorm:"type:text;column:merchant_pic" json:"merchant_pic"`
	MerchantPhone           string     `gorm:"type:text;column:merchant_phone" json:"merchant_phone"`
	MerchantAddress         string     `gorm:"type:text;column:merchant_address" json:"merchant_address"`
	MerchantLatitude        *float64   `gorm:"column:merchant_latitude" json:"merchant_latitude"`
	MerchantLongitude       *float64   `gorm:"column:merchant_longitude" json:"merchant_longitude"`
	TaskCount               int        `gorm:"column:task_count" json:"task_count"`
	WORemark                string     `gorm:"type:text;column:wo_remark" json:"wo_remark"`
	ReasonCode              string     `gorm:"type:text;column:reason_code" json:"reason_code"`
	FirstJOCompleteDatetime *time.Time `gorm:"column:first_jo_complete_datetime" json:"first_jo_complete_datetime"`
	FirstJOReason           string     `gorm:"type:text;column:first_jo_reason" json:"first_jo_reason"`
	FirstJOMessage          string     `gorm:"type:text;column:first_jo_message" json:"first_jo_message"`
	FirstJOReasonCode       string     `gorm:"type:text;column:first_jo_reason_code" json:"first_jo_reason_code"`
	LinkWO                  string     `gorm:"type:text;column:link_wo" json:"link_wo"`
	WOFirst                 string     `gorm:"type:text;column:wo_first" json:"wo_first"`
	WOLast                  string     `gorm:"type:text;column:wo_last" json:"wo_last"`
	StatusEDC               string     `gorm:"type:text;column:status_edc" json:"status_edc"`
	KondisiMerchant         string     `gorm:"type:text;column:kondisi_merchant" json:"kondisi_merchant"`
	EDCType                 string     `gorm:"type:text;column:edc_type" json:"edc_type"`
	EDCSerial               string     `gorm:"type:text;column:edc_serial" json:"edc_serial"`
	Source                  string     `gorm:"type:text;column:source" json:"source"`
	Description             string     `gorm:"type:text;column:description" json:"description"`

	IsPaid bool `gorm:"column:is_paid" json:"is_paid"`
}

func (MonitoringTicketODOOMS) TableName() string {
	return config.WebPanel.Get().Database.TbReportMonitoringTicket
}
