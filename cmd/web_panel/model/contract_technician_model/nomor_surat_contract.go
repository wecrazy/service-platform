package contracttechnicianmodel

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type NomorSuratContract struct {
	ID string `json:"id" gorm:"column:id;primaryKey;autoIncrement:false"`
	gorm.Model
	LastNomorSurat int `json:"last_nomor_surat" gorm:"column:last_nomor_surat"`
}

func (NomorSuratContract) TableName() string {
	return config.WebPanel.Get().ContractTechnicianODOO.TBNomorSuratContract
}
