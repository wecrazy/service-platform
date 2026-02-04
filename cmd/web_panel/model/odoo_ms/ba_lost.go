package odooms

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type CSNABALost struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	No                   int        `gorm:"column:no" json:"no"`
	Count                int        `gorm:"column:count" json:"count"`
	SerialNumber         string     `gorm:"column:serialnumber" json:"serialnumber"`
	CekSN                string     `gorm:"column:cek_sn" json:"cek_sn"`
	LastUpdate           *time.Time `gorm:"column:last_update" json:"last_update"`
	StatusUpdate         string     `gorm:"column:status_update" json:"status_update"`
	Vendor               string     `gorm:"column:vendor" json:"vendor"`
	Location             string     `gorm:"column:location" json:"location"`
	Aging                *time.Time `gorm:"column:aging" json:"aging"`
	NoteDailyStock       string     `gorm:"column:note_daily_stock" json:"note_daily_stock"`
	StatusEDC            string     `gorm:"column:status_edc" json:"status_edc"`
	Peripheral           string     `gorm:"column:peripheral" json:"peripheral"`
	MID                  string     `gorm:"column:mid" json:"mid"`
	TID                  string     `gorm:"column:tid" json:"tid"`
	RBM                  string     `gorm:"column:rbm" json:"rbm"`
	RBM1                 string     `gorm:"column:rbm1" json:"rbm1"`
	BASTOut              string     `gorm:"column:bast_out" json:"bast_out"`
	NoteLocation         string     `gorm:"column:note_location" json:"note_location"`
	BASTDate             *time.Time `gorm:"column:bast_date" json:"bast_date"`
	Alokasi              string     `gorm:"column:alokasi" json:"alokasi"`
	Device               string     `gorm:"column:device" json:"device"`
	PROCCategory         string     `gorm:"column:proc_category" json:"proc_category"`
	Merk                 string     `gorm:"column:merk" json:"merk"`
	EDCType              string     `gorm:"column:edc_type" json:"edc_type"`
	COMM                 string     `gorm:"column:comm" json:"comm"`
	KontrakMB            string     `gorm:"column:kontrak_mb" json:"kontrak_mb"`
	CAT                  string     `gorm:"column:cat" json:"cat"`
	CAT1                 string     `gorm:"column:cat1" json:"cat1"`
	Maas                 string     `gorm:"column:maas" json:"maas"`
	VendorSO             string     `gorm:"column:vendor_so" json:"vendor_so"`
	ServicePoint         string     `gorm:"column:service_point" json:"service_point"`
	TglSO                *time.Time `gorm:"column:tgl_so" json:"tgl_so"`
	Region2              string     `gorm:"column:region2" json:"region2"`
	KondisiEDC           string     `gorm:"column:kondisi_edc" json:"kondisi_edc"`
	SanggahanVendor      string     `gorm:"column:sanggahan_vendor" json:"sanggahan_vendor"`
	Feedback             string     `gorm:"column:feedback" json:"feedback"`
	SesuaiVendor         string     `gorm:"column:sesuai_vendor" json:"sesuai_vendor"`
	Note                 string     `gorm:"column:note" json:"note"`
	Location2            string     `gorm:"column:location2" json:"location2"`
	Region               string     `gorm:"column:region" json:"region"`
	LocationODOO         string     `gorm:"column:location_odoo" json:"location_odoo"`
	Head                 string     `gorm:"column:head" json:"head"`
	SP                   string     `gorm:"column:sp" json:"sp"`
	AdaDiBALostPrevMonth string     `gorm:"column:ada_di_ba_lost_prev" json:"ada_di_ba_lost_prev"`
	NoteMonitoring       string     `gorm:"column:note_monitoring" json:"note_monitoring"`
	LinkWOD              string     `gorm:"column:link_wod" json:"link_wod"`
	NoteAll              string     `gorm:"column:note_all" json:"note_all"`
	LinkFoto             string     `gorm:"column:link_foto" json:"link_foto"`
	Summary              string     `gorm:"column:summary" json:"summary"`
	ApprovedStatus       string     `gorm:"column:approved_status" json:"approved_status"`
}

func (CSNABALost) TableName() string {
	return config.WebPanel.Get().Database.TbBALost
}
