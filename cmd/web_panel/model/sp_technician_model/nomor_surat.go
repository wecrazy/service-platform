package sptechnicianmodel

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type NomorSuratSP struct {
	ID string `json:"id" gorm:"column:id;primaryKey;autoIncrement:false"`
	gorm.Model
	LastNomorSuratSP int `json:"last_nomor_surat_sp" gorm:"column:last_nomor_surat_sp"`
}

func (NomorSuratSP) TableName() string {
	return config.WebPanel.Get().SPTechnician.TBNomorSuratSP
}
