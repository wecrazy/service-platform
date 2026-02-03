package dkimodel

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type TicketDKI struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	gorm.Model

	Priority                  string     `gorm:"type:text;column:priority;default:NULL" json:"priority"`
	Subject                   string     `gorm:"type:text;column:subject;default:NULL" json:"subject"`
	Stage                     string     `gorm:"type:text;column:stage;default:NULL" json:"stage"`
	Mid                       string     `gorm:"type:text;column:mid;default:NULL" json:"mid"`
	Tid                       string     `gorm:"type:text;column:tid;default:NULL" json:"tid"`
	WoNumberFirst             string     `gorm:"type:text;column:wo_number_first;default:NULL" json:"-"`
	WoNumberLast              string     `gorm:"type:text;column:wo_number_last;default:NULL" json:"-"`
	ReasonCode                string     `gorm:"type:text;column:reason_code;default:NULL" json:"reason_code"`
	Reason                    string     `gorm:"type:text;column:reason;default:NULL" json:"reason"`
	LinkWod                   string     `gorm:"type:text;column:link_wod;default:NULL" json:"link_wod"`
	MerchantName              string     `gorm:"type:text;column:merchant_name;default:NULL" json:"merchant_name"`
	MerchantPic               string     `gorm:"type:text;column:merchant_pic;default:NULL" json:"merchant_pic"`
	PicPhoneNumber            string     `gorm:"type:text;column:pic_phone_number;default:NULL" json:"pic_phone_number"`
	MerchantCity              string     `gorm:"type:text;column:merchant_city;default:NULL" json:"merchant_city"`
	MerchantZipCode           string     `gorm:"type:text;column:merchant_zip_code;default:NULL" json:"merchant_zip_code"`
	MerchantAddress           string     `gorm:"type:text;column:merchant_address;default:NULL" json:"merchant_address"`
	MerchantCondition         string     `gorm:"type:text;column:merchant_condition;default:NULL" json:"merchant_condition"`
	EdcStatus                 string     `gorm:"type:text;column:edc_status;default:NULL" json:"edc_status"`
	EdcCondition              string     `gorm:"type:text;column:edc_condition;default:NULL" json:"edc_condition"`
	WoRemark                  string     `gorm:"type:text;column:wo_remark;default:NULL" json:"wo_remark"`
	Latitude                  *float64   `gorm:"type:decimal(10,2);column:latitude;default:0" json:"latitude"`
	Longitude                 *float64   `gorm:"type:decimal(10,2);column:longitude;default:0" json:"longitude"`
	Location                  string     `gorm:"-" json:"location"`
	TicketType                string     `gorm:"type:text;column:ticket_type;default:NULL" json:"ticket_type"`
	Technician                string     `gorm:"type:text;column:technician;default:NULL" json:"technician"`
	SnEdc                     string     `gorm:"type:text;column:sn_edc;default:NULL" json:"sn_edc"`
	EdcType                   string     `gorm:"type:text;column:edc_type;default:NULL" json:"edc_type"`
	SLAStatus                 string     `gorm:"type:text;column:sla_status;default:NULL" json:"sla_status"`
	ReceivedDatetimeSpk       *time.Time `gorm:"type:datetime;column:received_datetime_spk;default:NULL" json:"received_datetime_spk"`
	SlaDeadline               *time.Time `gorm:"type:datetime;column:sla_deadline;default:NULL" json:"sla_deadline"`
	CompleteDatetimeWO        *time.Time `gorm:"type:datetime;column:complete_datetime_wo;default:NULL" json:"complete_datetime_wo"`
	FirstTaskCompleteDatetime *time.Time `gorm:"type:datetime;column:first_task_complete_datetime;default:NULL" json:"first_task_complete_datetime"`
	FirstTaskReason           string     `gorm:"type:text;column:first_task_reason;default:NULL" json:"-"`
	FirstTaskReasonCode       string     `gorm:"type:text;column:first_task_reason_code;default:NULL" json:"-"`
	FirstTaskMessage          string     `gorm:"type:text;column:first_task_message;default:NULL" json:"-"`
}

func (TicketDKI) TableName() string {
	return config.GetConfig().DKI.TBDataTicketODOOMS
}
