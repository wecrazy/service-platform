package model

import (
	"time"

	"gorm.io/gorm"
)

type TeknisiKunjungan struct {
	ID                uint           `gorm:"primaryKey" json:"id" form:"id"`
	CreatedAt         time.Time      `gorm:"column:created_at" json:"created_at" form:"created_at"`
	NamaAdmin         string         `form:"nama_admin" json:"nama_admin" gorm:"column:nama_admin"`
	NamaTeknisi       string         `gorm:"column:nama_teknisi" json:"nama_teknisi" form:"nama_teknisi"`
	IdEmployee        string         `form:"id_employee" json:"id_employee" gorm:"column:id_employee"`
	ServicePoin       string         `form:"service_poin" json:"service_poin" gorm:"column:service_poin"`
	SPKNumber         string         `gorm:"column:spk_number" json:"spk_number" form:"spk_number"`
	WONumber          string         `gorm:"column:wo_number" json:"wo_number" form:"wo_number"`
	SerialNumber      string         `gorm:"column:serial_number" json:"serial_number" form:"serial_number"`
	MerchantName      string         `gorm:"column:merchant_name" json:"merchant_name" form:"merchant_name"`
	NamaAplikasi      string         `gorm:"column:nama_aplikasi" json:"nama_aplikasi" form:"nama_aplikasi"`
	VersiAplikasi     string         `gorm:"column:versi_aplikasi" json:"versi_aplikasi" form:"versi_aplikasi"`
	TID               string         `gorm:"column:tid" json:"tid" form:"tid"`
	MID               string         `gorm:"column:mid" json:"mid" form:"mid"`
	Kunci             string         `gorm:"column:kunci" json:"kunci" form:"kunci"`
	ParamNamaMerchant string         `gorm:"column:param_nama_merchant" json:"param_nama_merchant" form:"param_nama_merchant"`
	ParamAddr1        string         `gorm:"column:param_addr1" json:"param_addr1" form:"param_addr1"`
	ParamAddr2        string         `gorm:"column:param_addr2" json:"param_addr2" form:"param_addr2"`
	ParamAddr3        string         `gorm:"column:param_addr3" json:"param_addr3" form:"param_addr3"`
	Remark            string         `gorm:"column:remark" json:"remark" form:"remark"`
	UpdatedAt         time.Time      `gorm:"column:updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" `
}

func (TeknisiKunjungan) TableName() string {
	return "teknisi_kunjungan"
}
