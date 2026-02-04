package reportmodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type MTIReportVTR struct {
	ID uint `gorm:"primaryKey" json:"id"`
	gorm.Model

	SysIDTrCaseID     string     `gorm:"type:text;column:sys_id_tr_case_id" json:"sys_id_tr_case_id"`
	AcceptDate        *time.Time `gorm:"type:datetime;column:accept_date" json:"accept_date"`
	AcceptTime        *time.Time `gorm:"type:datetime;column:accept_time" json:"accept_time"`
	Active            string     `gorm:"type:text;column:active" json:"active"`
	ActualDate        *time.Time `gorm:"type:datetime;column:actual_date" json:"actual_date"`
	Address           string     `gorm:"type:text;column:address" json:"address"`
	Ado               string     `gorm:"type:text;column:ado" json:"ado"`
	Aging             string     `gorm:"type:text;column:aging" json:"aging"`
	Alert             string     `gorm:"type:text;column:alert" json:"alert"`
	Aom               string     `gorm:"type:text;column:aom" json:"aom"`
	NameRUser         string     `gorm:"type:text;column:name_r_user" json:"name_r_user"`
	NameRGroup        string     `gorm:"type:text;column:name_r_group" json:"name_r_group"`
	NameTrCategory    string     `gorm:"type:text;column:name_tr_category" json:"name_tr_category"`
	CaseID            string     `gorm:"type:text;column:case_id" json:"case_id"`
	Cid               string     `gorm:"type:text;column:cid" json:"cid"`
	CaseIDSla         string     `gorm:"type:text;column:case_id_sla" json:"case_id_sla"`
	CaseType          string     `gorm:"type:text;column:case_type" json:"case_type"`
	Channel           string     `gorm:"type:text;column:channel" json:"channel"`
	NameRCity         string     `gorm:"type:text;column:name_r_city" json:"name_r_city"`
	Comments          string     `gorm:"type:text;column:comments" json:"comments"`
	Created           string     `gorm:"type:text;column:created" json:"created"`
	DataChngMst       *time.Time `gorm:"type:datetime;column:data_chng_mst" json:"data_chng_mst"`
	DateChngDttm      *time.Time `gorm:"type:datetime;column:date_chng_dttm" json:"date_chng_dttm"`
	EmailVendor       string     `gorm:"type:text;column:email_vendor" json:"email_vendor"`
	FlActive          string     `gorm:"type:text;column:fl_active" json:"fl_active"`
	Identifier        string     `gorm:"type:text;column:identifier" json:"identifier"`
	NameRType         string     `gorm:"type:text;column:name_r_type" json:"name_r_type"`
	MemberBank        string     `gorm:"type:text;column:member_bank" json:"member_bank"`
	MerchantName      string     `gorm:"type:text;column:merchant_name" json:"merchant_name"`
	MerchantType      string     `gorm:"type:text;column:merchant_type" json:"merchant_type"`
	Mid               string     `gorm:"type:text;column:mid" json:"mid"`
	MidAstrapay       string     `gorm:"type:text;column:mid_astrapay" json:"mid_astrapay"`
	MidBni            string     `gorm:"type:text;column:mid_bni" json:"mid_bni"`
	MidBri            string     `gorm:"type:text;column:mid_bri" json:"mid_bri"`
	MidBtn            string     `gorm:"type:text;column:mid_btn" json:"mid_btn"`
	NameRRegion       string     `gorm:"type:text;column:name_r_region" json:"name_r_region"`
	Regular           string     `gorm:"type:text;column:regular" json:"regular"`
	RegularThermal    string     `gorm:"type:text;column:regular_thermal" json:"regular_thermal"`
	Segment           string     `gorm:"type:text;column:segment" json:"segment"`
	SerialNumber      string     `gorm:"type:text;column:serial_number" json:"serial_number"`
	NameRState        string     `gorm:"type:text;column:name_r_state" json:"name_r_state"`
	StatusReplace     string     `gorm:"type:text;column:status_replace" json:"status_replace"`
	NameTrSubCategory string     `gorm:"type:text;column:name_tr_sub_category" json:"name_tr_sub_category"`
	TabelName         string     `gorm:"type:text;column:tabel_name" json:"tabel_name"`
	TanggalVisit      *time.Time `gorm:"type:datetime;column:tanggal_visit" json:"tanggal_visit"`
	Tid               string     `gorm:"type:text;column:tid" json:"tid"`
	TidAstrapay       string     `gorm:"type:text;column:tid_astrapay" json:"tid_astrapay"`
	TidBtn            string     `gorm:"type:text;column:tid_btn" json:"tid_btn"`
	TidBri            string     `gorm:"type:text;column:tid_bri" json:"tid_bri"`
	TidBtn1           string     `gorm:"type:text;column:tid_btn_1" json:"tid_btn_1"`
	Updated           *time.Time `gorm:"type:datetime;column:updated" json:"updated"`
	NameRCompany      string     `gorm:"type:text;column:name_r_company" json:"name_r_company"`
	Voc               string     `gorm:"type:text;column:voc" json:"voc"`
	// Red Mark in Excel => maybe soon compared with ODOO data
	Stage         string     `gorm:"type:text;column:stage" json:"stage"`
	Remark        string     `gorm:"type:text;column:remark" json:"remark"`
	TanggalVisit2 *time.Time `gorm:"type:datetime;column:tanggal_visit2" json:"tanggal_visit2"`
	LinkWod       string     `gorm:"type:text;column:link_wod" json:"link_wod"`
	RootCause     string     `gorm:"type:text;column:rootcause" json:"rootcause"`
}

func (MTIReportVTR) TableName() string {
	return config.WebPanel.Get().ReportMTI.DBTableVTR
}
