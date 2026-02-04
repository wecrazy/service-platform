package reportmodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type EngineersProductivityData struct {
	gorm.Model
	ID                        uint       `gorm:"primarykey"`
	TicketNumber              string     `gorm:"type:text;column:ticket_number" json:"ticket_number"`
	Company                   string     `gorm:"type:text;column:company" json:"company"`
	Technician                string     `gorm:"type:text;column:technician" json:"technician"`
	TechnicianGroup           string     `gorm:"type:text;column:technician_group" json:"technician_group"`
	TicketType                string     `gorm:"type:text;column:ticket_type" json:"ticket_type"`
	WorksheetTemplate         string     `gorm:"type:text;column:worksheet_template" json:"worksheet_template"`
	TaskType                  string     `gorm:"type:text;column:task_type" json:"task_type"`
	Stage                     string     `gorm:"type:text;column:stage" json:"stage"`
	ReasonCode                string     `gorm:"type:text;column:reason_code" json:"reason_code"`
	Reason                    string     `gorm:"type:text;column:reason" json:"reason"`
	Project                   string     `gorm:"type:text;column:project" json:"project"`
	Source                    string     `gorm:"type:text;column:source" json:"source"`
	MerchantName              string     `gorm:"type:text;column:merchant_name" json:"merchant_name"`
	MerchantPic               string     `gorm:"type:text;column:merchant_pic" json:"merchant_pic"`
	MerchantAddress           string     `gorm:"type:text;column:merchant_address" json:"merchant_address"`
	MerchantCity              string     `gorm:"type:text;column:merchant_city" json:"merchant_city"`
	Mid                       string     `gorm:"type:text;column:mid" json:"mid"`
	Tid                       string     `gorm:"type:text;column:tid" json:"tid"`
	SnEdc                     string     `gorm:"type:text;column:sn_edc" json:"sn_edc"`
	EdcType                   string     `gorm:"type:text;column:edc_type" json:"edc_type"`
	TaskCount                 int        `gorm:"column:task_count" json:"task_count"`
	LinkWod                   string     `gorm:"type:text;column:link_wod" json:"link_wod"`
	WoNumberFirst             string     `gorm:"type:text;column:wo_number_first" json:"wo_number_first"`
	WoNumberLast              string     `gorm:"type:text;column:wo_number_last" json:"wo_number_last"`
	SlaStatus                 string     `gorm:"type:text;column:sla_status" json:"sla_status"`
	SlaExpired                string     `gorm:"type:text;column:sla_expired" json:"sla_expired"`
	Description               string     `gorm:"type:text;column:description" json:"description"`
	WoRemark                  string     `gorm:"type:text;column:wo_remark" json:"wo_remark"`
	FirstTaskReasonCode       string     `gorm:"type:text;column:first_task_reason_code" json:"first_task_reason_code"`
	FirstTaskReason           string     `gorm:"type:text;column:first_task_reason" json:"first_task_reason"`
	FirstTaskMessage          string     `gorm:"type:text;column:first_task_message" json:"first_task_message"`
	FirstTaskCompleteDatetime *time.Time `gorm:"column:first_task_complete_datetime" json:"first_task_complete_datetime"`
	ReceivedDatetimeSpk       *time.Time `gorm:"column:received_datetime_spk" json:"received_datetime_spk"`
	SlaDeadline               *time.Time `gorm:"column:sla_deadline" json:"sla_deadline"`
	CompleteDateWo            *time.Time `gorm:"column:complete_date_wo" json:"complete_date_wo"`
	TicketCreatedAt           *time.Time `gorm:"column:create_date" json:"create_date"`
}

func (EngineersProductivityData) TableName() string {
	return config.WebPanel.Get().Database.TbReportEngineersProd
}
