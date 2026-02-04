package reportmodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type MTIReportPenarikan struct {
	ID uint `gorm:"primaryKey" json:"id"`
	gorm.Model

	WorkOrderNumber      string     `gorm:"column:work_order_number" json:"work_order_number"`
	WorkType             string     `gorm:"column:work_type" json:"work_type"`
	MID                  string     `gorm:"column:mid" json:"mid"`
	TID                  string     `gorm:"column:tid" json:"tid"`
	TIDPrevious          string     `gorm:"column:tid_previous" json:"tid_previous"`
	MerchantOfficialName string     `gorm:"column:merchant_official_name" json:"merchant_official_name"`
	MerchantName         string     `gorm:"column:merchant_name" json:"merchant_name"`
	Address123           string     `gorm:"column:address_1_3" json:"address_1_3"`
	ContactPerson        string     `gorm:"column:contact_person" json:"contact_person"`
	PhoneNumber          string     `gorm:"column:phone_number" json:"phone_number"`
	Region               string     `gorm:"column:region" json:"region"`
	City                 string     `gorm:"column:city" json:"city"`
	ZipPostalCode        string     `gorm:"column:zip_postal_code" json:"zip_postal_code"`
	MobileNumber         string     `gorm:"column:mobile_number" json:"mobile_number"`
	MerchantSegment      string     `gorm:"column:merchant_segment" json:"merchant_segment"`
	EDCType              string     `gorm:"column:edc_type" json:"edc_type"`
	SNEDC                string     `gorm:"column:sn_edc" json:"sn_edc"`
	SNSimcard            string     `gorm:"column:sn_simcard" json:"sn_simcard"`
	SNSamcard            string     `gorm:"column:sn_samcard" json:"sn_samcard"`
	SimcardProvider      string     `gorm:"column:simcard_provider" json:"simcard_provider"`
	Vendor               string     `gorm:"column:vendor" json:"vendor"`
	EDCFeature           string     `gorm:"column:edc_feature" json:"edc_feature"`
	EDCConnType          string     `gorm:"column:edc_conn_type" json:"edc_conn_type"`
	WorkOrderStartDate   *time.Time `gorm:"column:work_order_start_date" json:"work_order_start_date"`
	SLATargetDate        *time.Time `gorm:"column:sla_target_date" json:"sla_target_date"`
	WorkOrderEndDate     *time.Time `gorm:"column:work_order_end_date" json:"work_order_end_date"`
	EDCStatus            string     `gorm:"column:edc_status" json:"edc_status"`
	Version              string     `gorm:"column:version" json:"version"`
	WorkOrderStatus      string     `gorm:"column:work_order_status" json:"work_order_status"`
	Remarks              string     `gorm:"column:remarks" json:"remarks"`
	PendingOwner         string     `gorm:"column:pending_owner" json:"pending_owner"`
	PendingReason        string     `gorm:"column:pending_reason" json:"pending_reason"`
	Engineer             string     `gorm:"column:engineer" json:"engineer"`
	ServicePoint         string     `gorm:"column:service_point" json:"service_point"`
	WorkRequestNumber    string     `gorm:"column:work_request_number" json:"work_request_number"`
	Warehouse            string     `gorm:"column:warehouse" json:"warehouse"`
	// Red Mark in Excel => maybe soon compared with ODOO data
	Stage      string     `gorm:"column:stage" json:"stage"`
	MemberBank string     `gorm:"column:member_bank" json:"member_bank"`
	Remark     string     `gorm:"column:remark" json:"remark"`
	RootCause  string     `gorm:"column:root_cause" json:"root_cause"`
	TglTarik   *time.Time `gorm:"column:tgl_tarik" json:"tgl_tarik"`
	LinkWod    string     `gorm:"column:link_wod" json:"link_wod"`
}

func (MTIReportPenarikan) TableName() string {
	return config.WebPanel.Get().ReportMTI.DBTablePenarikan
}
